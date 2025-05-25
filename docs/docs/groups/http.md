# HTTP Interface

This guide explains how to manage light groups using the keylightd REST API.

## Authentication

All HTTP API requests must include a valid API key using one of these methods:

**Method 1: HTTP Bearer Authentication (Recommended)**
```bash
curl -H "Authorization: Bearer YOUR_API_KEY" http://localhost:8080/api/v1/groups
```

**Method 2: Custom Header**
```bash
curl -H "X-API-Key: YOUR_API_KEY" http://localhost:8080/api/v1/groups
```

## Creating Groups

Create a new group:

```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-group"}' \
  http://localhost:8080/api/v1/groups
```

Create a group with initial lights:

```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-group", "light_ids": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]}' \
  http://localhost:8080/api/v1/groups
```

Response format:
```json
{
  "id": "group-123451",
  "name": "my-group",
  "lights": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]
}
```

## Listing Groups

List all groups:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/groups
```

Response format:
```json
{
  "group-123451": {
    "id": "group-123451",
    "name": "office-lights",
    "lights": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]
  },
  "group-123452": {
    "id": "group-123452",
    "name": "desk-lights",
    "lights": ["Elgato Key Light ABC1._elg._tcp.local."]
  }
}
```

## Getting Group Information

Get details of a specific group:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/groups/GROUP_ID
```

Response format:
```json
{
  "id": "group-123451",
  "name": "office-lights",
  "lights": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]
}
```

## Controlling Groups

Set group state by sending a PUT request to the group's state endpoint:

```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true, "brightness": 75, "temperature": 3200}' \
  http://localhost:8080/api/v1/groups/GROUP_ID/state
```

You can include any combination of the following properties in the request body:
- `on` (boolean): Power state
- `brightness` (integer 0-100): Brightness level  
- `temperature` (integer 2900-7000): Color temperature in Kelvin

**Note:** Unlike individual lights, the Unix socket API sets group properties individually, but the HTTP API can set multiple properties at once.

### Power Control

Turn all lights in a group on:
```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true}' \
  http://localhost:8080/api/v1/groups/GROUP_ID/state
```

Turn all lights in a group off:
```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": false}' \
  http://localhost:8080/api/v1/groups/GROUP_ID/state
```

### Brightness Control

Set brightness for all lights in a group:
```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"brightness": 80}' \
  http://localhost:8080/api/v1/groups/GROUP_ID/state
```

### Color Temperature Control

Set color temperature for all lights in a group:
```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"temperature": 4500}' \
  http://localhost:8080/api/v1/groups/GROUP_ID/state
```

## Modifying Group Membership

Update the lights in a group:

```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"light_ids": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]}' \
  http://localhost:8080/api/v1/groups/GROUP_ID/lights
```

This replaces all lights in the group with the specified lights.

## Deleting Groups

Delete a group:

```bash
curl -X DELETE \
  -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/groups/GROUP_ID
```

This removes the group but does not affect any lights. Successful deletion returns HTTP 204 No Content with no response body.

## Advanced Features

### Multiple Group Identifiers

You can use multiple group identifiers in a single request by separating them with commas:

```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true}' \
  "http://localhost:8080/api/v1/groups/group-123451,office-lights/state"
```

This will apply the settings to all matching groups, whether they're matched by ID or name. The server will deduplicate groups if the same group is matched by both ID and name.

### Error Handling

If an operation fails for some lights in a group but succeeds for others, the API will return a `207 Multi-Status` response with details about which operations failed.

## Response Formats

### Success Response

Successful state changes return:
```json
{
  "status": "ok"
}
```

### Multi-Status Response

Partial failures return:
```json
{
  "status": "partial",
  "errors": [
    "Failed to update light-2: device unavailable"
  ]
}
```

### Error Responses

Error responses include appropriate HTTP status codes with JSON error messages:

```json
{
  "error": "Group not found"
}
```

Common status codes:
- `200` - Success
- `201` - Created (for new groups)
- `204` - No Content (for deletions)
- `207` - Multi-Status (partial success)
- `400` - Bad Request (invalid parameters)
- `401` - Unauthorized (invalid API key)
- `404` - Group not found
- `500` - Internal Server Error

## Group Properties

### Controllable Properties
- **on**: Power state (boolean)
- **brightness**: Brightness level (integer 0-100)
- **temperature**: Color temperature in Kelvin (integer 2900-7000)

### Group Information
- **id**: Unique group identifier
- **name**: Human-readable group name
- **lights**: Array of light IDs in the group