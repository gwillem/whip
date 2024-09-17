#!/bin/bash

cd $(dirname "$0")/..

source "$(dirname "$0")/build.sh"

git fetch --tags

# $version is set by release.sh script
export VERSION=${version:-$(git describe --tags --abbrev=0)}

find build -type f -delete

build_for_target deputy linux arm64
build_for_target deputy linux amd64
build_for_target whip darwin arm64
build_for_target whip darwin amd64
build_for_target whip linux arm64
build_for_target whip linux amd64


