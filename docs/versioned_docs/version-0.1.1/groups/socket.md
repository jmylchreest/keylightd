---
sidebar_position: 3
---

# Socket Interface

This guide explains how to manage light groups using the keylightd Unix socket API.

## Socket Location

The Unix socket is located at:
- `$XDG_RUNTIME_DIR/keylightd.sock` (default on most systems)
- Or fallback to `/run/user/<uid>/keylightd.sock`

## Authentication

The Unix socket interface relies on Unix socket permissions for security. Only processes running as the same user as keylightd can access the socket.

## Group Operations

### List Groups

Retrieves all configured light groups.

**Request:**
```json
{
    "action": "list_groups",
    "id": "optional-request-id"
}
```

**Response:**
```json
{
    "status": "ok",
    "id": "optional-request-id",
    "groups": [
        {
            "id": "group-123451",
            "name": "office",
            "lights": [
                "Elgato Key Light ABC1._elg._tcp.local.",
                "Elgato Key Light XYZ2._elg._tcp.local."
            ]
        },
        {
            "id": "group-123452",
            "name": "office-left",
            "lights": [
                "Elgato Key Light ABC1._elg._tcp.local."
            ]
        }
    ]
}
```

### Get Group

Retrieves information about a specific group.

**Request:**
```json
{
    "action": "get_group",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451"
    }
}
```

**Response:**
```json
{
    "status": "ok",
    "id": "optional-request-id",
    "group": {
        "id": "group-123451",
        "name": "Office Lights",
        "lights": [
            "Elgato Key Light ABC1._elg._tcp.local.",
            "Elgato Key Light XYZ2._elg._tcp.local."
        ]
    }
}
```

### Create Group

Creates a new light group.

**Request:**
```json
{
    "action": "create_group",
    "id": "optional-request-id",
    "data": {
        "name": "Office Lights",
        "lights": [
            "Elgato Key Light ABC1._elg._tcp.local.",
            "Elgato Key Light XYZ2._elg._tcp.local."
        ]
    }
}
```

**Response:**
```json
{
    "status": "ok",
    "id": "optional-request-id",
    "group": {
        "id": "group-123451",
        "name": "Office Lights",
        "lights": [
            "Elgato Key Light ABC1._elg._tcp.local.",
            "Elgato Key Light XYZ2._elg._tcp.local."
        ]
    }
}
```

### Delete Group

Deletes a light group.

**Request:**
```json
{
    "action": "delete_group",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451"
    }
}
```

**Response:**
```json
{
    "status": "ok",
    "id": "optional-request-id"
}
```

### Set Group Lights

Updates the list of lights in a group.

**Request:**
```json
{
    "action": "set_group_lights",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451",
        "lights": [
            "Elgato Key Light ABC1._elg._tcp.local.",
            "Elgato Key Light XYZ2._elg._tcp.local."
        ]
    }
}
```

**Response:**
```json
{
    "status": "ok",
    "id": "optional-request-id"
}
```

### Set Group State

Changes properties for all lights in one or more groups simultaneously. Supports both single-property and multi-property modes.

The `id` field supports **comma-separated values** to target multiple groups at once. Groups can be matched by ID or name, and duplicates are automatically deduplicated.

#### Single-Property Mode

**Request:**
```json
{
    "action": "set_group_state",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451",
        "property": "on",
        "value": true
    }
}
```

#### Multi-Property Mode

Set multiple properties in a single request by including them directly in `data`:

**Request:**
```json
{
    "action": "set_group_state",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451",
        "on": true,
        "brightness": 80,
        "temperature": 3200
    }
}
```

#### Multi-Group Targeting

Target multiple groups by separating IDs or names with commas:

**Request:**
```json
{
    "action": "set_group_state",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451,office-lights",
        "on": true,
        "brightness": 75
    }
}
```

#### Success Response

```json
{
    "status": "ok",
    "id": "optional-request-id"
}
```

#### Partial Failure Response

If some lights or groups fail while others succeed, a partial response is returned:

```json
{
    "status": "partial",
    "id": "optional-request-id",
    "errors": [
        "group group-123451: failed to set brightness: device unavailable"
    ]
}
```

#### Group State Properties

| Property | Type | Valid Range | Description |
|----------|------|------------|-------------|
| `on` | boolean | `true` or `false` | Power state for all lights in group |
| `brightness` | integer | 0-100 | Brightness percentage for all lights |
| `temperature` | integer | 2900-7000 | Color temperature in Kelvin for all lights |

## Example Usage

### Using netcat

List all groups:
```bash
echo '{"action": "list_groups"}' | nc -U /run/user/1000/keylightd.sock
```

Create a new group:
```bash
echo '{"action": "create_group", "data": {"name": "Desk Setup", "lights": ["Elgato Key Light ABC1._elg._tcp.local."]}}' | nc -U /run/user/1000/keylightd.sock
```

Turn on all lights in a group (single-property mode):
```bash
echo '{"action": "set_group_state", "data": {"id": "group-123451", "property": "on", "value": true}}' | nc -U /run/user/1000/keylightd.sock
```

Set multiple properties at once (multi-property mode):
```bash
echo '{"action": "set_group_state", "data": {"id": "group-123451", "on": true, "brightness": 80, "temperature": 3200}}' | nc -U /run/user/1000/keylightd.sock
```

Target multiple groups at once:
```bash
echo '{"action": "set_group_state", "data": {"id": "group-123451,office-lights", "on": true}}' | nc -U /run/user/1000/keylightd.sock
```

### Using Python

```python
import socket
import json

def send_socket_request(request):
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.connect('/run/user/1000/keylightd.sock')
    
    sock.sendall(json.dumps(request).encode('utf-8'))
    response = sock.recv(4096).decode('utf-8')
    sock.close()
    
    return json.loads(response)

# List all groups
groups = send_socket_request({"action": "list_groups"})
print("Groups:", groups)

# Create a new group
new_group = send_socket_request({
    "action": "create_group",
    "data": {
        "name": "Studio Lights",
        "lights": ["Elgato Key Light ABC1._elg._tcp.local."]
    }
})
print("Created group:", new_group)

# Set brightness for all lights in group (single-property mode)
send_socket_request({
    "action": "set_group_state",
    "data": {
        "id": new_group["group"]["id"],
        "property": "brightness",
        "value": 75
    }
})

# Set multiple properties at once (multi-property mode)
send_socket_request({
    "action": "set_group_state",
    "data": {
        "id": new_group["group"]["id"],
        "on": True,
        "brightness": 80,
        "temperature": 3200
    }
})
```

## Error Handling

Common error responses when working with groups:

```json
{
    "error": "Group not found: group-123456789",
    "id": "optional-request-id"
}
```

```json
{
    "error": "Light not found: invalid-light-id",
    "id": "optional-request-id"
}
```

```json
{
    "error": "Missing required parameter: name",
    "id": "optional-request-id"
}
```