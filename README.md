# govpn

A minimal, cross-platform Layer-3 VPN written in Go — zero runtime dependencies beyond the standard library and a single vendored TUN helper.

```
sudo govpn server --config server.json
sudo govpn client --config client.json
```

---

## Features

- **AES-256-GCM** authenticated encryption — hardware-accelerated on amd64 / arm64
- **UDP transport** — avoids TCP-in-TCP meltdown
- **Cross-platform** — Linux (iproute2), macOS (ifconfig/route), Windows (netsh + Wintun)
- **Structured logging** — `log/slog` with selectable verbosity
- **Context-aware** — clean shutdown on `SIGINT`/`SIGTERM`, race-detector clean
- **No CGo** — pure Go, single static binary

---

## Installation

```bash
# From source (requires Go 1.22+)
git clone https://github.com/govpn/govpn
cd govpn
make build           # → dist/govpn

# Install to $GOPATH/bin
make install
```

---

## Quick start

### 1 — Generate configs

```bash
# On the server machine
govpn init --mode server --out server.json

# On the client machine
govpn init --mode client --out client.json
```

### 2 — Edit the configs

**Both sides** — set the same `passphrase`.  
**`client.json`** — set `server_addr` to your server's public IP and port.

### 3 — Run

```bash
# Server (needs root for TUN access)
sudo govpn server --config server.json

# Client
sudo govpn client --config client.json
```

---

## CLI reference

```
govpn <command> [flags]

Commands:
  server    Start a VPN server node
  client    Connect to a VPN server
  init      Generate a config file
  version   Print version and build info
```

### `govpn server`

| Flag | Description |
|---|---|
| `--config <file>` | Path to server JSON config **(required)** |
| `--verbose` | Enable per-packet debug logging |

### `govpn client`

| Flag | Description |
|---|---|
| `--config <file>` | Path to client JSON config **(required)** |
| `--verbose` | Enable per-packet debug logging |

### `govpn init`

| Flag | Description |
|---|---|
| `--mode server\|client` | Node mode **(required)** |
| `--out <file>` | Write to file (default: stdout) |

---

## Config reference

```jsonc
{
  "mode": "server",           // "server" or "client"
  "listen_addr": "0.0.0.0:1194",  // server: UDP bind address
  "server_addr": "1.2.3.4:1194",  // client: server's public address
  "tun_name":  "govpn0",      // interface name hint (Linux only)
  "local_ip":  "10.8.0.1/24", // this node's VPN IP (CIDR)
  "peer_ip":   "10.8.0.2",    // remote VPN IP (point-to-point, macOS/Windows)
  "routes": ["10.8.0.0/24"],  // subnets to route through the tunnel
  "passphrase": "…",          // pre-shared secret — same on both sides
  "mtu": 1400,                // TUN MTU (lower than 1500 for UDP overhead)
  "keepalive_interval": "15s",// client PING interval (keep NAT alive)
  "verbose": false            // per-packet slog debug output
}
```

Route all internet traffic through the tunnel:

```json
"routes": ["0.0.0.0/1", "128.0.0.0/1"]
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/govpn         CLI entry point, subcommand dispatch     │
├─────────────────────────────────────────────────────────────┤
│  internal/vpn      Node — wires all layers together         │
├──────────────┬──────────────┬───────────────┬──────────────┤
│  tunnel      │  device      │  routing      │  cipher      │
│  UDP framing │  TUN r/w     │  OS routes    │  AES-256-GCM │
│  keepalives  │  PacketDevice│  Manager iface│  AEAD wrap   │
│  stats       │  interface   │  per-platform │              │
├──────────────┴──────────────┴───────────────┴──────────────┤
│  config            JSON load, validation, sentinel errors   │
└─────────────────────────────────────────────────────────────┘
```

### Package responsibilities

| Package | Responsibility |
|---|---|
| `internal/cipher` | AES-256-GCM encrypt/decrypt; key derivation; sentinel errors |
| `internal/config` | JSON config load/validate; human-readable Duration type |
| `internal/device` | `PacketDevice` interface; TUN open/read/write; platform name injection |
| `internal/routing` | `Manager` interface; per-OS interface config and route management |
| `internal/tunnel` | UDP framing; context-aware pump goroutines; keepalives; stats |
| `internal/vpn` | `Node` — composes all layers; lifecycle (open → run → cleanup) |
| `cmd/govpn` | Subcommand CLI; spinner UI; structured flag help |

