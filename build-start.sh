#!/bin/bash

CONTAINER_USERNAME='dummy'
CONTAINER_GROUPNAME='dummy'
HOME_DIR='/home/'$CONTAINER_USERNAME
GROUP_ID=$1
USER_ID=$2
TARGET_DIR=$3

if [ ! "$(id -u ${CONTAINER_USERNAME})" ]; then
  groupadd -f -g $GROUP_ID $CONTAINER_GROUPNAME
  useradd -u $USER_ID -g $CONTAINER_GROUPNAME $CONTAINER_USERNAME
  mkdir --parent $HOME_DIR
  chown -R $CONTAINER_USERNAME:$CONTAINER_GROUPNAME $HOME_DIR

  cd ${TARGET_DIR}
  apt-get update
  apt-get install -y sudo python2.7-dev python-pip
  pip install -r requirements.txt

  chown -R $CONTAINER_USERNAME:$CONTAINER_GROUPNAME $GOPATH

  sudo -E -u ${CONTAINER_USERNAME} HOME=${HOME_DIR} PATH=${PATH} bash -c \
    'cd '${TARGET_DIR}' && \
    invoke deps'
fi

sudo -E -u ${CONTAINER_USERNAME} HOME=${HOME_DIR} PATH=${PATH} bash -c \
  'cd '${TARGET_DIR}' && \
  invoke agent.build --build-exclude=snmp,systemd'