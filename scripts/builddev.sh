#!/bin/bash

targets="linux,arm64 linux,amd64 darwin,arm64" # linux,arm64

for cmd in deputy whip; do
    for x in $targets; do
        # alternatively use bash arrays: https://www.digitalocean.com/community/tutorials/how-to-build-go-executables-for-multiple-platforms-on-ubuntu-16-04
        os=$(echo $x | cut -d, -f1)
        arch=$(echo $x | cut -d, -f2)

        if [ "$cmd-$os" == "deputy-darwin" ]; then
            continue
        fi

        if [ "$cmd-$os" == "whip-linux" ]; then
            continue
        fi

        dst="build/$os-$arch"
        mkdir -p $dst
        echo $dst/$cmd ..

        env GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o $dst/$cmd ./cmd/$cmd

        if [ "$cmd-$os" = "deputy-linux" ]; then
             cp $dst/$cmd cmd/whip/deputies/$os-$arch
            # sha256sum $dst/$cmd > cmd/whip/deputies/$os-$arch/$cmd.sha256
            # gzip --best --stdout $dst/$cmd > cmd/whip/deputies/$os-$arch
        fi

    done
done
