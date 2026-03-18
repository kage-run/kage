# CLAUDE.md — Kage

## Project Overview

Kage is a daemon for Linux that secures, observes, and remotely controls AI agents. It wraps any agent process and enforces a three-tier permission policy (allow/deny/ask) on filesystem, network, and process execution at the OS kernel level. With first-class Claude Code support, you can start agents, send tasks, approve permissions, check output, and stop agents — all from your phone via Telegram.

Read PLAN.md for full architecture, implementation phases, and design decisions.

## Tech Stack

- **Language:** Go (core daemon, CLI, proxy, enforcement)
- **Frontend:** TypeScript + React + Tailwind (dashboard, embedded in Go binary)
- **Storage:** SQLite (audit log, pending approvals)
- **Config:** YAML (policy files, grants, saved agent configs)
- **Build:** Make + go:embed for frontend assets

## Key Architecture Decisions

1. **Three-tier enforcement:** `allow` (kernel-level pass via Landlock/seccomp/nftables), `deny` (kernel-level block), `ask` (userspace proxy returns informative 403 that the LLM reads and acts on).

2. **Daemon architecture.** A long-lived daemon process (`kage daemon start`) owns all agent supervisors, output ring buffers, adapters, the audit DB, proxy, dashboard server, and notification handlers. CLI commands (`kage run`, `kage ps`, `kage output`, `kage task`, etc.) are thin clients that talk to the daemon over a Unix socket at `~/.kage/kage.sock` (user) or `/var/run/kage/kage.sock` (root). Auto-starts on first `kage run` if not already running.

3. **Single binary.** The Go binary includes the CLI, daemon, proxy, audit logger, notification system, and embedded dashboard.

4. **Sandbox as a reusable struct.** `NewSandbox(name, policy) → Sandbox` with `Start()`/`Stop()`/`Status()`/`Output(n)` methods. The daemon creates one Sandbox per agent. The future orchestrator (`kage up`) creates N.

5. **Agent-scoped everything.** Every internal data structure includes an `agent_id` field: audit events, grants, approval requests, WebSocket events, dashboard API. Defaults to "default" for single-agent use.

6. **Agent adapters are optional.** The `--agent` flag on `kage run` activates an adapter for a specific agent. Without it: sandboxing + monitoring only. With it: can also send tasks and check structured status. Claude Code adapter ships in the MVP.

7. **Grants have TTLs.** When a user approves a resource, they choose: once, 1h, 24h, or permanent. Grants merge with static policy at runtime. Policy self-trains over time.

8. **Output capture.** Supervisor tees agent stdout/stderr to terminal (if attached) and a ring buffer (default 500 lines). Exposed via `kage output`, dashboard API, and Telegram `/output`.

## Directory Structure

```
cmd/kage/               → CLI entrypoint (cobra)
cmd/kage-testpilot/     → Test binary for verifying enforcement
internal/daemon/        → Long-lived daemon process, Unix socket API, saved configs
internal/sandbox/       → Sandbox struct, supervisor goroutine, ring buffer
internal/policy/        → YAML parsing, grant management
internal/enforce/       → Landlock, seccomp, nftables enforcement
internal/proxy/         → Transparent HTTP proxy (ask flow), DNS interceptor
internal/adapter/       → Agent adapters (claude-code in MVP)
internal/audit/         → SQLite audit logger
internal/notify/        → Telegram/Slack/webhook notifications + Telegram commands
internal/approval/      → Pending approval state machine
internal/dashboard/     → HTTP server, WebSocket, go:embed
web/                    → React dashboard frontend
configs/                → Default and example policy files
testdata/               → Test policy YAML files
scripts/                → Install and packaging scripts
```

## Build Commands

```bash
make build          # Build Go binary (includes frontend)
make test           # Run all tests
make frontend       # Build frontend only
make dev            # Run with hot reload (air)
make deb            # Build .deb package
make lint           # Run golangci-lint
```

## Development Notes

- Develop on macOS, cross-compile with `GOOS=linux GOARCH=amd64 go build`, test on Ubuntu machine (kernel 6.8).
- Integration tests need Linux with Landlock (5.13+). Full features require 6.2+ (Landlock v3 with network rules).
- The proxy uses iptables REDIRECT — needs NET_ADMIN capability.
- Frontend dev: `cd web && npm run dev` for Vite dev server, proxies API to Go backend.

## Code Style

- Go: follow standard Go conventions, use `golangci-lint`
- Error handling: wrap errors with context using `fmt.Errorf("doing X: %w", err)`
- Logging: use `slog` (stdlib structured logging)
- Tests: table-driven tests, use testify for assertions where it improves clarity
- Frontend: functional React components, hooks, Tailwind utility classes
