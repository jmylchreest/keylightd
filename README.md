# keylightd

[![Build Status](https://github.com/jmylchreest/keylightd/actions/workflows/release.yml/badge.svg)](https://github.com/jmylchreest/keylightd/actions)
[![Codecov](https://codecov.io/gh/jmylchreest/keylightd/branch/main/graph/badge.svg)](https://codecov.io/gh/jmylchreest/keylightd)

**keylightd** is a daemon and CLI tool for managing Elgato Key Lights on your local network. While designed primarily for Elgato Key Lights, it may also support other HTTP-based lights with similar interfaces (if you have a compatible device not explicitly supported, please open a ticket).

## Features
- Automatic discovery of lights via mDNS
- Grouping of lights for batch control
- HTTP REST API for remote control
- Unix socket and CLI interface for local control
- Configurable discovery interval and logging

## Components
- **keylightd**: Daemon that discovers lights, persists configuration, and exposes APIs
- **keylightctl**: CLI tool for managing lights and groups

```
+-----------+      Unix Socket      +-----------+      mDNS/HTTP      +-------------------+
| keylightctl| <------------------> | keylightd | <---------------> | Elgato Key Lights |
+-----------+                      +-----------+                    +-------------------+
```

## Quick Start

Download the latest [release binaries](https://github.com/jmylchreest/keylightd/releases) or [snapshot builds](https://github.com/jmylchreest/keylightd/releases) (tagged with `snapshot-`) and run:

```bash
# Start the daemon
./keylightd

# List discovered lights
./keylightctl light list

# Create a light group
./keylightctl group add "Office"

# Control a light group
./keylightctl group set Office on true
```

Configuration files are automatically generated on first save in `~/.config/keylight/`.

## Systemd Service
A systemd service file is available in `contrib/systemd/keylightd.service` for running the daemon as a system service.

## GNOME Extension
An experimental GNOME extension for controlling lights from your desktop is available in the `contrib/gnome-extension` directory.

## Documentation
For detailed documentation, see the [docs](./docs) directory.

The `docs/mkdocs-build.sh` and `docs/mkdocs-serve.sh` scripts are provided for local documentation development and preview. They allow you to build and serve the documentation locally using Docker or Podman, but are not used in the CI/CD process.

## Development Versions

Snapshot builds are automatically generated on each commit to the main branch. These builds are available as pre-releases in the [releases section](https://github.com/jmylchreest/keylightd/releases) with version numbers like `0.0.0-SNAPSHOT-{commit_hash}`. They contain the latest features and fixes but may not be as stable as formal releases.

## Contributing
PRs and issues are welcome! Please ensure all tests pass and code is formatted.

## License
MIT License - see the [LICENSE](LICENSE) file for details.
