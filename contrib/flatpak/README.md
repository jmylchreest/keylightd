# Flatpak Package for KeylightD

This directory contains the necessary files to build a Flatpak package for KeylightD and KeylightCTL. The package uses the "light-enabled.svg" icon from the GNOME extension for desktop integration.

**IMPORTANT:** Both the Flatpak manifest file and the AppStream metainfo.xml file are entirely **generated** by the GitHub Actions workflow and are not stored in the repository. The workflow dynamically creates these files with the correct version information, dependencies, build instructions, and release history at build time.

## CI Build Process

The Flatpak is built as part of the release workflow in GitHub Actions. The process:

1. The release workflow is triggered when a tag is pushed or manually triggered
2. After GoReleaser builds and uploads the binaries, the flatpak job runs
3. The flatpak job builds packages for both amd64 and arm64 architectures
4. For each architecture:
   - It creates a full source archive of the repository
   - Vendors Go modules for reproducible builds
   - Fetches all GitHub releases (excluding pre-releases) via the GitHub API
   - Dynamically generates the metainfo.xml file with complete release history
   - Dynamically generates a complete Flatpak manifest with correct version information
   - Specifies the 23.08 SDK version to ensure compatibility
   - Builds the Flatpak directly from source
5. Builds and publishes both flatpak packages to the same release

## Features

- Packages both keylightd (daemon) and keylightctl (control utility)
- Sets up proper desktop integration
- Provides autostart capability for the daemon
- Includes user systemd service for management
- Automatically generates AppStream metainfo.xml with complete release history from GitHub
- Excludes pre-release versions from the metadata for stable software center listings

# Building Locally

To build the Flatpak package locally:

```bash
# Install required tools
sudo dnf install flatpak-builder # Fedora
sudo apt install flatpak-builder # Ubuntu/Debian

# Add Flathub remote if not already added
flatpak remote-add --user --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo
```

### Building Locally

To build the Flatpak package locally, you'll need to:

1. Create your own Flatpak manifest (since it's not stored in the repo):

   ```bash
   # Create a manifest based on the CI-generated one
   cat > io.github.jmylchreest.keylightd.yml << 'EOF'
   app-id: io.github.jmylchreest.keylightd
   runtime: org.freedesktop.Platform
   runtime-version: "23.08"
   sdk: org.freedesktop.Sdk
   sdk-extensions:
     - org.freedesktop.Sdk.Extension.golang
   command: keylightctl
   finish-args:
     - --share=network
     - --socket=x11
     - --socket=wayland
     - --own-name=io.github.jmylchreest.keylightd
     - --filesystem=home
     - --talk-name=org.freedesktop.systemd1
   
   modules:
     - name: keylightd
       buildsystem: simple
       build-options:
         env:
           - CGO_ENABLED=0
         append-path: /usr/lib/sdk/golang/bin
       build-commands:
         # Build keylightd and keylightctl
         - go build -o keylightd ./cmd/keylightd
         - go build -o keylightctl ./cmd/keylightctl

         # Install binaries and supporting files
         - install -Dm755 keylightd /app/bin/keylightd
         - install -Dm755 keylightctl /app/bin/keylightctl
         - mkdir -p /app/share/keylightd
         - install -Dm644 contrib/flatpak/io.github.jmylchreest.keylightd.service /app/share/systemd/user/io.github.jmylchreest.keylightd.service
         - install -Dm644 contrib/flatpak/io.github.jmylchreest.keylightd-autostart.desktop /app/share/applications/io.github.jmylchreest.keylightd-autostart.desktop
         - install -Dm644 contrib/flatpak/io.github.jmylchreest.keylightd.desktop /app/share/applications/io.github.jmylchreest.keylightd.desktop
         - install -Dm644 contrib/gnome-extension/keylightd-control@jmylchreest.github.io/icons/hicolor/scalable/actions/light-enabled.svg /app/share/icons/hicolor/scalable/apps/io.github.jmylchreest.keylightd.svg
         - install -Dm644 contrib/flatpak/io.github.jmylchreest.keylightd.metainfo.xml /app/share/metainfo/io.github.jmylchreest.keylightd.metainfo.xml
       sources:
         - type: git
           url: https://github.com/jmylchreest/keylightd.git
           tag: main
   EOF
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

- `io.github.jmylchreest.keylightd.service` - Systemd user service
- `io.github.jmylchreest.keylightd-autostart.desktop` - Desktop autostart file
- `io.github.jmylchreest.keylightd.desktop` - Desktop application entry
- `io.github.jmylchreest.keylightd.metainfo.xml` - AppStream metadata (automatically generated from GitHub releases)
- Uses the icon from `contrib/gnome-extension/keylightd-control@jmylchreest.github.io/icons/hicolor/scalable/actions/light-enabled.svg`

Note that both the Flatpak manifest (`io.github.jmylchreest.keylightd.yml`) and the metainfo.xml file are not stored in the repository. They are dynamically generated by the GitHub Actions workflow with:
- Proper version information from the release tags
- Complete release history fetched from GitHub releases API (excluding pre-releases)
- Correct SDK/runtime versions (currently fixed at 23.08)
- Custom build instructions for snapshot vs. release builds
- All necessary dependencies and build configurations
- Properly formatted AppStream metadata for software centers

## Architecture Support

The CI builds flatpaks for both amd64 (x86_64) and arm64 architectures. When downloading:

- For regular Intel/AMD machines: use the `*-amd64.flatpak` file
- For Apple Silicon or other ARM-based systems: use the `*-arm64.flatpak` file

Each architecture-specific build contains binaries optimized for that platform.

## Notes

- Flatpak configuration is stored in `~/.var/app/io.github.jmylchreest.keylightd/config/keylight/`
- Socket path will be in the XDG runtime directory for the Flatpak