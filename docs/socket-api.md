# keylightd Unix Socket API

The keylightd daemon exposes a Unix socket interface for local control of Elgato Keylights. The socket is located at `$XDG_RUNTIME_DIR/keylightd.sock` or `/run/user/<uid>/keylightd.sock` by default.

## Protocol

The API uses JSON for request and response messages. Each request must include an `action` field specifying the operation to perform.

## Authentication

The Unix socket interface is only accessible to the local user, providing inherent security through Unix socket permissions.

## API Endpoints

### List Lights
```json
// Request
{
    "action": "list_lights"
}

// Response
{
    "light-id-1": {
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

### Get Light
```json
// Request
{
    "action": "get_light",
    "id": "light-id-1"
}

// Response
{
    "light": {
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
```json
// Request
{
    "action": "set_light_state",
    "id": "light-id-1",
    "property": "on",
    "value": true
}

// Response
{
    "success": true
}
```

### List Groups
```json
// Request
{
    "action": "list_groups"
}

// Response
{
  "status": "ok",
  "groups": {
    "group-1746907750613172530": {
      "name": "office",
      "lights": [
        "Elgato Key Light 1E2D._elg._tcp.local.",
        "Elgato Key Light 218C._elg._tcp.local."
      ]
    },
    "group-1746947787717716493": {
      "name": "office-left",
      "lights": [
        "Elgato Key Light 1E2D._elg._tcp.local."
      ]
    },
    "group-1746947790203301385": {
      "name": "office-right",
      "lights": [
        "Elgato Key Light 218C._elg._tcp.local."
      ]
    }
  }
}
```

### Get Group
```json
// Request
{
    "action": "get_group",
    "id": "group-1"
}

// Response
{
    "group": {
        "id": "group-1",
        "name": "Office Lights",
        "lights": ["light-id-1", "light-id-2"]
    }
}
```

### Create Group
```json
// Request
{
    "action": "create_group",
    "name": "Office Lights"
}

// Response
{
    "group": {
        "id": "group-1",
        "name": "Office Lights",
        "lights": []
    }
}
```

### Set Group State
```json
// Request
{
    "action": "set_group_state",
    "id": "group-1",
    "property": "on",
    "value": true
}

// Response
{
    "success": true
}
```

### Delete Group
```json
// Request
{
    "action": "delete_group",
    "id": "group-1"
}

// Response
{
    "success": true
}
```

### Set Group Lights
```json
// Request
{
    "action": "set_group_lights",
    "id": "group-1",
    "lights": ["light-id-1", "light-id-2"]
}

// Response
{
    "success": true
}
```

## Error Responses

All endpoints may return an error response in the following format:

```json
{
    "error": "error message describing what went wrong"
}
```

## Property Values

### Light Properties
- `on`: boolean (true/false)
- `brightness`: integer (0-100 %)
- `temperature`: integer (2900-7000 Kelvin)

### Group Properties
- `on`: boolean (true/false)
- `brightness`: integer (0-100 %)
- `temperature`: integer (2900-7000 Kelvin)