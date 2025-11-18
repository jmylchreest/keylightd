# Keylightd Tray

A desktop tray application for controlling Key Lights via keylightd. Built with [Wails](https://wails.io/).

## Usage

```bash
keylightd-tray [flags]
```

### Flags

- `-css <path>`: Path to custom CSS file (default: `$XDG_CONFIG_HOME/keylightd/keylightd-tray/custom.css`)

### Examples

```bash
# Run with default config location
keylightd-tray

# Run with custom CSS file
keylightd-tray -css /path/to/my/theme.css
```

## Building

### Prerequisites

- Go 1.21+
- Node.js 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- System dependencies (Arch/CachyOS): `sudo pacman -S webkit2gtk-4.1 gtk3`

### Development

```bash
make dev
```

### Production Build

```bash
make build
```

The binary will be in `build/bin/`.

## Features

- Control individual lights and groups
- Brightness and color temperature sliders
- Real-time status updates
- Settings persistence
- Custom CSS theming

## Configuration

Settings are stored in the browser's localStorage and include:

- **Connection Type**: Unix Socket or HTTP API
- **Socket Path**: Path to keylightd socket
- **API URL**: HTTP API endpoint (when using HTTP mode)
- **API Key**: Authentication key for HTTP API
- **Refresh Interval**: How often to poll for updates (ms)
- **Visibility**: Show/hide specific lights and groups

## Custom CSS Theming

The application supports custom CSS overrides for theming.

### Default Location

```
~/.config/keylightd/keylightd-tray/custom.css
```

Or use `$XDG_CONFIG_HOME/keylightd/keylightd-tray/custom.css` if `XDG_CONFIG_HOME` is set.

You can override this with the `-css` flag.

### How It Works

1. The app loads `style.css` (default styles) first
2. Then loads `custom.css` from the config directory (if it exists)
3. The app watches for changes to `custom.css` and reloads automatically (hot reload)

### Available CSS Variables

Override these in `custom.css` to change the color scheme:

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
}
```

### Example Themes

The `custom.css` file includes commented examples for:

- **Nord** - Arctic, north-bluish color palette
- **Gruvbox** - Retro groove colors
- **Dracula** - Dark theme with purple accents

Uncomment a theme block to use it.

### Creating Your Own Theme

1. Copy one of the example themes in `custom.css`
2. Modify the color values to your preference
3. Save the file - the app will reload the styles automatically

### Tips

- Use a color picker tool to find complementary colors
- Test with both lights on and off states
- The `--accent` color is used for sliders and interactive elements
- The `--success` color is used for the "on" state of power buttons

## Architecture

- **Backend** (`main.go`, `app.go`): Go application using Wails, connects to keylightd
- **Frontend** (`frontend/`): Vanilla JavaScript with CSS, no framework dependencies
- **Styling**: CSS variables for easy theming, Catppuccin-inspired default theme

## Troubleshooting

### Build fails with webkit2gtk-4.0 not found

On Arch/CachyOS, you have webkit2gtk-4.1. Use the Makefile which passes the correct build tag:

```bash
make dev   # instead of wails dev
make build # instead of wails build
```

### "Loading..." stuck in status badge

- Ensure keylightd is running
- Check the socket exists: `ls -la /run/user/$(id -u)/keylightd.sock`
- Open browser dev tools (F12) to see error messages

### Custom CSS not loading

- Ensure `custom.css` exists at `~/.config/keylightd/keylightd-tray/custom.css`
- Or specify a custom path with `-css /path/to/custom.css`
- The file must be valid CSS (no syntax errors)
- Check the app logs for any error messages
