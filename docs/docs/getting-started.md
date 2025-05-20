# Getting Started with KeylightD

This guide will help you get started with KeylightD, a daemon service for controlling Elgato Key Light devices and potentially other HTTP-based lights with similar interfaces. If you have a similar device that's not explicitly supported, please open a ticket to request support.

## Installation

### Prerequisites

- Linux operating system
- Go 1.24 or higher (if building from source)
- Network connectivity to your Elgato Key Light devices

### Option 1: Installing from Binary Releases

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

### Option 2: Building from Source

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

## Configuration

KeylightD uses a configuration file located at `~/.config/keylightd/config.yaml`. The file will be created automatically the first time you run KeylightD, but you can also create it manually.

### Basic Configuration Example

```yaml
server:
  unix_socket: /tmp/keylightd.sock  # Unix socket path
api:
  listen_address: 127.0.0.1:8080    # HTTP API address and port
  enable: true                       # Enable HTTP API
discovery:
  interval: 30                       # Discovery interval in seconds
  cleanup_interval: 60               # Cleanup check interval in seconds
  cleanup_timeout: 180               # Device timeout in seconds
logging:
  level: info                        # Logging level (debug, info, warn, error)
```

## Running KeylightD

### Starting Manually

Run KeylightD in your terminal:

```
keylightd
```

You should see output indicating that the daemon has started and is discovering devices.

### Setting Up as a System Service

For persistent operation, it's recommended to set up KeylightD as a system service using systemd.

1. Create a service file:
   ```
   sudo vi /etc/systemd/system/keylightd.service
   ```

2. Add the following content:
   ```
   [Unit]
   Description=KeylightD daemon for controlling Elgato Key Light devices
   After=network.target

   [Service]
   ExecStart=/usr/local/bin/keylightd
   Restart=on-failure
   User=YOUR_USERNAME  # Replace with your username

   [Install]
   WantedBy=multi-user.target
   ```

3. Enable and start the service:
   ```
   sudo systemctl enable keylightd
   sudo systemctl start keylightd
   ```

4. Check the service status:
   ```
   sudo systemctl status keylightd
   ```

## Creating Your First API Key

Before using the HTTP API, you need to create an API key:

```
keylightctl api-key add my-first-key
```

This will generate a new API key that you can use to authenticate API requests.

## Basic Usage

### Using the CLI

KeylightD comes with a command-line interface for controlling lights:

```
# List all discovered lights
keylightctl light list

# Turn on a specific light
keylightctl light set --id light-1 --on

# Change brightness
keylightctl light set --id light-1 --brightness 50

# Change color temperature
keylightctl light set --id light-1 --temperature 4000
```

### Using the API

You can also use the HTTP API directly:

```bash
# List all lights
curl -H "Authorization: Bearer YOUR_API_KEY" http://127.0.0.1:8080/api/v1/lights

# Turn on a light
curl -X POST -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true}' \
  http://127.0.0.1:8080/api/v1/lights/light-1/state
```

See the [API Reference](api/index.md) for full details on all available endpoints.

## GNOME Extension

There is an experimental GNOME extension available that allows you to control your lights directly from the GNOME desktop. You can find it in the `contrib/gnome-extension` directory of the project.

## Next Steps

Now that you have KeylightD up and running, you can:

- [Learn about authentication](authentication.md)
- [Explore light control options](lights.md)
- [Create and manage light groups](groups.md)
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

### API Key Issues

- If your API key isn't working, try creating a new one
- Check that you're including the key correctly in your requests

For more help, check the [GitHub issues page](https://github.com/jmylchreest/keylightd/issues) or submit a new issue.