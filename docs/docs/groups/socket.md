# Groups - Unix Socket API

The keylightd Unix socket interface provides comprehensive group management capabilities for organizing and controlling multiple Elgato Key Lights simultaneously.

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
    "groups": {
        "group-123451": {
            "id": "group-123451",
            "name": "office",
            "lights": [
                "Elgato Key Light ABC1._elg._tcp.local.",
                "Elgato Key Light XYZ2._elg._tcp.local."
            ]
        },
        "group-123452": {
            "id": "group-123452",
            "name": "office-left",
            "lights": [
                "Elgato Key Light ABC1._elg._tcp.local."
            ]
        }
    }
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

Changes a property for all lights in a group simultaneously.

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

**Response:**
```json
{
    "status": "ok",
    "id": "optional-request-id"
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

Turn on all lights in a group:
```bash
echo '{"action": "set_group_state", "data": {"id": "group-123451", "property": "on", "value": true}}' | nc -U /run/user/1000/keylightd.sock
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

# Set brightness for all lights in group
send_socket_request({
    "action": "set_group_state",
    "data": {
        "id": new_group["group"]["id"],
        "property": "brightness",
        "value": 75
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