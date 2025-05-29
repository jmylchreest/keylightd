# Getting Started with keylightd

This guide will help you get started with keylightd, a daemon service for controlling Elgato Key Light devices and potentially other HTTP-based lights with similar interfaces. If you have a similar device that's not explicitly supported, please open a ticket to request support.

## Installation

### Prerequisites

- Linux or macOS operating system (basic testing has been performed on Windows and it should work.)
- Go 1.24 or higher (if building from source)
- Network connectivity to your Elgato Key Light devices

### Option 1: Install via Homebrew (Recommended)

The easiest way to install keylightd is through our Homebrew tap:

```bash
brew tap jmylchreest/keylightd
brew install keylightd
```

This will install both the `keylightd` daemon and `keylightctl` CLI tool. The installation automatically sets up a launchd service on macOS and a systemd service on Linux.

To start the service:
```bash
brew services start jmylchreest/keylightd/keylightd
```

**Note:** You can also run keylightd manually by simply executing `keylightd` in your terminal if you prefer not to use the system service.

### Option 2: Installing from Binary Releases

1. Download the latest release from the [GitHub releases page](https://github.com/jmylchreest/keylightd/releases)
2. Extract the archive:
   ```
   tar -xzf keylightd-v*.tar.gz
   ```
3. Move the binary to a directory in your PATH:
   ```
   sudo mv keylightd /usr/local/bin/
   chmod +x /usr/local/bin/keylightd
   ```

### Option 3: Building from Source

1. Clone the repository:
   ```
   git clone https://github.com/jmylchreest/keylightd.git
   cd keylightd
   ```

2. Build the binary:
   ```
   go build -o keylightd ./cmd/keylightd
   ```

3. Install the binary:
   ```
   sudo mv keylightd /usr/local/bin/
   chmod +x /usr/local/bin/keylightd
   ```

**Note:** After installation, you can run keylightd manually by executing `keylightd` in your terminal.

### Option 4: Arch Linux (AUR)

For Arch Linux users, keylightd is available in the AUR:

```bash
# Using yay
yay -S keylightd-bin

# Or using paru
paru -S keylightd-bin
```

After installation:

1. Add your user to the `keylightd` group for socket access:
   ```bash
   sudo usermod -a -G keylightd $USER
   ```

2. Enable and start the systemd service:
   ```bash
   sudo systemctl enable keylightd
   sudo systemctl start keylightd
   ```

3. Log out and back in for group changes to take effect

**Socket Permissions:** The systemd service creates a Unix socket at `/run/keylightd/keylightd.sock` that is accessible by users in the `keylightd` group. This allows `keylightctl` to communicate with the daemon running as a system service.

## Configuration

keylightd uses a configuration file located at `~/.config/keylightd/config.yaml`. The configuration file is created when settings are first saved, but you can also create it manually.

### Complete Configuration Example

```yaml
# Application state (automatically managed)
state:
  api_keys:
    - key: "your-generated-api-key-here"
      name: "my-api-key"
      created_at: "2024-01-01T00:00:00Z"
      expires_at: "2024-02-01T00:00:00Z"
      last_used_at: "2024-01-15T12:00:00Z"
  groups:
    group-123451:
      id: "group-123451"
      name: "office-lights"
      lights:
        - "Elgato Key Light ABC1._elg._tcp.local."
        - "Elgato Key Light XYZ2._elg._tcp.local."

# Configuration settings
config:
  # Server configuration
  server:
    # Unix socket path for local communication
    unix_socket: "/run/user/1000/keylightd.sock"

  # HTTP API configuration
  api:
    # Address and port for the HTTP API (default: :9123)
    listen_address: ":9123"

  # Device discovery settings
  discovery:
    # How often to scan for new devices (seconds, default: 30)
    interval: 30
    # How often to check for offline devices (seconds, default: 180)
    cleanup_interval: 180
    # How long before marking a device as offline (seconds, default: 180)
    cleanup_timeout: 180

  # Logging configuration
  logging:
    # Log level: debug, info, warn, error (default: info)
    level: info
    # Log format: text, json (default: text)
    format: text
```

## Creating Your First API Key

Before using the HTTP API, you need to create an API key:

```
keylightctl api-key add my-first-key
```

This will generate a new API key that you can use to authenticate API requests.

## Basic Usage

keylightd comes with a command-line interface for controlling lights:

```
# List all discovered lights
keylightctl light list

# Turn on a specific light
keylightctl light set "Elgato Key Light ABC1._elg._tcp.local." on true

# Change brightness
keylightctl light set "Elgato Key Light ABC1._elg._tcp.local." brightness 50

# Change color temperature
keylightctl light set "Elgato Key Light ABC1._elg._tcp.local." temperature 4000
```

## GNOME Extension

There is a GNOME extension available that allows you to control your lights directly from the GNOME desktop. You can download the extension from the [GitHub releases page](https://github.com/jmylchreest/keylightd/releases) and install it using:

```bash
gnome-extensions install keylightd-control@jmylchreest.github.io.zip
```

After installation, enable the extension through GNOME Extensions or from the command line:

```bash
gnome-extensions enable keylightd-control@jmylchreest.github.io
```

## Next Steps

Now that you have keylightd up and running, you can:

- [Explore light control options](lights/cli.md)
- [Create and manage light groups](groups/cli.md)
- [Review the complete API reference](api/index.md)

## Troubleshooting

### Lights Not Being Discovered

- Ensure your Key Lights are on the same network as your computer
- Check that mDNS/Bonjour is not blocked by your firewall
- Try running with debug logging: `keylightd --log-level debug`

### Connection Issues

- Check that the Key Lights are powered on and connected to your network
- Verify network connectivity by pinging the light's IP address
- Ensure no firewall is blocking the connection

### Socket Permission Issues

If you get a "permission denied" error when using `keylightctl` with a systemd service:

```
Error: failed to connect to socket: dial unix /run/keylightd/keylightd.sock: connect: permission denied
```

This means your user doesn't have permission to access the daemon's socket. To fix this:

1. Add your user to the `keylightd` group:
   ```bash
   sudo usermod -a -G keylightd $USER
   ```

2. Log out and back in for the group changes to take effect

3. Verify you're in the group:
   ```bash
   groups | grep keylightd
   ```

4. Check socket permissions:
   ```bash
   ls -la /run/keylightd/keylightd.sock
   ```

   The socket should be owned by `keylightd:keylightd` with group write permissions.


For more help, check the [GitHub issues page](https://github.com/jmylchreest/keylightd/issues) or submit a new issue.
