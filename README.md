# Kage

**Secure, observe, and control your AI agents from anywhere.**

Kage is a daemon that wraps any AI agent process and provides process supervision, output capture, and a security policy framework. It works with any agent — Claude Code, Cursor, Aider, custom scripts — anything that runs as a process.

```bash
# Start an agent in a sandbox
kage run --name coder -- claude

# Check what it's doing
kage ps
kage output coder

# Stop it
kage stop coder
```

## Quick Start

### Build from source

```bash
git clone https://github.com/kage-run/kage.git
cd kage
make build
```

### Run your first sandboxed agent

```bash
# Run a command (daemon auto-starts)
./bin/kage run --name hello -- echo "hello from kage"

# Run in background
./bin/kage run -d --name coder -- claude

# Check running agents
./bin/kage ps

# View agent output
./bin/kage output coder
./bin/kage output coder -f    # follow mode (like tail -f)

# Stop an agent
./bin/kage stop coder
```

## How It Works

The Kage daemon is a single long-lived process. CLI commands are thin clients that talk to the daemon over a Unix socket. The daemon auto-starts on your first `kage run`.

```
┌─────────────────────────────────────────────┐
│              Kage Daemon                     │
│                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ Sandbox  │  │ Sandbox  │  │ Sandbox  │  │
│  │ "coder"  │  │ "research"│  │ "monitor"│  │
│  └──────────┘  └──────────┘  └──────────┘  │
│                                              │
│  Unix Socket API ← CLI commands talk here    │
└─────────────────────────────────────────────┘
```

Each sandbox supervises a child process, captures stdout/stderr to a ring buffer, and applies a security policy.

## CLI Reference

| Command | Description |
|---------|-------------|
| `kage run --name <name> -- <cmd>` | Start a sandboxed process |
| `kage run -d --name <name> -- <cmd>` | Start in background (detached) |
| `kage ps` | List all sandboxes with state, PID, uptime |
| `kage output <name>` | Show last 20 lines of output |
| `kage output <name> -f` | Stream output in real-time |
| `kage output <name> -n 100` | Show last 100 lines |
| `kage stop <name>` | Gracefully stop a sandbox |
| `kage daemon start` | Start the daemon (foreground) |
| `kage daemon stop` | Stop the daemon and all sandboxes |
| `kage daemon status` | Show daemon version, uptime, sandbox count |

## How is Kage different?

AI agents like OpenClaw run with the full privileges of the host user. A prompt injection can make them execute arbitrary commands, exfiltrate data, or access credentials — and OpenClaw's application-level approvals only cover tool calls that go through its own dispatch system. A direct `child_process.exec()` bypasses them entirely.

NemoClaw (NVIDIA) addresses this by wrapping OpenClaw in a Docker + K3s container with out-of-process policy enforcement via OpenShell. This works, but it adds significant deployment complexity — a full Kubernetes stack for a single agent.

Kage takes a different approach:

| | OpenClaw | NemoClaw / OpenShell | Kage |
|---|---|---|---|
| **Isolation** | None — runs as host user | Docker + K3s container | Kernel-level (Landlock, seccomp, nftables) |
| **Escape difficulty** | Trivial | Container escape (documented attack class) | Requires kernel exploit |
| **Overhead** | None | Docker + Kubernetes stack | Single daemon binary, negligible overhead |
| **Enforcement** | Application-level (bypassable) | Container policies (out-of-process) | Kernel primitives (irrevocable once applied) |
| **Observability** | Application logs | OpenShell violation logs | Kernel audit trail (tamper-resistant) |
| **Deployment** | `npm install` | Docker + K3s + YAML | Single binary, no containers |

Kage applies Landlock, seccomp, and nftables restrictions directly to the agent process. All child processes inherit these restrictions automatically — it's a one-way ratchet enforced by the kernel. No container runtime, no Kubernetes, no abstraction layers.

The tradeoff: Kage's kernel enforcement is Linux-specific. The daemon and process supervision work on macOS, but full security enforcement requires Linux 5.13+ (Landlock).

## Sponsors

[Deployment.io](https://deployment.io)

## Requirements

- **Go 1.24+** for building from source.
- **macOS or Linux.** The daemon and process supervision work on both. Kernel-level enforcement (Landlock, seccomp) is planned for Linux.

## License

MIT License - Copyright (c) 2026 Deployment AI Inc.
