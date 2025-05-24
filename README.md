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

Download the latest [release binaries](https://github.com/jmylchreest/keylightd/releases) or [snapshot builds](https://github.com/jmylchreest/keylightd/releases) (look for pre-releases) and run:

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

## Installation Methods

### Homebrew (macOS/Linux)
Install via Homebrew using our official tap:

```bash
# Add the tap
brew tap jmylchreest/keylightd

# Install keylightd
brew install keylightd
```

Or install directly:
```bash
brew install jmylchreest/keylightd/keylightd
```

This installs both `keylightd` and `keylightctl` binaries.

#### Running as a Service

After installation, you can run keylightd as a background service:

**On Linux with systemd:**
```bash
# Enable and start the user service
systemctl --user enable keylightd
systemctl --user start keylightd

# Check status
systemctl --user status keylightd
```

**On macOS:**
```bash
# Run in background manually
keylightd &

# Or create a launchd service (advanced users)
# See: https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html
```

**Manual startup (all platforms):**
```bash
keylightd
```

### Flatpak
Flatpak packages are automatically built for each release and are available from the [releases page](https://github.com/jmylchreest/keylightd/releases). Packages are built for both amd64 and arm64 architectures.

The Flatpak build process automatically:
- Generates the metainfo.xml file with release information pulled from GitHub releases
- Excludes pre-release versions from the metadata
- Creates proper desktop integration with autostart capabilities

Download the appropriate `.flatpak` file for your architecture and install with:
```bash
flatpak install keylightd-VERSION-ARCH.flatpak
```

See `contrib/flatpak/README.md` for detailed Flatpak documentation.

### Systemd Service
A systemd service file is available in `contrib/systemd/keylightd.service` for running the daemon as a system service.

### GNOME Extension
An experimental GNOME extension for controlling lights from your desktop is available in the `contrib/gnome-extension` directory.

## Documentation
For detailed documentation, see the [docs](./docs) directory.

The `docs/mkdocs-build.sh` and `docs/mkdocs-serve.sh` scripts are provided for local documentation development and preview. They allow you to build and serve the documentation locally using Docker or Podman, but are not used in the CI/CD process.

## Development Versions

Snapshot builds are automatically generated on each commit to the main branch. These builds are available as pre-releases in the [releases section](https://github.com/jmylchreest/keylightd/releases) with version tags like `0.0.0-SNAPSHOT-{hash}`. The version is determined by GoReleaser's snapshot mode. These builds contain the latest features and fixes but may not be as stable as formal releases.

## Contributing
PRs and issues are welcome! Please ensure all tests pass and code is formatted.

## License
MIT License - see the [LICENSE](LICENSE) file for details.
