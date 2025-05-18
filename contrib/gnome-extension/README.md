# Keylightd Control GNOME Extension

This directory contains a GNOME Shell extension for controlling Elgato Keylights via the keylightd HTTP API.

## Features
- Integrates with GNOME quick settings
- Control individual lights and groups
- Configure API endpoint and authentication key via UI
- Follows GNOME extension best practices for structure and packaging

## Directory Structure

```
contrib/gnome-extension/
  Makefile
  README.md
  keylightd-control@jmylchreest.github.io/
    extension.js
    stylesheet.css
    version.js
    metadata.json
    preferences/
      prefs.js
    schemas/
      org.gnome.shell.extensions.keylightd-control.gschema.xml
    icons/
      hicolor/
        scalable/
          actions/
            keylight-symbolic.svg
```

## Development & Packaging

1. **Build schemas:**
   ```sh
   make build
   ```
2. **Package the extension:**
   ```sh
   make pack
   ```
   This uses `gnome-extensions pack` and outputs a zip in `../../dist/gnome-extension/`.

## Installation & Testing

1. **Install the packed extension:**
   ```sh
   gnome-extensions install --force dist/gnome-extension/keylightd-control@jmylchreest.github.io.shell-extension.zip
   ```
   Or copy the folder to your local extensions directory:
   ```sh
   cp -r contrib/gnome-extension/keylightd-control@jmylchreest.github.io ~/.local/share/gnome-shell/extensions/
   ```
2. **Compile schemas (if not using the zip):**
   ```sh
   glib-compile-schemas ~/.local/share/gnome-shell/extensions/keylightd-control@jmylchreest.github.io/schemas
   ```
3. **Enable the extension:**
   ```sh
   gnome-extensions enable keylightd-control@jmylchreest.github.io
   ```
4. **Reload GNOME Shell:** (Alt+F2, type 'r', press Enter)
5. **Configure:** Click the icon in the quick settings menu and set the API endpoint and key.

> **Note:** If the API key is not set, the extension will not attempt to interact with the API and will only show the configuration UI.

## Icon
- The extension icon is located at `icons/hicolor/scalable/actions/keylight-symbolic.svg` inside the extension directory, following GNOME icon theme conventions.

## Preferences
- The preferences dialog is implemented in `preferences/prefs.js` and can be accessed from the extension menu or via the GNOME Extensions app.

## Contributing
- Please follow the structure and conventions in this repository. PRs are welcome!

## Testing & Debugging

### Testing in a Nested Wayland Session (Recommended)

Running GNOME Shell in a nested Wayland session allows you to test and debug your extension without affecting your main desktop session.

1. **Start a nested GNOME Shell session:**
   ```sh
   dbus-run-session -- gnome-shell --nested --wayland
   ```
   This will open a new GNOME Shell window inside your current session.

2. **Install and enable your extension inside the nested session:**
   - Open a terminal inside the nested session (e.g., with `Ctrl+Alt+T` or from the app grid).
   - Install the extension as described above.
   - Use `gnome-extensions` commands inside the nested session to enable/disable and debug.

3. **View logs for debugging:**
   - Run `journalctl -f /usr/bin/gnome-shell` in a terminal to see GNOME Shell logs in real time.
   - Use `gnome-extensions info keylightd-control@jmylchreest.github.io` for extension status.

### Quick Reload (X11 only)
- On X11, you can reload GNOME Shell with `Alt+F2`, type `r`, and press Enter.
- On Wayland, you must log out and back in, or use a nested session as above.

### General Tips
- Use the [Looking Glass](https://wiki.gnome.org/Projects/GnomeShell/LookingGlass) debugger (`lg` in Alt+F2) for live JS console and object inspection.
- Use `console.log()` and `console.error()` in your JS code for debugging output.
- Always test with both the preferences dialog and the quick settings menu. 