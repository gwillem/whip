#!/bin/bash

source "$(dirname "$0")/build.sh"

export VERSION="DEV-$(date +%Y%m%d-%H%M%S)"

# build_for_target deputy linux arm64
build_for_target deputy linux amd64
build_for_target whip darwin arm64
