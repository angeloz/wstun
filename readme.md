# wstun-go - WebSocket Tunneling for Embedded Systems

A high-performance Go implementation of wstun optimized for embedded devices like OpenWRT.

This repository contains a Go-first implementation of the wstun tunneling tool, designed for resource-constrained embedded systems. The Go binary is small, statically linkable, and suitable for cross-compilation to targets such as ARM and MIPS commonly found on routers and IoT devices.

## Key Benefits

- Embedded-ready: optimized for OpenWRT, DD-WRT, and similar Linux distributions
- Cross-platform: easy cross-compilation for ARM, MIPS, x86, and more
- Static binary: no runtime dependencies required on the target
- Low memory and CPU footprint suitable for constrained devices
- Fast startup and reliable behavior under systemd or init scripts

## Features

- Forward TCP tunnels over WebSocket
- Reverse TCP tunnels over WebSocket
- SSL/TLS support (WSS)
- Client authorization via allowlist
- UUID-based client identification
- Bidirectional binary data streaming with flow control

## Installation

### Prerequisites

- Go 1.21 or later
- Make (optional)

### Build from source

```bash
cd wstun
go mod download
make build
# or
go build -ldflags="-s -w" -o wstun cmd/wstun/main.go
```

### Cross compilation

Use the provided Makefile targets (e.g. `make linux-arm`, `make linux-mipsle`, `make openwrt-mips`).

## Usage

### Forward tunnel

Server:
```bash
./wstun -s 8080
# with SSL
./wstun -s 8443 --ssl true --key server.key --cert server.crt
```

Client:
```bash
./wstun -t 33:remotehost:33 ws://serverhost:8080
```

### Reverse tunnel

Server:
```bash
./wstun -r -s 8080
# with allowlist
./wstun -r -s 8080 -a allowlist.json
```

Client:
```bash
./wstun -r 2222:localhost:22 -u my-device-uuid ws://serverhost:8080
```

### Common options

```bash
--log <file>     # Write logs to file
--debug          # Enable debug logging
-h, --help       # Show help message
```

## Deployment

Copy the built binary to the target device and install as a service (systemd or init script) depending on the platform.

Example systemd service:

```ini
[Unit]
Description=WSTUN WebSocket Tunnel
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/wstun -r 2222:localhost:22 -u device-001 ws://tunnel-server:8080
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

## Configuration

Allowlist file format (JSON):

```json
[
  { "client": "device-001", "port": "2222" },
  { "client": "device-002", "port": "3333" }
]
```

## Project Layout

```
cmd/
  wstun/              # CLI entry point
pkg/
  tunnel/             # Core tunneling logic
  logger/             # Logging utilities
Makefile              # Build helpers
go.mod                # Go dependencies
```

## Testing

```bash
# Run unit tests (if any)
make test

# Example manual test
# Terminal 1: start server
./wstun -s 8080

# Terminal 2: start client
./wstun -t 8888:example.com:80 ws://localhost:8080

# Terminal 3: test
curl http://localhost:8888
```

## Troubleshooting

- Use `--debug` for verbose logs.
- Verify network connectivity and firewall rules.
- For small devices, consider using UPX to compress the binary.

## License

Apache License 2.0

---

This file documents the Go implementation only. The original Node.js implementation has been moved to the `node_version/` folder.
