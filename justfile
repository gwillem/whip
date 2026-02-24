# Version from git tags, can be overridden
version := `git describe --tags --abbrev=0 2>/dev/null || echo "dev"`
dev_version := `git describe --tags 2>/dev/null || echo "dev"` + "-DEV"

build_dir := "build"

# Cross-platform sha256 (macOS uses shasum, Linux uses sha256sum)
sha256 := if `command -v sha256sum >/dev/null 2>&1 && echo y || echo n` == "y" { "sha256sum" } else { "shasum -a 256" }

# Platforms
linux_arm64 := "linux-arm64"
linux_amd64 := "linux-amd64"
darwin_arm64 := "darwin-arm64"
darwin_amd64 := "darwin-amd64"

# Full build for all platforms
default: build

# Development build (local platform + linux deputies)
devbuild: deputies
    @echo "Building whip for darwin-arm64..."
    @mkdir -p {{ build_dir }}/{{ darwin_arm64 }}
    GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.buildVersion={{ dev_version }}" -o {{ build_dir }}/{{ darwin_arm64 }}/whip ./cmd/whip

# Full build for all platforms
build v=version: clean (deputies v)
    @echo "Building whip for all platforms..."
    @mkdir -p {{ build_dir }}/{{ linux_arm64 }} {{ build_dir }}/{{ linux_amd64 }} {{ build_dir }}/{{ darwin_arm64 }} {{ build_dir }}/{{ darwin_amd64 }}
    GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.buildVersion={{ v }}" -o {{ build_dir }}/{{ linux_arm64 }}/whip ./cmd/whip
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.buildVersion={{ v }}" -o {{ build_dir }}/{{ linux_amd64 }}/whip ./cmd/whip
    GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.buildVersion={{ v }}" -o {{ build_dir }}/{{ darwin_arm64 }}/whip ./cmd/whip
    GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.buildVersion={{ v }}" -o {{ build_dir }}/{{ darwin_amd64 }}/whip ./cmd/whip

# Build deputies (linux only, embedded in whip)
deputies v=version:
    @echo "Building deputies..."
    @mkdir -p {{ build_dir }}/{{ linux_arm64 }} {{ build_dir }}/{{ linux_amd64 }} cmd/whip/deputies
    GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.buildVersion={{ v }}" -o {{ build_dir }}/{{ linux_arm64 }}/deputy ./cmd/deputy
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.buildVersion={{ v }}" -o {{ build_dir }}/{{ linux_amd64 }}/deputy ./cmd/deputy
    xz -c {{ build_dir }}/{{ linux_arm64 }}/deputy > cmd/whip/deputies/linux-arm64
    xz -c {{ build_dir }}/{{ linux_amd64 }}/deputy > cmd/whip/deputies/linux-amd64
    {{ sha256 }} {{ build_dir }}/{{ linux_arm64 }}/deputy > cmd/whip/deputies/linux-arm64.sha256
    {{ sha256 }} {{ build_dir }}/{{ linux_amd64 }}/deputy > cmd/whip/deputies/linux-amd64.sha256

# Create GitHub release (auto-increments version if not specified)
release release_version="":
    #!/usr/bin/env bash
    set -euo pipefail
    git fetch --tags
    rv="{{ release_version }}"
    if [ -z "$rv" ]; then
        latest=$(git describe --tags --abbrev=0)
        current=${latest#v}
        next=$(echo $current | awk -F. '{$NF = $NF + 1;} 1' | sed 's/ /./g')
        echo "No version provided. Using next version: v$next"
        rv="v$next"
    fi
    just build "$rv"
    mkdir -p {{ build_dir }}/github
    gzip -9c {{ build_dir }}/{{ linux_arm64 }}/whip > {{ build_dir }}/github/whip-Linux-aarch64.gz
    gzip -9c {{ build_dir }}/{{ linux_amd64 }}/whip > {{ build_dir }}/github/whip-Linux-x86_64.gz
    gzip -9c {{ build_dir }}/{{ darwin_arm64 }}/whip > {{ build_dir }}/github/whip-Darwin-arm64.gz
    gzip -9c {{ build_dir }}/{{ darwin_amd64 }}/whip > {{ build_dir }}/github/whip-Darwin-x86_64.gz
    gh release create "$rv" --generate-notes {{ build_dir }}/github/*
    git fetch --tags
    echo ""
    echo "Install command:"
    echo "base=https://github.com/gwillem/whip/releases/latest/download/whip"
    echo 'curl -L $base-$(uname -s)-$(uname -m).gz|gzip -d>whip&&chmod +x whip'

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Clean build artifacts
clean:
    rm -rf {{ build_dir }}/*
