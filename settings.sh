#!/bin/sh

export GO111MODULE=on

export APP_NAME=gramola
export OPERATOR_NAME=${APP_NAME}-operator
export OPERATOR_IMAGE=${OPERATOR_NAME}-image

export ORGANIZATION=atarazana
export DOMAIN=${ORGANIZATION}.com
export API_VERSION=${APP_NAME}.${DOMAIN}/v1

export PROJECT_NAME=${OPERATOR_NAME}-system

export USERNAME=cvicens

export FROM_VERSION=0.0.1
export VERSION=0.0.2