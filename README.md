# keylightd

[![Build Status](https://github.com/jmylchreest/keylightd/actions/workflows/goreleaser.yml/badge.svg)](https://github.com/jmylchreest/keylightd/actions)
[![Test Status](https://github.com/jmylchreest/keylightd/actions/workflows/test.yml/badge.svg)](https://github.com/jmylchreest/keylightd/actions)
[![Codecov](https://codecov.io/gh/jmylchreest/keylightd/branch/main/graph/badge.svg)](https://codecov.io/gh/jmylchreest/keylightd)

**keylightd** is a daemon and CLI tool for managing Elgato Keylights on your local network. It discovers lights via mDNS, allows grouping, and provides a robust CLI for automation and scripting.

---

## Features
- Automatic discovery of Elgato Keylights via mDNS
- Local daemon (`keylightd`) exposes a Unix socket for CLI/API access
- CLI tool (`keylightctl`) for managing lights and groups
- Grouping of lights for batch control
- Configurable discovery interval and logging
- No REST API (all communication is via Unix socket and CLI)

---

## Architecture
- **keylightd**: Runs as a background daemon, discovers and manages lights, persists configuration, and exposes a Unix socket for local control.
- **keylightctl**: CLI tool that communicates with the daemon via the Unix socket. All user interaction and scripting is done through this CLI.

```
+-----------+      Unix Socket      +-----------+      mDNS/HTTP      +-------------------+
| keylightctl| <------------------> | keylightd | <---------------> | Elgato Key Lights |
+-----------+                      +-----------+                    +-------------------+
```

---

## Installation & Building

### Prerequisites
- Go 1.24 or later
- [goreleaser](https://goreleaser.com/)

### Building with GoReleaser

To build and create release artifacts:

```bash
goreleaser build --clean --snapshot
```

To create a full release (for maintainers):

```bash
goreleaser release --clean --snapshot
```

Binaries for `linux/amd64` and `linux/arm64` will be produced.

---

## Usage

### Running the Daemon

```bash
./keylightd
```

- The daemon will discover lights and listen on a Unix socket (default: `$XDG_RUNTIME_DIR/keylightd.sock` or `/run/user/<uid>/keylightd.sock`).
- Configuration is stored in `$XDG_CONFIG_HOME/keylight/keylightd.yaml` or `~/.config/keylight/keylightd.yaml`.

### Using the CLI

```bash
./keylightctl light list
./keylightctl group add "My Group"
./keylightctl group set <group-id> on true
```

- The CLI will connect to the daemon via the Unix socket.
- CLI config is stored in `$XDG_CONFIG_HOME/keylight/keylightctl.yaml` or `~/.config/keylight/keylightctl.yaml`.

---

## Configuration
- Config files are YAML and are auto-generated on first run.
- You can override config locations with environment variables (`XDG_CONFIG_HOME`, etc).
- See the `internal/config` package for details.

---

## Testing

Run all tests:

```bash
go test ./...
```

Generate a coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Contributing
- PRs and issues are welcome!
- Please ensure all tests pass and code is formatted (`gofmt`, `goimports`).
- See [CONTRIBUTING.md](CONTRIBUTING.md) if present.

---

## License

This project is licensed under the MIT License - see the LICENSE file for details. 