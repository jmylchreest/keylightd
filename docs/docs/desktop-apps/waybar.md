---
sidebar_position: 3
---

# Waybar Integration

keylightctl supports waybar-compatible JSON output for creating custom modules.

## Basic Module

Add this to your waybar config (`~/.config/waybar/config`):

```json
{
    "custom/keylight": {
        "exec": "keylightctl light list --waybar",
        "return-type": "json",
        "interval": 30,
        "on-click": "keylightctl group set default on true",
        "on-click-right": "keylightctl group set default on false",
        "on-scroll-up": "keylightctl group set default brightness $(( $(keylightctl light get $(keylightctl light list -p | head -1 | cut -d'\"' -f2) brightness) + 5 ))",
        "on-scroll-down": "keylightctl group set default brightness $(( $(keylightctl light get $(keylightctl light list -p | head -1 | cut -d'\"' -f2) brightness) - 5 ))"
    }
}
```

## Output Format

The `--waybar` flag outputs JSON in waybar's expected format:

```json
{
    "text": "2/3",
    "tooltip": "Lights: 2 on, 1 off\nKey Light ABC1: 75% @ 4500K\nKey Light DEF2: 50% @ 5000K\nKey Light GHI3: off",
    "class": "on",
    "percentage": 62
}
```

Fields:
- `text`: Shows "on/total" count (e.g., "2/3")
- `tooltip`: Detailed status of each light
- `class`: Either "on" or "off" for styling
- `percentage`: Average brightness of lights that are on

## Styling

Add to your waybar style (`~/.config/waybar/style.css`):

```css
#custom-keylight {
    padding: 0 10px;
}

#custom-keylight.on {
    color: #a6e3a1;
}

#custom-keylight.off {
    color: #6c7086;
}
```

## Advanced: Toggle Script

For more control, create a toggle script at `~/.local/bin/keylight-toggle`:

```bash
#!/bin/bash

# Get current state of first light
STATE=$(keylightctl light list -j | jq -r '.[0].on')

if [ "$STATE" = "true" ]; then
    keylightctl group set default on false
else
    keylightctl group set default on true
fi
```

Then use it in your waybar config:

```json
{
    "custom/keylight": {
        "exec": "keylightctl light list --waybar",
        "return-type": "json",
        "interval": 30,
        "on-click": "~/.local/bin/keylight-toggle"
    }
}
```

## JSON Output for Scripting

For general scripting, use the `--json` flag:

```bash
# List all lights as JSON
keylightctl light list --json

# List all groups as JSON
keylightctl group list --json
```

Example light JSON output:

```json
[
  {
    "id": "Elgato Key Light ABC1",
    "product_name": "Elgato Key Light",
    "serial_number": "ABC123",
    "firmware_version": "1.0.3",
    "firmware_build": 200,
    "on": true,
    "brightness": 75,
    "temperature": 222,
    "temperature_kelvin": 4504,
    "ip": "192.168.1.100",
    "port": 9123,
    "last_seen": 1699900000
  }
]
```

## Hyprland Keybindings

Example keybindings for Hyprland (`~/.config/hypr/hyprland.conf`):

```bash
# Toggle lights
bind = $mainMod, F5, exec, ~/.local/bin/keylight-toggle

# Brightness control
bind = $mainMod SHIFT, F5, exec, keylightctl group set default brightness 100
bind = $mainMod CTRL, F5, exec, keylightctl group set default brightness 50
```
