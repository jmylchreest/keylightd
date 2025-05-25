# CLI Interface

This guide explains how to control lights using the `keylightctl` command-line tool.

## Discovering Lights

View all discovered lights:

```bash
keylightctl light list
```

For parseable output:

```bash
keylightctl light list --parseable
```

This shows all discovered lights with their IDs, names, IP addresses, and current state.

## Getting Light Information

View the status of a specific light:

```bash
keylightctl light get LIGHT_ID
```

Get a specific property:

```bash
keylightctl light get LIGHT_ID on
keylightctl light get LIGHT_ID brightness
keylightctl light get LIGHT_ID temperature
```

For parseable output:

```bash
keylightctl light get LIGHT_ID --parseable
```

## Controlling Lights

The basic syntax for setting light properties is:

```bash
keylightctl light set LIGHT_ID PROPERTY VALUE
```

### Power Control

Turn a light on:

```bash
keylightctl light set LIGHT_ID on true
```

Turn a light off:

```bash
keylightctl light set LIGHT_ID on false
```

You can also use "on" and "off" as values:

```bash
keylightctl light set LIGHT_ID on on
keylightctl light set LIGHT_ID on off
```

### Brightness Control

Set brightness (0-100):

```bash
keylightctl light set LIGHT_ID brightness 75
keylightctl light set LIGHT_ID brightness 0
keylightctl light set LIGHT_ID brightness 100
```

### Color Temperature Control

Set color temperature in Kelvin (2900-7000):

```bash
keylightctl light set LIGHT_ID temperature 4500
keylightctl light set LIGHT_ID temperature 2900  # Warm
keylightctl light set LIGHT_ID temperature 7000  # Cool
```

The CLI will automatically clamp values to the valid range and show you the conversion to mireds.

## Interactive Mode

If you don't provide all required arguments, `keylightctl` will prompt you interactively:

```bash
# This will show you a list of lights to choose from
keylightctl light get

# This will prompt for light, property, and value
keylightctl light set
```

## Light Properties

Each Key Light has the following controllable properties:

- **on**: Power state (true/false, on/off)
- **brightness**: Brightness level (0-100)
- **temperature**: Color temperature in Kelvin (2900-7000)

## Examples

Common usage patterns:

```bash
# Get all lights
keylightctl light list

# Get specific light info
keylightctl light get "Elgato Key Light ABC1._elg._tcp.local."

# Turn on a light at 80% brightness, warm temperature
keylightctl light set "Elgato Key Light ABC1._elg._tcp.local." on true
keylightctl light set "Elgato Key Light ABC1._elg._tcp.local." brightness 80
keylightctl light set "Elgato Key Light ABC1._elg._tcp.local." temperature 3200

# Turn off a light
keylightctl light set "Elgato Key Light ABC1._elg._tcp.local." on false
```

## Light IDs

Light IDs are typically in the format `"Elgato Key Light XXXX._elg._tcp.local."` where XXXX is a unique identifier. Use quotes around light IDs that contain spaces or special characters.

You can also use the shortened form if it's unique enough to identify the light.