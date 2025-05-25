# Unix Socket API

The keylightd daemon exposes a Unix socket interface for local control of Elgato Key Lights. This method is preferable for local automation scripts and command-line tooling, while the HTTP REST API is better suited for remote or web-based integrations.

## Socket Location

The Unix socket is located at:
- `$XDG_RUNTIME_DIR/keylightd.sock` (default on most systems)
- Or fallback to `/run/user/<uid>/keylightd.sock`

You can override this location in the configuration file or through the `--socket` command-line flag when starting keylightd.

## Protocol Overview

The API uses JSON for both request and response messages. Each request requires an `action` field specifying the operation to perform, and most operations require additional parameters.

## Authentication

The Unix socket interface relies on Unix socket permissions for security. Only processes running as the same user as keylightd can access the socket, providing inherent security without additional authentication.

## General Response Format

Successful responses have this structure:

```json
{
    "status": "ok",
    "id": "request-id-if-provided",
    // Additional data specific to the request
}
```

Error responses look like this:

```json
{
    "error": "Error message explaining what went wrong",
    "id": "request-id-if-provided"
}
```

## Light Operations

### List Lights

Retrieves all discovered lights.

```json
// Request
{
    "action": "list_lights",
    "id": "optional-request-id"
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "lights": {
        "Elgato Key Light ABC1._elg._tcp.local.": {
            "id": "Elgato Key Light ABC1._elg._tcp.local.",
            "productname": "Elgato Key Light",
            "serialnumber": "ABC123456",
            "firmwareversion": "1.0.3",
            "firmwarebuild": 194,
            "on": true,
            "brightness": 50,
            "temperature": 5000,
            "ip": "192.168.1.100",
            "port": 9123,
            "lastseen": "2024-03-20T10:00:00Z"
        }
    }
}
```

### Get Light

Retrieves information about a specific light.

```json
// Request
{
    "action": "get_light",
    "id": "optional-request-id",
    "data": {
        "id": "Elgato Key Light ABC1._elg._tcp.local."
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "light": {
        "id": "Elgato Key Light ABC1._elg._tcp.local.",
        "productname": "Elgato Key Light",
        "serialnumber": "ABC123456",
        "firmwareversion": "1.0.3",
        "firmwarebuild": 194,
        "on": true,
        "brightness": 50,
        "temperature": 5000,
        "ip": "192.168.1.100",
        "port": 9123,
        "lastseen": "2024-03-20T10:00:00Z"
    }
}
```

### Set Light State

Changes properties of a specific light.

```json
// Request
{
    "action": "set_light_state",
    "id": "optional-request-id",
    "data": {
        "id": "Elgato Key Light ABC1._elg._tcp.local.",
        "property": "on",  // Can be "on", "brightness", or "temperature"
        "value": true      // Boolean for "on", integer for others
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id"
}
```

#### Light Properties

| Property | Type | Valid Range | Description |
|----------|------|------------|-------------|
| `on` | boolean | `true` or `false` | Power state of the light |
| `brightness` | integer | 0-100 | Brightness percentage |
| `temperature` | integer | 2900-7000 | Color temperature in Kelvin |

## Group Operations

### List Groups

Retrieves all configured light groups.

```json
// Request
{
    "action": "list_groups",
    "id": "optional-request-id"
}

// Response
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

```json
// Request
{
    "action": "get_group",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451"
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "group": {
        "id": "group-123451",
        "name": "Office Lights",
        "lights": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]
    }
}
```

### Create Group

Creates a new light group.

```json
// Request
{
    "action": "create_group",
    "id": "optional-request-id",
    "data": {
        "name": "Office Lights",
        "lights": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]  // Optional initial lights
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "group": {
        "id": "group-123451",
        "name": "Office Lights",
        "lights": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]
    }
}
```

### Delete Group

Deletes a light group.

```json
// Request
{
    "action": "delete_group",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451"
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id"
}
```

### Set Group Lights

Updates the list of lights in a group.

```json
// Request
{
    "action": "set_group_lights",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451",
        "lights": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id"
}
```

### Set Group State

Changes a property for all lights in a group.

```json
// Request
{
    "action": "set_group_state",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451",
        "property": "on",  // Can be "on", "brightness", or "temperature"
        "value": true      // Boolean for "on", integer for others
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id"
}
```

## API Key Operations

### List API Keys

Retrieves all configured API keys.

```json
// Request
{
    "action": "apikey_list",
    "id": "optional-request-id"
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "keys": [
        {
            "key": "api-key-value",
            "name": "My API Key",
            "created_at": "2024-03-20T10:00:00Z",
            "expires_at": "2024-03-21T10:00:00Z",
            "last_used_at": "2024-03-20T10:00:00Z",
            "disabled": false
        }
    ]
}
```

### Create API Key

Creates a new API key.

```json
// Request
{
    "action": "apikey_add",
    "id": "optional-request-id",
    "data": {
        "name": "My New API Key",
        "expires_in": "86400"  // Optional: expiration in seconds
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "key": {
        "key": "actual-key-value",
        "name": "My New API Key",
        "created_at": "2024-03-20T10:00:00Z",
        "expires_at": "2024-03-21T10:00:00Z",
        "last_used_at": "2024-03-20T10:00:00Z",
        "disabled": false
    }
}
```

### Delete API Key

Deletes an API key.

```json
// Request
{
    "action": "apikey_delete",
    "id": "optional-request-id",
    "data": {
        "key": "api-key-value"
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id"
}
```

## Error Codes

When an error occurs, you'll receive an error response with a descriptive message:

```json
{
    "error": "Light not found: Elgato Key Light ABC1._elg._tcp.local.",
    "id": "optional-request-id"
}
```

Common error messages:

| Error Message | Description |
|---------------|-------------|
| "Invalid action" | The specified action doesn't exist |
| "Missing required parameter: X" | A required parameter is missing |
| "Light not found: Elgato Key Light ABC1._elg._tcp.local." | The specified light doesn't exist |
| "Group not found: group-123451" | The specified group doesn't exist |
| "API key not found: X" | The specified API key doesn't exist |
| "Device unavailable" | The light device couldn't be reached |
| "Invalid input: X" | The provided input is invalid (e.g., brightness outside allowed range) |

### Set API Key Disabled Status

Enable or disable an API key.

```json
// Request
{
    "action": "apikey_set_disabled_status",
    "id": "optional-request-id",
    "data": {
        "key_or_name": "api-key-value or key-name",
        "disabled": "true"  // String "true" or "false"
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "key": {
        "key": "api-key-value",
        "name": "My API Key",
        "created_at": "2024-03-20T10:00:00Z",
        "expires_at": "2024-03-21T10:00:00Z",
        "last_used_at": "2024-03-20T10:00:00Z",
        "disabled": true
    }
}
```

## Example Usage

Here's an example using the `netcat` (`nc`) command to send a request to the socket:

```bash
echo '{"action": "list_lights"}' | nc -U /run/user/1000/keylightd.sock
```

Another example using Python:

```python
import socket
import json

sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
sock.connect('/run/user/1000/keylightd.sock')

request = {
    'action': 'set_light_state',
    'data': {
        'id': 'Elgato Key Light ABC1._elg._tcp.local.',
        'property': 'on',
        'value': True
    }
}

sock.sendall(json.dumps(request).encode('utf-8'))
response = sock.recv(4096).decode('utf-8')
print(json.loads(response))
sock.close()
```