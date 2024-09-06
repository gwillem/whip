#!/bin/bash

source "$(dirname "$0")/build.sh"

git fetch --tags

# $version is set by release.sh script
export VERSION=${version:-$(git describe --tags --abbrev=0)}

build_for_target deputy linux arm64
build_for_target deputy linux amd64
build_for_target whip darwin arm64
build_for_target whip darwin amd64


