# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Whip is a fast, simple Ansible replacement for server configuration management, optimized for projects with 1-20 servers. It bundles tasks into a single job to eliminate SSH round trips that make Ansible slow.

## Build Commands

```bash
make devbuild          # Dev build (deputy linux + whip darwin-arm64)
make build             # Full build (all platforms: linux/darwin, amd64/arm64)
make test              # Run all tests (go test ./...)
make release           # GitHub release (auto-increments version)

# Run a specific test
go test ./internal/runners -run TestTree
```

## Architecture

### Two-Binary Model

- **whip** (`cmd/whip/`): Controller on local machine. Loads playbook, connects to targets via SSH, streams results. Embeds xz-compressed deputy binaries via `//go:embed`.
- **deputy** (`cmd/deputy/`): Agent on target servers. Receives gob-encoded jobs via stdin, executes tasks, streams `TaskResult` back via stdout.

### Data Flow

1. Whip loads playbook YAML → parses into `model.Playbook` via mapstructure decode hooks
2. Runs PreRun phase on controller (e.g., tree runner loads file assets via `assets.DirToAsset`)
3. Creates `model.Job` per target host containing plays, vars, and embedded assets
4. Job is gob-encoded, zstd-compressed, streamed to deputy over SSH
5. Deputy decodes job, runs each task through runners, streams `model.TaskResult` back
6. Whip displays progress with bubbletea TUI

### Key Packages

- `internal/model/` — Core types: Job, Playbook, Play, Task, TaskResult. Wire types must be registered with `gob.Register()`.
- `internal/playbook/` — YAML parsing with mapstructure decode hooks. Task maps are matched against registered runner names.
- `internal/runners/` — Task executors (apt, shell, command, tree, service, etc.)
- `internal/ssh/` — SSH connection, SFTP uploads, generic gob streaming via `RunGobStreamer[T]()`
- `internal/vault/` — Secrets encryption (Age preferred, Ansible Vault supported)
- `internal/assets/` — File embedding with `afero` in-memory FS and zstd compression

### Runners

Each runner registers itself in `init()` via `registerRunner()`. Runners implement `runnerFunc` signature (`func(*model.Task) model.TaskResult`) and return status codes (`Success`, `Failed`, `Skipped`).

To add a new runner: create a file in `internal/runners/`, register in `init()` with a `runner` struct containing `run` (required), and optionally `prerun` (runs on controller), `validate`, and `meta` (required/optional args). See `shell.go` for a minimal example or `tree.go` for a complex one with prerun.

Runners use `afero.Fs` (package-level `fs` and `fsutil` vars) for filesystem operations. Tests swap in `afero.MemMapFs` for isolation.

### Template Engine

Task arguments support Jinja2-style templates via Gonja (`{{ variable }}`). Configured with `StrictUndefined = true` — undefined variables cause errors. Variables are merged: job vars → play vars → task vars (each level overrides previous).

### Playbook Location

Default playbook path: `.whip/playbook.yml` (searched in current and parent directories)

## Vault/Secrets

- `WHIP_KEY` env var for Age encryption (preferred)
- `ANSIBLE_VAULT_PASSWORD` for legacy Ansible vault support
- `whip edit <file>` to edit encrypted files
- `whip convert <file>` to migrate from Ansible Vault to Age
