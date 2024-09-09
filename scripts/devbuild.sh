#!/bin/bash

source "$(dirname "$0")/build.sh"

export VERSION="$(git describe --tags)-DEV"

build_for_target deputy linux arm64
build_for_target deputy linux amd64
build_for_target whip darwin arm64
