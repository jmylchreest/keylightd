---
sidebar_position: 1
---

# CLI Interface

This guide explains how to manage light groups using the `keylightctl` command-line tool.

## Creating Groups

Create a new group:

```bash
keylightctl group add my-group
```

You can also specify the name as a flag:

```bash
keylightctl group add --name my-group
```

Or without any arguments to be prompted interactively:

```bash
keylightctl group add
```

## Listing Groups

View all groups:

```bash
keylightctl group list
```

For parseable output:

```bash
keylightctl group list --parseable
```

This shows all groups with their IDs, names, and member lights.

## Getting Group Information

View the details of a specific group:

```bash
keylightctl group get GROUP_ID
```

For parseable output:

```bash
keylightctl group get GROUP_ID --parseable
```

## Controlling Groups

The basic syntax for setting group properties is:

```bash
keylightctl group set GROUP_ID PROPERTY VALUE
```

### Power Control

Turn all lights in a group on:

```bash
keylightctl group set GROUP_ID on true
```

Turn all lights in a group off:

```bash
keylightctl group set GROUP_ID on false
```

You can also use "on" and "off" as values:

```bash
keylightctl group set GROUP_ID on on
keylightctl group set GROUP_ID on off
```

### Brightness Control

Set brightness for all lights in a group (0-100):

```bash
keylightctl group set GROUP_ID brightness 80
```

### Color Temperature Control

Set color temperature for all lights in a group in Kelvin (2900-7000):

```bash
keylightctl group set GROUP_ID temperature 4500
```

## Modifying Group Membership

Edit the lights in a group:

```bash
keylightctl group edit GROUP_ID light-1 light-2 light-3
```

This replaces all lights in the group with the specified lights.

## Deleting Groups

Delete a group:

```bash
keylightctl group delete GROUP_ID
```

Skip the confirmation prompt:

```bash
keylightctl group delete GROUP_ID --yes
```

This removes the group but does not affect any lights.

## Interactive Mode

If you don't provide all required arguments, `keylightctl` will prompt you interactively:

```bash
# This will show you a list of groups to choose from
keylightctl group get

# This will prompt for group, property, and value
keylightctl group set

# This will prompt for group name
keylightctl group add
```

## Group Properties

Groups support the same control operations as individual lights:

- **on**: Power state (true/false, on/off)
- **brightness**: Brightness level (0-100)
- **temperature**: Color temperature in Kelvin (2900-7000)

## Examples

Common usage patterns:

```bash
# Create a group
keylightctl group add office-lights

# List all groups
keylightctl group list

# Get group info
keylightctl group get group-123451

# Turn on all lights in a group at 80% brightness, warm temperature
keylightctl group set group-123451 on true
keylightctl group set group-123451 brightness 80
keylightctl group set group-123451 temperature 3200

# Edit group membership
keylightctl group edit group-123451 "Elgato Key Light ABC1._elg._tcp.local." "Elgato Key Light XYZ2._elg._tcp.local."

# Delete a group
keylightctl group delete group-123451
```

## Group IDs

Group IDs are typically in the format "group-" followed by a numeric identifier (e.g., "group-123451"). You can often use the group name as well for identification in some commands.