# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Whip is a fast, simple Ansible replacement for server configuration management, optimized for projects with 1-20 servers. It bundles tasks into a single job to eliminate SSH round trips that make Ansible slow.

## Build Commands

```bash
# Development build (builds deputy and whip for local platform)
make devbuild

# Full build (all platforms: linux/darwin, amd64/arm64)
make build

# Create a GitHub release (auto-increments version)
make release
# Or with specific version:
make release RELEASE_VERSION=v0.2.0

# Run all tests
make test

# Run a specific test
go test ./internal/runners -run TestTree
```

## Architecture

### Two-Binary Model

- **whip** (`cmd/whip/`): Controller that runs on your local machine. Loads playbook, connects to targets via SSH, and streams results.
- **deputy** (`cmd/deputy/`): Agent that runs on target servers. Receives gob-encoded jobs via stdin, executes tasks, and streams results back via stdout.

### Data Flow

1. Whip loads playbook YAML and parses into `model.Playbook`
2. Creates `model.Job` per target host containing plays, vars, and embedded file assets
3. Job is gob-encoded, zstd-compressed, and streamed to deputy over SSH
4. Deputy decodes job, runs each task through runners, streams `model.TaskResult` back
5. Whip displays progress with bubbletea TUI

### Key Packages

- `internal/model/` - Core types: Job, Playbook, Play, Task, TaskResult
- `internal/playbook/` - YAML parsing with mapstructure decode hooks
- `internal/runners/` - Task executors (apt, shell, command, tree, service, etc.)
- `internal/ssh/` - SSH connection, SFTP uploads, gob streaming
- `internal/vault/` - Secrets encryption (Age preferred, Ansible Vault supported)
- `internal/assets/` - File embedding and zstd compression

### Runners

Each runner registers itself in `init()` via `registerRunner()`. Runners implement `runnerFunc` signature and return `model.TaskResult`. The `tree` runner is notable - it handles file/copy/template operations with per-file state tracking.

### Playbook Location

Default playbook path: `.whip/playbook.yml` (searched in current and parent directories)

## Vault/Secrets

- Set `WHIP_KEY` env var for Age encryption (preferred)
- Set `ANSIBLE_VAULT_PASSWORD` for legacy Ansible vault support
- `whip edit <file>` to edit encrypted files
- `whip convert <file>` to migrate from Ansible Vault to Age
