# Socket Interface

This guide explains how to control lights using the keylightd Unix socket API.

## Authentication

The Unix socket interface relies on Unix socket permissions for security. Only processes running as the same user as keylightd can access the socket, providing inherent security without additional authentication.

## Socket Location

The Unix socket is located at:
- `$XDG_RUNTIME_DIR/keylightd.sock` (default on most systems)
- Or fallback to `/run/user/<uid>/keylightd.sock`

## Listing Lights

List all discovered lights:

```bash
echo '{"action": "list_lights"}' | nc -U /run/user/$(id -u)/keylightd.sock
```

Response format:
```json
{
  "status": "ok",
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

## Getting Light Information

Get information about a specific light:

```bash
echo '{"action": "get_light", "data": {"id": "LIGHT_ID"}}' | \
  nc -U /run/user/$(id -u)/keylightd.sock
```

Response format:
```json
{
  "status": "ok",
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

## Controlling Lights

The Unix socket API requires setting properties individually, unlike the CLI and HTTP API which can set multiple properties at once.

### Power Control

Turn a light on:

```bash
echo '{"action": "set_light_state", "data": {"id": "LIGHT_ID", "property": "on", "value": true}}' | \
  nc -U /run/user/$(id -u)/keylightd.sock
```

Turn a light off:

```bash
echo '{"action": "set_light_state", "data": {"id": "LIGHT_ID", "property": "on", "value": false}}' | \
  nc -U /run/user/$(id -u)/keylightd.sock
```

### Brightness Control

Set brightness (0-100):

```bash
echo '{"action": "set_light_state", "data": {"id": "LIGHT_ID", "property": "brightness", "value": 75}}' | \
  nc -U /run/user/$(id -u)/keylightd.sock
```

### Color Temperature Control

Set color temperature in Kelvin (2900-7000):

```bash
echo '{"action": "set_light_state", "data": {"id": "LIGHT_ID", "property": "temperature", "value": 4500}}' | \
  nc -U /run/user/$(id -u)/keylightd.sock
```

## Response Formats

### Success Response

Successful operations return:
```json
{
  "status": "ok"
}
```

### Error Responses

Error responses include an error message:
```json
{
  "error": "Light not found: LIGHT_ID"
}
```

Common error messages:
- "Light not found: X" - The specified light doesn't exist
- "Device unavailable" - The light device couldn't be reached
- "Invalid input: X" - The provided input is invalid (e.g., brightness outside 0-100 range)
- "Missing required parameter: X" - A required parameter is missing

## Light Properties

### Controllable Properties
- **on**: Power state (boolean true/false)
- **brightness**: Brightness level (integer 0-100)
- **temperature**: Color temperature in Kelvin (integer 2900-7000)

### Read-only Properties
- **id**: Unique light identifier
- **productname**: Product name from the device
- **serialnumber**: Device serial number
- **firmwareversion**: Firmware version
- **firmwarebuild**: Firmware build number
- **ip**: IP address of the light
- **port**: Port number (usually 9123)
- **lastseen**: Timestamp when the light was last seen

## Using Python

Here's an example using Python instead of netcat:

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
result = json.loads(response)
print(result)
sock.close()
```

## Request Format

All socket requests follow this format:

```json
{
  "action": "action_name",
  "id": "optional-request-id",
  "data": {
    "parameter1": "value1",
    "parameter2": "value2"
  }
}
```

The `id` field is optional and will be included in the response if provided.

## Multiple Property Changes

To change multiple properties, you need to send separate requests for each property:

```bash
# Turn on light and set brightness and temperature
echo '{"action": "set_light_state", "data": {"id": "LIGHT_ID", "property": "on", "value": true}}' | \
  nc -U /run/user/$(id -u)/keylightd.sock

echo '{"action": "set_light_state", "data": {"id": "LIGHT_ID", "property": "brightness", "value": 80}}' | \
  nc -U /run/user/$(id -u)/keylightd.sock

echo '{"action": "set_light_state", "data": {"id": "LIGHT_ID", "property": "temperature", "value": 3200}}' | \
  nc -U /run/user/$(id -u)/keylightd.sock
```