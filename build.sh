#!/bin/bash
GROUP_ID=$(id -g)
USER_ID=$(id -u)
SOURCE_DIR=${PWD}
TARGET_DIR=/go/src/github.com/DataDog/datadog-agent
CONTAINER_NAME=datadog-agent-build
DOCKER_IMAGE=golang:1.10

if [ "$(docker ps -q -f name=${CONTAINER_NAME})" ]; then
  echo "Build already running"
  exit 1
fi

if [ "$(docker ps -aq -f status=exited -f name=${CONTAINER_NAME})" ]; then
  docker start -a ${CONTAINER_NAME}
else
  docker run \
    --name=${CONTAINER_NAME} \
    -v ${SOURCE_DIR}:${TARGET_DIR} \
    -w ${TARGET_DIR} \
    ${DOCKER_IMAGE} \
    /bin/bash ${TARGET_DIR}/build-start.sh ${GROUP_ID} ${USER_ID} ${TARGET_DIR}
fi