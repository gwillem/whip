#!/bin/bash

targets="linux,amd64 linux,arm64 darwin,arm64 darwin,amd64" # linux,arm64


for cmd in deputy chief; do
    for x in $targets; do
        # alternatively use bash arrays: https://www.digitalocean.com/community/tutorials/how-to-build-go-executables-for-multiple-platforms-on-ubuntu-16-04
        os=$(echo $x | cut -d, -f1)
        arch=$(echo $x | cut -d, -f2)

        dst="build/$os-$arch"
        mkdir -p $dst
        echo $dst/$cmd ..

        env GOOS=$os GOARCH=$arch go build -o $dst/$cmd ./cmd/$cmd

        if [ "$cmd-$os" = "deputy-linux" ]; then
            cp $dst/$cmd cmd/chief/deputies/$os-$arch
        fi

    done
done

