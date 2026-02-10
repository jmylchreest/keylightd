---
sidebar_position: 2
---

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
    "id": "request-id-if-provided"
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

Changes properties of a specific light. Supports both single-property and multi-property modes.

**Single-property mode:**
```json
// Request
{
    "action": "set_light_state",
    "id": "optional-request-id",
    "data": {
        "id": "Elgato Key Light ABC1._elg._tcp.local.",
        "property": "on",
        "value": true
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id"
}
```

**Multi-property mode** — set multiple properties in a single request:
```json
// Request
{
    "action": "set_light_state",
    "id": "optional-request-id",
    "data": {
        "id": "Elgato Key Light ABC1._elg._tcp.local.",
        "on": true,
        "brightness": 80,
        "temperature": 3200
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
    "groups": [
        {
            "id": "group-123451",
            "name": "office",
            "lights": [
                "Elgato Key Light ABC1._elg._tcp.local.",
                "Elgato Key Light XYZ2._elg._tcp.local."
            ]
        }
    ]
}
```

### Get Group

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

```json
// Request
{
    "action": "create_group",
    "id": "optional-request-id",
    "data": {
        "name": "Office Lights",
        "lights": ["Elgato Key Light ABC1._elg._tcp.local.", "Elgato Key Light XYZ2._elg._tcp.local."]
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

Supports both single-property and multi-property modes. The `id` field supports **comma-separated values** to target multiple groups by ID or name.

**Single-property mode:**
```json
// Request
{
    "action": "set_group_state",
    "id": "optional-request-id",
    "data": {
        "id": "group-123451",
        "property": "on",
        "value": true
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id"
}
```

**Multi-property mode:**
```json
// Request
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

// Response
{
    "status": "ok",
    "id": "optional-request-id"
}
```

**Multi-group targeting** — target multiple groups with comma-separated IDs or names:
```json
// Request
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

**Partial failure response** — if some groups or lights fail:
```json
{
    "status": "partial",
    "id": "optional-request-id",
    "errors": [
        "group group-123451: failed to set brightness: device unavailable"
    ]
}
```

## API Key Operations

### List API Keys

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

```json
// Request
{
    "action": "apikey_add",
    "id": "optional-request-id",
    "data": {
        "name": "My New API Key",
        "expires_in": "86400"
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

### Set API Key Disabled Status

```json
// Request
{
    "action": "apikey_set_disabled_status",
    "id": "optional-request-id",
    "data": {
        "key_or_name": "api-key-value or key-name",
        "disabled": "true"
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

## System Operations

### Ping

Simple health check that returns a pong response.

```json
// Request
{
    "action": "ping",
    "id": "optional-request-id"
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "message": "pong"
}
```

### Health

Returns the service health status.

```json
// Request
{
    "action": "health",
    "id": "optional-request-id"
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "health": "ok"
}
```

### Version

Returns the running daemon's version, commit, and build date.

```json
// Request
{
    "action": "version",
    "id": "optional-request-id"
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "version": "0.1.1",
    "commit": "abc1234",
    "build_date": "2024-03-20T10:00:00Z"
}
```

### Subscribe to Events

Subscribes the connection to real-time state change events. After subscribing, the server will stream events as newline-delimited JSON (NDJSON) until the client disconnects.

```json
// Request
{
    "action": "subscribe_events",
    "id": "optional-request-id"
}

// Initial Response
{
    "status": "ok",
    "id": "optional-request-id",
    "subscribed": true
}

// Subsequent event messages (NDJSON stream)
{"type": "light_state_changed", "data": {"id": "Elgato Key Light ABC1._elg._tcp.local.", "on": true, "brightness": 80}}
{"type": "group_state_changed", "data": {"id": "group-123451", "property": "brightness", "value": 75}}
```

:::note
After subscribing, the connection enters streaming mode. No further request/response interactions are possible on this connection — it is dedicated to receiving events until disconnected.
:::

## Logging Operations

### List Filters

Returns the current global log level and all active log filters.

```json
// Request
{
    "action": "list_filters",
    "id": "optional-request-id"
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "level": "info",
    "filters": [
        {
            "type": "source",
            "pattern": "internal/server/*",
            "level": "debug",
            "enabled": true
        }
    ]
}
```

### Set Filters

Validates and replaces all active log filters. Invalid filters are rejected entirely.

```json
// Request
{
    "action": "set_filters",
    "id": "optional-request-id",
    "data": {
        "filters": [
            {
                "type": "source",
                "pattern": "internal/server/*",
                "level": "debug",
                "enabled": true
            }
        ]
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "level": "info",
    "filters": [
        {
            "type": "source",
            "pattern": "internal/server/*",
            "level": "debug",
            "enabled": true
        }
    ]
}
```

Each filter object supports:

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Filter type (e.g., `"source"`) |
| `pattern` | string | Glob pattern to match |
| `level` | string | Minimum level for matching entries |
| `output_level` | string | (Optional) Override output level |
| `enabled` | boolean | Whether this filter is active |
| `expires_at` | string | (Optional) RFC3339 expiration timestamp |

### Set Level

Changes the global log level at runtime. Valid values: `debug`, `info`, `warn`, `error`.

```json
// Request
{
    "action": "set_level",
    "id": "optional-request-id",
    "data": {
        "level": "debug"
    }
}

// Response
{
    "status": "ok",
    "id": "optional-request-id",
    "level": "debug"
}
```

## Error Codes

Common error messages:

| Error Message | Description |
|---------------|-------------|
| "Invalid action" | The specified action doesn't exist |
| "Missing required parameter: X" | A required parameter is missing |
| "Light not found: X" | The specified light doesn't exist |
| "Group not found: X" | The specified group doesn't exist |
| "API key not found: X" | The specified API key doesn't exist |
| "Device unavailable" | The light device couldn't be reached |
| "Invalid input: X" | The provided input is invalid |

## Example Usage

### Using netcat

```bash
echo '{"action": "list_lights"}' | nc -U /run/user/1000/keylightd.sock
```

### Using Python

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
