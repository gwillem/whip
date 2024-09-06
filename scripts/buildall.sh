#!/bin/bash

source "$(dirname "$0")/build.sh"

git fetch --tags
export VERSION=$(git describe --tags)

build_for_target deputy linux arm64
build_for_target deputy linux amd64
build_for_target whip darwin arm64
build_for_target whip darwin amd64


