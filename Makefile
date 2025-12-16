.PHONY: all build devbuild release test clean deputies

# Version from git tags, can be overridden
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
DEV_VERSION = $(shell git describe --tags 2>/dev/null || echo "dev")-DEV

BUILD_DIR = build
LDFLAGS = -ldflags="-s -w -X main.buildVersion=$(VERSION)"

# Cross-platform sha256 (macOS uses shasum, Linux uses sha256sum)
SHA256 = $(shell command -v sha256sum >/dev/null 2>&1 && echo "sha256sum" || echo "shasum -a 256")

# Platforms
LINUX_ARM64 = linux-arm64
LINUX_AMD64 = linux-amd64
DARWIN_ARM64 = darwin-arm64
DARWIN_AMD64 = darwin-amd64

all: build

# Development build (local platform + linux deputies)
devbuild: VERSION = $(DEV_VERSION)
devbuild: deputies
	@echo "Building whip for darwin-arm64..."
	@mkdir -p $(BUILD_DIR)/$(DARWIN_ARM64)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DARWIN_ARM64)/whip ./cmd/whip

# Full build for all platforms
build: clean deputies
	@echo "Building whip for all platforms..."
	@mkdir -p $(BUILD_DIR)/$(LINUX_ARM64) $(BUILD_DIR)/$(LINUX_AMD64) $(BUILD_DIR)/$(DARWIN_ARM64) $(BUILD_DIR)/$(DARWIN_AMD64)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(LINUX_ARM64)/whip ./cmd/whip
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(LINUX_AMD64)/whip ./cmd/whip
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DARWIN_ARM64)/whip ./cmd/whip
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(DARWIN_AMD64)/whip ./cmd/whip

# Build deputies (linux only, embedded in whip)
deputies:
	@echo "Building deputies..."
	@mkdir -p $(BUILD_DIR)/$(LINUX_ARM64) $(BUILD_DIR)/$(LINUX_AMD64) cmd/whip/deputies
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(LINUX_ARM64)/deputy ./cmd/deputy
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(LINUX_AMD64)/deputy ./cmd/deputy
	xz -c $(BUILD_DIR)/$(LINUX_ARM64)/deputy > cmd/whip/deputies/linux-arm64
	xz -c $(BUILD_DIR)/$(LINUX_AMD64)/deputy > cmd/whip/deputies/linux-amd64
	$(SHA256) $(BUILD_DIR)/$(LINUX_ARM64)/deputy > cmd/whip/deputies/linux-arm64.sha256
	$(SHA256) $(BUILD_DIR)/$(LINUX_AMD64)/deputy > cmd/whip/deputies/linux-amd64.sha256

# Create GitHub release
# Files are named to match uname -s and uname -m output for the install script
release:
	@git fetch --tags
	@if [ -z "$(RELEASE_VERSION)" ]; then \
		latest=$$(git describe --tags --abbrev=0); \
		current=$${latest#v}; \
		next=$$(echo $$current | awk -F. '{$$NF = $$NF + 1;} 1' | sed 's/ /./g'); \
		echo "No RELEASE_VERSION provided. Using next version: v$$next"; \
		$(MAKE) release RELEASE_VERSION=v$$next; \
	else \
		$(MAKE) build VERSION=$(RELEASE_VERSION); \
		mkdir -p $(BUILD_DIR)/github; \
		gzip -9c $(BUILD_DIR)/$(LINUX_ARM64)/whip > $(BUILD_DIR)/github/whip-Linux-aarch64.gz; \
		gzip -9c $(BUILD_DIR)/$(LINUX_AMD64)/whip > $(BUILD_DIR)/github/whip-Linux-x86_64.gz; \
		gzip -9c $(BUILD_DIR)/$(DARWIN_ARM64)/whip > $(BUILD_DIR)/github/whip-Darwin-arm64.gz; \
		gzip -9c $(BUILD_DIR)/$(DARWIN_AMD64)/whip > $(BUILD_DIR)/github/whip-Darwin-x86_64.gz; \
		gh release create $(RELEASE_VERSION) --generate-notes $(BUILD_DIR)/github/*; \
		git fetch --tags; \
		echo ""; \
		echo "Install command:"; \
		echo "base=https://github.com/gwillem/whip/releases/latest/download/whip"; \
		echo "curl -L \$$base-\$$(uname -s)-\$$(uname -m).gz|gzip -d>whip&&chmod +x whip"; \
	fi

test:
	go test ./...

test-verbose:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)/*
