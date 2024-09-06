#!/bin/bash

git fetch --tags

build_for_target() {
    local cmd="$1"
    local os="$2"
    local arch="$3"

    dst="build/$os-$arch"
    mkdir -p $dst
    echo $dst/$cmd ..

    env GOOS=$os GOARCH=$arch go build -ldflags="-s -w -X main.buildVersion=$VERSION" -o $dst/$cmd ./cmd/$cmd

    if [ "$cmd-$os" = "deputy-linux" ]; then
        mkdir -p cmd/whip/deputies
        xz -c $dst/$cmd > cmd/whip/deputies/$os-$arch
        sha256sum $dst/$cmd > cmd/whip/deputies/$os-$arch.sha256
    fi
}
