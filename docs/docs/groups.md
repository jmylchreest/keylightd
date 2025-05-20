# Managing Light Groups

This guide explains how to create and manage groups of lights using KeylightD.

## What are Light Groups?

Groups allow you to control multiple lights together. This is particularly useful for:

- Setting up identical lighting for multiple lights in a room
- Creating scenes where all lights need to change together
- Simplifying control of complex lighting setups

## Creating Groups

### Creating Groups via CLI

To create a new group using the command line:

```bash
keylightctl group create --name my-group --lights light-1,light-2
```

You can specify multiple light IDs as a comma-separated list.

### Creating Groups via API

To create a group using the API:

```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-group", "light_ids": ["light-1", "light-2"]}' \
  http://localhost:8080/api/v1/groups
```

## Listing Groups

### Listing Groups via CLI

To list all groups:

```bash
keylightctl group list
```

This shows all groups with their IDs, names, and member lights.

### Listing Groups via API

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/groups
```

## Getting Group Details

### Getting Group Details via CLI

To view the details of a specific group:

```bash
keylightctl group get --id group-1234
```

### Getting Group Details via API

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/groups/group-1234
```

## Controlling Groups

Groups support the same control operations as individual lights.

### Setting Group State via CLI

Turn all lights in a group on:

```bash
keylightctl group set --id group-1234 --on
```

Set brightness for all lights in a group:

```bash
keylightctl group set --id group-1234 --brightness 80
```

Set color temperature for all lights in a group:

```bash
keylightctl group set --id group-1234 --temperature 4500
```

Combine multiple settings:

```bash
keylightctl group set --id group-1234 --on --brightness 75 --temperature 3200
```

### Setting Group State via API

```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true, "brightness": 75, "temperature": 3200}' \
  http://localhost:8080/api/v1/groups/group-1234/state
```

## Modifying Group Membership

### Modifying Group Lights via CLI

To update the lights in a group:

```bash
keylightctl group set-lights --id group-1234 --lights light-1,light-3,light-4
```

This replaces all lights in the group with the specified lights.

### Modifying Group Lights via API

```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"light_ids": ["light-1", "light-3", "light-4"]}' \
  http://localhost:8080/api/v1/groups/group-1234/lights
```

## Deleting Groups

### Deleting Groups via CLI

To delete a group:

```bash
keylightctl group delete --id group-1234
```

This removes the group but does not affect any lights.

### Deleting Groups via API

```bash
curl -X DELETE \
  -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/groups/group-1234
```

## Advanced Group Features

### Multiple Group Identifiers

When controlling groups via the API, you can use multiple group identifiers in a single request by separating them with commas:

```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true}' \
  http://localhost:8080/api/v1/groups/group-1234,my-other-group/state
```

This will apply the settings to all matching groups, whether they're matched by ID or name.

### Error Handling

If an operation fails for some lights in a group but succeeds for others, the API will return a `207 Multi-Status` response with details about which operations failed.

## Best Practices

- Use meaningful group names that describe the location or purpose
- Keep groups focused on specific functional areas
- Consider creating separate groups for different use cases even if they contain the same lights
- Use groups to standardize settings across multiple lights

## Troubleshooting

### Group Command Not Affecting All Lights

If a group command doesn't affect all lights:

1. Check if all lights in the group are online using `keylightctl light list`
2. Verify the group's membership using `keylightctl group get`
3. Try sending commands to individual lights to see if they respond

### Failed Group Operations

If a group operation fails:

1. Check for error messages in the response
2. Verify that all lights in the group are accessible
3. Try with a simpler command (e.g., just turning the lights on)

## Next Steps

- Read about [controlling individual lights](lights.md)
- Review the [API reference](api/index.md) for detailed API documentation