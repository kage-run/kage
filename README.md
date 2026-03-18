# Kage

**Secure, observe, and remotely control your AI agents from anywhere.**

Kage is a daemon that wraps any AI agent process — Claude Code, Cursor, Aider, custom LangChain/CrewAI agents, raw Python scripts — and provides kernel-level security, real-time observability, and remote control from your phone.

```bash
# Start an agent in a sandbox
kage run --name coder -- claude

# Check what it's doing
kage ps
kage output coder

# Stop it
kage stop coder
```

## Why Kage?

AI agents run commands, read files, and make network requests on your machine. You need to know what they're doing and control what they're allowed to do — even when you're away from your desk.

**Kernel-level security.** Three-tier permission policy (allow/deny/ask) on filesystem, network, and process execution. Enforced via Landlock, seccomp, and nftables. Unbypassable — even by prompt injection.

**Unified observability.** A single audit trail and real-time dashboard showing what every agent process is doing at the OS level.

**Remote control from your phone.** Start agents, send tasks, approve permissions, check output, and stop agents from Telegram. No SSH needed.

**Works with any agent.** Kage operates at the OS level. It doesn't care what framework your agent uses. If it runs as a process, Kage can sandbox it.

## How It Works

```
┌─────────────────────────────────────────────┐
│              Kage Daemon                     │
│                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ Sandbox  │  │ Sandbox  │  │ Sandbox  │  │
│  │ "coder"  │  │ "research"│  │ "monitor"│  │
│  │ Claude   │  │ Python   │  │ Custom   │  │
│  └──────────┘  └──────────┘  └──────────┘  │
│                                              │
│  Unix Socket API ← CLI commands talk here    │
│  Dashboard       ← Real-time web UI         │
│  Audit Log       ← SQLite event trail       │
│  Notifications   ← Telegram/Slack           │
└─────────────────────────────────────────────┘
```

The Kage daemon is a single long-lived process. CLI commands (`kage run`, `kage ps`, `kage output`, etc.) are thin clients that talk to the daemon over a Unix socket. The daemon auto-starts on your first `kage run`.

## Quick Start

### Build from source

```bash
git clone https://github.com/kage-run/kage.git
cd kage/kage
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

### With a security policy

```bash
# Run with a policy file
kage run --name coder --policy policies/claude-code.yaml -- claude
```

Example policy (`policies/claude-code.yaml`):

```yaml
version: 1

filesystem:
  allow_read:
    - /home/user/projects/myapp
    - /tmp
  allow_write:
    - /home/user/projects/myapp
    - /tmp
  deny:
    - "**/.ssh"
    - "**/.aws"
    - "**/.gnupg"

network:
  allow:
    - api.anthropic.com
    - api.github.com
  deny:
    - "169.254.*"
    - "10.*"
  ask:
    default: ask

process:
  allow:
    - node
    - python3
    - bash
    - git
  deny:
    - sudo
    - su
    - apt
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `kage run --name <name> -- <cmd>` | Start a sandboxed agent process |
| `kage run -d --name <name> -- <cmd>` | Start in background (detached) |
| `kage ps` | List all agents with state, PID, uptime |
| `kage output <name>` | Show last 20 lines of agent output |
| `kage output <name> -f` | Stream agent output in real-time |
| `kage output <name> -n 100` | Show last 100 lines |
| `kage stop <name>` | Gracefully stop an agent |
| `kage daemon start` | Start the daemon (foreground) |
| `kage daemon start -d` | Start the daemon (background) |
| `kage daemon stop` | Stop the daemon and all agents |
| `kage daemon status` | Show daemon version, uptime, agent count |

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Core daemon + CLI | Go |
| Kernel enforcement | Landlock LSM, seccomp-bpf, nftables |
| Network proxy | Go net/http (SNI inspection, readable 403s) |
| Audit log | SQLite |
| Dashboard | React + TypeScript + Tailwind (embedded in binary) |
| Notifications | Telegram Bot API, Slack Webhooks |
| Config | YAML |

## Project Structure

```
kage/
├── cmd/kage/              # CLI entrypoint (cobra)
├── internal/
│   ├── daemon/            # Daemon process, Unix socket server/client
│   ├── sandbox/           # Sandbox struct, supervisor, ring buffer
│   ├── policy/            # YAML policy parsing
│   ├── enforce/           # Landlock, seccomp, nftables (coming soon)
│   ├── proxy/             # HTTP/TLS proxy for ask flow (coming soon)
│   ├── audit/             # SQLite audit logger (coming soon)
│   ├── notify/            # Telegram/Slack notifications (coming soon)
│   ├── approval/          # Approval state machine (coming soon)
│   ├── adapter/           # Agent adapters - Claude Code (coming soon)
│   └── dashboard/         # Web dashboard (coming soon)
├── testdata/              # Test policy files
├── plans/                 # Implementation plans
├── go.mod
├── Makefile
└── CLAUDE.md              # AI context file
```

## Roadmap

### Step 1: Daemon + Process Supervision (current)
The foundational daemon architecture. `kage run`, `kage ps`, `kage stop`, `kage output` work end-to-end over the Unix socket protocol.

### Step 2: Kernel Enforcement
Landlock filesystem restrictions, seccomp process filtering, nftables network isolation. The parsed policy YAML becomes actionable — denied operations fail at the kernel level.

### Step 3: Audit Trail
SQLite event logger recording every allow/deny decision. `kage log` command to query and tail events.

### Step 4: Ask Flow + Proxy
Transparent HTTP/TLS proxy returning readable 403s that LLMs understand. DNS interceptor. Terminal-based approval UI.

### Step 5: Dashboard
Embedded web UI with real-time action stream, agent list, approval buttons, policy status.

### Step 6: Telegram Integration
Start agents, send tasks, approve permissions, check output — all from your phone.

### Step 7: Claude Code Adapter
First-class integration: send coding tasks, check structured output, manage sessions.

### Step 8: Fleet Orchestration (`kage.yaml`)
Declarative multi-agent fleet definition. `kage up` brings up your entire agent stack.

## Requirements

- **Linux** with kernel 5.13+ (Landlock support). Full features require kernel 6.2+ (Landlock v3 with network rules).
- **Go 1.24+** for building from source.

## License

TBD
