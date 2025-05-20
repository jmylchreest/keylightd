# Flatpak Package for KeylightD

This directory contains the necessary files to build a Flatpak package for KeylightD and KeylightCTL. The package uses the "light-enabled.svg" icon from the GNOME extension for desktop integration.

## Features

- Packages both keylightd (daemon) and keylightctl (control utility)
- Sets up proper desktop integration
- Provides autostart capability for the daemon
- Includes user systemd service for management

## Building Locally

To build the Flatpak package locally:

```bash
# Install required tools
sudo dnf install flatpak-builder # Fedora
sudo apt install flatpak-builder # Ubuntu/Debian

# Add Flathub remote if not already added
flatpak remote-add --user --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo

# Build the Flatpak
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

- `io.github.jmylchreest.keylightd.yml` - Flatpak manifest
- `io.github.jmylchreest.keylightd.service` - Systemd user service
- `io.github.jmylchreest.keylightd-autostart.desktop` - Desktop autostart file
- `io.github.jmylchreest.keylightd.desktop` - Desktop application entry
- `io.github.jmylchreest.keylightd.metainfo.xml` - AppStream metadata
- Uses the icon from `contrib/gnome-extension/keylightd-control@jmylchreest.github.io/icons/hicolor/scalable/actions/light-enabled.svg`

## Notes

- Flatpak configuration is stored in `~/.var/app/io.github.jmylchreest.keylightd/config/keylight/`
- Socket path will be in the XDG runtime directory for the Flatpak