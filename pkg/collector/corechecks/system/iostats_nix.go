// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2019 Datadog, Inc.

// +build !windows

package system

import (
	"bytes"
	"math"
	"regexp"
	"time"

	"github.com/DataDog/datadog-agent/pkg/aggregator"
	"github.com/DataDog/datadog-agent/pkg/autodiscovery/integration"
	core "github.com/DataDog/datadog-agent/pkg/collector/corechecks"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/shirou/gopsutil/disk"
)

// For testing purpose
var (
	ioCounters = disk.IOCounters

	// for test purpose
	nowNano = func() int64 { return time.Now().UnixNano() }
)

// IOCheck doesn't need additional fields
type IOCheck struct {
	core.CheckBase
	blacklist *regexp.Regexp
	ts        int64
	stats     map[string]disk.IOCountersStat
}

// Configure the IOstats check
func (c *IOCheck) Configure(data integration.Data, initConfig integration.Data) error {
	err := c.commonConfigure(data, initConfig)
	return err
}

// round a float64 with 2 decimal precision
func roundFloat(val float64) float64 {
	return math.Round(val*100) / 100
}

func (c *IOCheck) nixIO() error {
	sender, err := aggregator.GetSender(c.ID())
	if err != nil {
		return err
	}
	// See: https://www.xaprb.com/blog/2010/01/09/how-linux-iostat-computes-its-results/
	//      https://www.kernel.org/doc/Documentation/iostats.txt
	iomap, err := ioCounters()
	if err != nil {
		log.Errorf("system.IOCheck: could not retrieve io stats: %s", err)
		return err
	}

	// tick in millisecond
	now := nowNano() / 1000000
	delta := float64(now - c.ts)
	deltaSecond := delta / 1000

	var tagbuff bytes.Buffer
	for device, ioStats := range iomap {
		if c.blacklist != nil && c.blacklist.MatchString(device) {
			continue
		}

		tagbuff.Reset()
		tagbuff.WriteString("device:")
		if ioStats.Label != "" {
			tagbuff.WriteString(ioStats.Label)
		} else {
			tagbuff.WriteString(device)
		}
		tags := []string{tagbuff.String()}

		sender.Rate("system.io.r_s", float64(ioStats.ReadCount), "", tags)
		sender.Rate("system.io.w_s", float64(ioStats.WriteCount), "", tags)
		sender.Rate("system.io.rrqm_s", float64(ioStats.MergedReadCount), "", tags)
		sender.Rate("system.io.wrqm_s", float64(ioStats.MergedWriteCount), "", tags)

		if c.ts == 0 {
			continue
		}
		lastIOStats, ok := c.stats[device]
		if !ok {
			log.Debug("New device stats (possible hotplug) - full stats unavailable this iteration.")
			continue
		}

		if delta == 0 {
			log.Debug("No delta to compute - skipping.")
			continue
		}

		// computing kB/s
		rkbs := float64(ioStats.ReadBytes-lastIOStats.ReadBytes) / kB / deltaSecond
		wkbs := float64(ioStats.WriteBytes-lastIOStats.WriteBytes) / kB / deltaSecond
		avgqusz := float64(ioStats.WeightedIO-lastIOStats.WeightedIO) / kB / deltaSecond

		rAwait := 0.0
		wAwait := 0.0
		diffNRIO := float64(ioStats.ReadCount - lastIOStats.ReadCount)
		diffNWIO := float64(ioStats.WriteCount - lastIOStats.WriteCount)
		if diffNRIO != 0 {
			rAwait = float64(ioStats.ReadTime-lastIOStats.ReadTime) / diffNRIO
		}
		if diffNWIO != 0 {
			wAwait = float64(ioStats.WriteTime-lastIOStats.WriteTime) / diffNWIO
		}

		avgrqsz := 0.0
		aWait := 0.0
		diffNIO := diffNRIO + diffNWIO
		if diffNIO != 0 {
			avgrqsz = float64((ioStats.ReadBytes-lastIOStats.ReadBytes+ioStats.WriteBytes-lastIOStats.WriteBytes)/SectorSize) / diffNIO
			aWait = float64(ioStats.ReadTime-lastIOStats.ReadTime+ioStats.WriteTime-lastIOStats.WriteTime) / diffNIO
		}

		// we are aligning ourselves with the metric reported by
		// sysstat, so itv is a time interval in 1/100th of a second
		itv := delta / 10
		tput := diffNIO * 100 / itv
		util := float64(ioStats.IoTime-lastIOStats.IoTime) / itv * 100
		svctime := 0.0
		if tput != 0 {
			svctime = util / tput
		}

		sender.Gauge("system.io.rkb_s", roundFloat(rkbs), "", tags)
		sender.Gauge("system.io.wkb_s", roundFloat(wkbs), "", tags)
		sender.Gauge("system.io.avg_rq_sz", roundFloat(avgrqsz), "", tags)
		sender.Gauge("system.io.await", roundFloat(aWait), "", tags)
		sender.Gauge("system.io.r_await", roundFloat(rAwait), "", tags)
		sender.Gauge("system.io.w_await", roundFloat(wAwait), "", tags)
		sender.Gauge("system.io.avg_q_sz", roundFloat(avgqusz), "", tags)
		sender.Gauge("system.io.svctm", roundFloat(svctime), "", tags)

		// Stats should be per device no device groups.
		// If device groups ever become a thing - util / 10.0 / n_devs_in_group
		// See more: (https://github.com/sysstat/sysstat/blob/v11.5.6/iostat.c#L1033-L1040)
		sender.Gauge("system.io.util", roundFloat(util/10.0), "", tags)

	}

	c.stats = iomap
	c.ts = now
	return nil
}

// Run executes the check
func (c *IOCheck) Run() error {
	sender, err := aggregator.GetSender(c.ID())
	if err != nil {
		return err
	}
	err = c.nixIO()

	if err == nil {
		sender.Commit()
	}
	return err
}
