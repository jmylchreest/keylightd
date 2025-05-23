# Flatpak Package for KeylightD

This directory contains the necessary files to build a Flatpak package for KeylightD and KeylightCTL. The package uses the "light-enabled.svg" icon from the GNOME extension for desktop integration.

## CI Build Process

The Flatpak is built as part of the release workflow in GitHub Actions. The process:

1. The release workflow is triggered when a tag is pushed or manually triggered
2. After GoReleaser builds and uploads the binaries, the flatpak job runs
3. The flatpak job builds packages for both amd64 and arm64 architectures
4. For each architecture, it fetches the corresponding pre-compiled binaries 
5. Creates a modified version of this manifest to use the pre-built binaries
6. Builds and publishes both flatpak packages to the same release

## Features

- Packages both keylightd (daemon) and keylightctl (control utility)
- Sets up proper desktop integration
- Provides autostart capability for the daemon
- Includes user systemd service for management

# Building Locally

To build the Flatpak package locally:

```bash
# Install required tools
sudo dnf install flatpak-builder # Fedora
sudo apt install flatpak-builder # Ubuntu/Debian

# Add Flathub remote if not already added
flatpak remote-add --user --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo
```

### Method 1: Using Pre-built Binaries (Recommended)

This is the same approach used by the CI workflow:

1. Download and extract binaries from a release (or build them yourself):
   ```bash
   # Create a directory for your binaries
   mkdir -p binaries
   
   # Option A: Download from a release
   wget https://github.com/jmylchreest/keylightd/releases/download/v1.0.0/keylightd_1.0.0_linux_amd64.tar.gz
   tar -xzf keylightd_1.0.0_linux_amd64.tar.gz
   cp keylightd binaries/
   cp keylightctl binaries/
   
   # Option B: Copy your locally built binaries
   # cp /path/to/keylightd binaries/
   # cp /path/to/keylightctl binaries/
   
   chmod +x binaries/keylightd
   chmod +x binaries/keylightctl
   ```

2. Build the flatpak:
   ```bash
   flatpak-builder --user --force-clean build-dir io.github.jmylchreest.keylightd.yml
   
   # Install the built Flatpak
   flatpak-builder --user --install build-dir io.github.jmylchreest.keylightd.yml
   
   # OR create a Flatpak bundle file
   flatpak-builder --repo=repo --force-clean build-dir io.github.jmylchreest.keylightd.yml
   flatpak build-bundle repo keylightd.flatpak io.github.jmylchreest.keylightd
   ```

## Using the Flatpak

### Starting the Daemon

After installation, the daemon can be started in several ways:

1. **Automatic start on login** (default): The desktop autostart entry will start keylightd automatically.

2. **Manual start**:
   ```bash
   flatpak run io.github.jmylchreest.keylightd
   ```

3. **Using Systemd** (if systemd portal is available):
   ```bash
   # Start the service
   systemctl --user start io.github.jmylchreest.keylightd.service
   
   # Enable automatic start on login
   systemctl --user enable io.github.jmylchreest.keylightd.service
   ```

### Using KeylightCTL

The CLI control utility can be run with:

```bash
flatpak run --command=keylightctl io.github.jmylchreest.keylightd [commands]
```

Example:
```bash
flatpak run --command=keylightctl io.github.jmylchreest.keylightd light list
```

## Files

- `io.github.jmylchreest.keylightd.yml` - Flatpak manifest (modified by CI to use pre-built binaries)
- `io.github.jmylchreest.keylightd.service` - Systemd user service
- `io.github.jmylchreest.keylightd-autostart.desktop` - Desktop autostart file
- `io.github.jmylchreest.keylightd.desktop` - Desktop application entry
- `io.github.jmylchreest.keylightd.metainfo.xml` - AppStream metadata
- Uses the icon from `contrib/gnome-extension/keylightd-control@jmylchreest.github.io/icons/hicolor/scalable/actions/light-enabled.svg`

## Architecture Support

The CI builds flatpaks for both amd64 (x86_64) and arm64 architectures. When downloading:

- For regular Intel/AMD machines: use the `*-amd64.flatpak` file
- For Apple Silicon or other ARM-based systems: use the `*-arm64.flatpak` file

Each architecture-specific build contains binaries optimized for that platform.

## Notes

- Flatpak configuration is stored in `~/.var/app/io.github.jmylchreest.keylightd/config/keylight/`
- Socket path will be in the XDG runtime directory for the Flatpak