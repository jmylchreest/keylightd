# keylightd

[![Build Status](https://github.com/jmylchreest/keylightd/actions/workflows/release.yml/badge.svg)](https://github.com/jmylchreest/keylightd/actions)
[![Codecov](https://codecov.io/gh/jmylchreest/keylightd/branch/main/graph/badge.svg)](https://codecov.io/gh/jmylchreest/keylightd)

**keylightd** is a daemon and CLI tool for managing Elgato Keylights on your local network. It discovers lights via mDNS, allows grouping, and provides a robust CLI for automation and scripting.

---

## Features
- Automatic discovery of Elgato Keylights via mDNS
- Local daemon (`keylightd`) exposes a Unix socket for CLI/API access
- CLI tool (`keylightctl`) for managing lights and groups
- Grouping of lights for batch control
- Configurable discovery interval and logging
- HTTP REST API and Unix socket/CLI for local and remote control

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

To create a snapshot:

```bash
goreleaser build --clean --snapshot
```

To create a full release (for maintainers):

```bash
goreleaser release --clean
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

### Running as a systemd Service

You can run `keylightd` as a systemd service for automatic startup and management:

1. **Copy the unit file:**
   ```sh
   sudo cp contrib/systemd/keylightd.service /etc/systemd/system/
   ```

2. **Create the user and directories:**
   ```sh
   sudo useradd --system --no-create-home --shell /usr/sbin/nologin keylightd
   sudo mkdir -p /var/lib/keylightd
   sudo chown keylightd:keylightd /var/lib/keylightd
   sudo mkdir -p /etc/keylight
   sudo chown keylightd:keylightd /etc/keylight
   ```

3. **Reload systemd and enable the service:**
   ```sh
   sudo systemctl daemon-reload
   sudo systemctl enable --now keylightd
   ```

4. **Check status:**
   ```sh
   sudo systemctl status keylightd
   ```

The default config path is `/etc/keylight/keylightd.yaml`. You can override this with the `KEYLIGHT_CONFIG` environment variable in the unit file or your environment.

> **Note:**
> Environment variables for configuration must use the `KEYLIGHT_` prefix (e.g., `KEYLIGHT_CONFIG` for the config file path), due to the use of `SetEnvPrefix("KEYLIGHT")` in the code. Using `KEYLIGHTD_CONFIG` (as previously shown) will **not** work unless the code is changed to use a different prefix. Always use `KEYLIGHT_` for environment variable overrides.

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