---

## Wire format

```
+----------+----------+--------------------+
| type (1) | len  (2) | payload (variable) |
+----------+----------+--------------------+

0x01  DATA  →  nonce(12) || AES-256-GCM ciphertext || tag(16)
0x02  PING  →  no payload  (client keepalive)
0x03  PONG  →  no payload  (server reply)
```

---

## Platform notes

### Linux
Uses `ip` from **iproute2**. Requires `CAP_NET_ADMIN` (run with `sudo`).  
The `tun_name` config field is honoured.

### macOS
TUN interfaces are always named `utunN` by the kernel — `tun_name` is ignored.  
Uses `ifconfig` + `route`. Requires `sudo`.

### Windows
Requires **Wintun** (the WireGuard TUN driver): <https://www.wintun.net/>  
Uses `netsh` + `route`. Must run as Administrator.

---

## Development

```bash
make test        # full test suite
make test-race   # with -race detector (zero races)
make bench       # cipher throughput benchmarks
make lint        # go vet
make build       # dist/govpn binary
```

### Running without root (for tests)

All tunnel tests use an in-memory `mockDevice` and real UDP loopback sockets — no TUN device, no root required.

```bash
go test ./internal/... -v -timeout 30s
```

### Build with version tag

```bash
make build VERSION=0.2.0
# or
go build -ldflags "-X main.Version=0.2.0" -o govpn ./cmd/govpn
```

### Adding a new OS

1. `internal/routing/routing_<goos>.go` — implement `Manager` with a `//go:build <goos>` tag
2. `internal/device/name_<goos>.go` — implement `applyName` (or reuse `name_other.go` no-op)

---

## Security notes

| Area | Current | Production recommendation |
|---|---|---|
| Key derivation | SHA-256(passphrase) | [Argon2id](https://pkg.go.dev/golang.org/x/crypto/argon2) with stored salt |
| Authentication | Per-packet AES-256-GCM | + mutual TLS or Noise handshake |
| Replay protection | None | Sequence numbers + sliding-window bitmap |
| Multi-client auth | Shared pre-shared key | Per-client keys or certificate PKI |

---

## Project layout

```
govpn/
├── cmd/govpn/              # CLI binary
│   ├── main.go             # entry point and top-level help
│   ├── cmd_run.go          # server / client subcommands
│   ├── cmd_init.go         # init subcommand
│   ├── cmd_version.go      # version subcommand
│   ├── flags.go            # shared FlagSet factory
│   ├── ui.go               # spinner, colors, table printer
│   ├── tty_unix.go         # TTY detection — Linux/macOS
│   ├── tty_windows.go      # TTY detection — Windows
│   └── tty_other.go        # TTY detection — fallback
├── internal/
│   ├── cipher/
│   │   ├── cipher.go       # AES-256-GCM AEAD
│   │   └── cipher_test.go
│   ├── config/
│   │   ├── config.go       # Config struct, Load, Validate, sentinel errors
│   │   └── config_test.go
│   ├── device/
│   │   ├── device.go       # PacketDevice interface + TUN implementation
│   │   ├── name_linux.go   # Linux: honour tun_name
│   │   └── name_other.go   # macOS/Windows: no-op (OS assigns name)
│   ├── routing/
│   │   ├── routing.go      # Manager interface + ErrNotSupported
│   │   ├── routing_linux.go
│   │   ├── routing_darwin.go
│   │   ├── routing_windows.go
│   │   └── routing_other.go
│   ├── tunnel/
│   │   ├── tunnel.go       # UDP framing, pump goroutines, keepalives, stats
│   │   └── tunnel_test.go  # in-memory mock, loopback tests, race-clean
│   └── vpn/
│       └── vpn.go          # Node — composes all layers into a runnable VPN
├── configs/
│   ├── server.json         # example server config
│   └── client.json         # example client config
├── vendor/                 # vendored dependencies
├── Makefile
├── go.mod
├── go.sum
└── README.md
```
