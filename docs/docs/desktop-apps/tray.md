---
sidebar_position: 1
---

# Tray Application

A desktop system tray application for controlling Key Lights via keylightd.

## Features

- System tray icon with dynamic status (on/off/unknown)
- Control individual lights and groups
- Brightness and color temperature sliders
- Real-time status updates
- Custom CSS theming with hot reload
- Connect via Unix socket or HTTP API

## Installation

### Arch Linux (AUR)

```bash
yay -S keylightd-tray-bin
```

### From Source

Prerequisites:
- Go 1.21+
- Node.js 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- System dependencies: `sudo pacman -S webkit2gtk-4.1 gtk3`

Build:
```bash
cd contrib/keylightd-tray
make build
```

The binary will be in `build/bin/keylightd-tray`.

## Usage

```bash
keylightd-tray [flags]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-css <path>` | Path to custom CSS file | `$XDG_CONFIG_HOME/keylightd/keylightd-tray/custom.css` |

### Examples

```bash
# Run with default settings
keylightd-tray

# Run with custom CSS theme
keylightd-tray -css ~/themes/dark.css
```

## System Tray

The application runs in the system tray with a dynamic icon:

- **Yellow bulb** - At least one light is on
- **Black bulb** - All lights are off
- **Gray bulb** - Unknown/disconnected

Right-click the tray icon for options:
- **Show/Hide** - Toggle the main window
- **Quit** - Exit the application

## Connection Settings

The app can connect to keylightd via:

### Unix Socket (Default)
- Socket path: `/run/user/<uid>/keylightd.sock`
- No authentication required
- Best for local use

### HTTP API
- API URL: `http://localhost:9123` (default)
- Requires API key authentication
- Use for remote connections

Configure in Settings > Connection.

### Paired HTTPS

On the daemon host, open **Preferences → Connection → Paired HTTPS → Enable and use HTTPS**. The daemon generates an identity and switches its API port to HTTPS. Existing plain HTTP clients will disconnect. The tray saves the new profile after an authenticated light read and a secure WebSocket handshake succeed.

To connect another tray, choose a reachable URL under **Network address**, then select **Create invitation**. Transfer the invitation privately and paste it into **Use an invitation** on the other computer. The invitation expires after five minutes and can be redeemed once. Each client receives its own API key and verifies the daemon's pinned public key.

**Disable HTTPS** restores the previous listener mode and keeps the identity. **Forget identity…** is available while HTTPS is disabled and requires clients to pair again after reactivation.

The CLI can issue and redeem the same invitations. On the daemon host, run:

```bash
keylightctl https enable
keylightctl https status
keylightctl https invite
```

On the client, pass the invitation on standard input to `keylightctl https pair --profile ./studio.json`. Select that profile with `keylightctl --profile ./studio.json light list`. The profile contains the client's API key and is saved with owner-only permissions.

## Custom Theming

### CSS Location

Default: `~/.config/keylightd/keylightd-tray/custom.css`

Override with `-css` flag.

### Available Variables

```css
:root {
    /* Background colors */
    --bg-primary: #1e1e2e;
    --bg-secondary: #313244;
    --bg-tertiary: #45475a;
    
    /* Text colors */
    --text-primary: #cdd6f4;
    --text-secondary: #a6adc8;
    --text-muted: #6c7086;
    
    /* Accent and state colors */
    --accent: #89b4fa;
    --success: #a6e3a1;
    --warning: #f9e2af;
    --error: #f38ba8;
    
    /* Surface colors */
    --surface: #181825;
    --overlay: #11111b;
    
    /* Component-specific colors */
    --slider-track: var(--bg-tertiary);
    --input-bg: var(--surface);
    --input-border: var(--bg-tertiary);
    --list-item-bg: var(--surface);
}
```

### Hot Reload

The app watches for changes to the CSS file and automatically reloads styles - no restart needed.

### Example: Nord Theme

```css
:root {
    --bg-primary: #2e3440;
    --bg-secondary: #3b4252;
    --bg-tertiary: #434c5e;
    --text-primary: #eceff4;
    --text-secondary: #d8dee9;
    --text-muted: #4c566a;
    --accent: #88c0d0;
    --success: #a3be8c;
    --warning: #ebcb8b;
    --error: #bf616a;
    --surface: #2e3440;
    --overlay: #242933;
}
```

## Settings

**Connection** shows the active method. **This computer** uses the Unix socket. **Manual address** accepts an API address and key. Both forms test the proposed connection before saving it. The tray stores the active connection in its user-only configuration file.

**Lights** and **Groups** control which entries appear in the tray. **Display** sets the refresh interval in seconds (1–10, default 2.5). These display preferences are stored locally for each tray. Group membership is saved on the daemon and shared by its clients.

## Troubleshooting

### App won't start

Ensure dependencies are installed:
```bash
# Arch Linux
sudo pacman -S webkit2gtk-4.1 gtk3
```

### "Loading..." stuck

1. Check keylightd is running: `systemctl --user status keylightd`
2. Verify socket exists: `ls -la /run/user/$(id -u)/keylightd.sock`
3. Test connection: `keylightctl light list`

### Custom CSS not loading

1. Verify file exists at the expected location
2. Check for CSS syntax errors
3. Try running with explicit path: `keylightd-tray -css /path/to/custom.css`

### Tray icon not visible

Some desktop environments need a system tray extension:
- GNOME: Install "AppIndicator and KStatusNotifierItem Support"
- Other DEs: Usually have built-in support

## Architecture

- **Backend**: Go with Wails framework
- **Frontend**: Vanilla JavaScript/CSS (no framework)
- **Tray**: fyne.io/systray
- **IPC**: Wails runtime bindings

The app embeds the frontend assets and uses WebKit2GTK for rendering.
