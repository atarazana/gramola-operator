#!/bin/sh

export GO111MODULE=on

export OPERATOR_NAME=gramola-operator
export OPERATOR_IMAGE=gramola-operator-image

export DOMAIN=atarazana.com
export API_VERSION=gramola.${DOMAIN}/v1

export PROJECT_NAME=${OPERATOR_NAME}-system

export USERNAME=cvicens

export VERSION=0.0.2