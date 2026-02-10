---
sidebar_position: 2
---

# HTTP Interface

This guide explains how to control lights using the keylightd REST API.

## Authentication

All HTTP API requests must include a valid API key using one of these methods:

**Method 1: HTTP Bearer Authentication (Recommended)**
```bash
curl -H "Authorization: Bearer YOUR_API_KEY" http://localhost:9123/api/v1/lights
```

**Method 2: Custom Header**
```bash
curl -H "X-API-Key: YOUR_API_KEY" http://localhost:9123/api/v1/lights
```

## Listing Lights

List all discovered lights:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:9123/api/v1/lights
```

Response format:
```json
{
  "lights": {
    "Elgato Key Light ABC1._elg._tcp.local.": {
      "id": "Elgato Key Light ABC1._elg._tcp.local.",
      "name": "Elgato Key Light",
      "ip": "192.168.1.100",
      "port": 9123,
      "temperature": 4500,
      "brightness": 75,
      "on": true,
      "productname": "Elgato Key Light",
      "hardwareboardtype": 2,
      "firmwareversion": "1.0.3",
      "firmwarebuild": 123,
      "serialnumber": "KL12345678",
      "lastseen": "2023-08-15T14:30:45Z"
    }
  }
}
```

## Getting Light Information

Get a specific light:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:9123/api/v1/lights/Elgato%20Key%20Light%20ABC1._elg._tcp.local.
```

Response format:
```json
{
  "id": "Elgato Key Light ABC1._elg._tcp.local.",
  "name": "Elgato Key Light",
  "ip": "192.168.1.100",
  "port": 9123,
  "temperature": 4500,
  "brightness": 75,
  "on": true,
  "productname": "Elgato Key Light",
  "hardwareboardtype": 2,
  "firmwareversion": "1.0.3",
  "firmwarebuild": 123,
  "serialnumber": "KL12345678",
  "lastseen": "2023-08-15T14:30:45Z"
}
```

## Controlling Lights

Update light state by sending a POST request to the light's state endpoint:

```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true, "brightness": 80, "temperature": 3200}' \
  http://localhost:9123/api/v1/lights/Elgato%20Key%20Light%20ABC1._elg._tcp.local./state
```

You can include any combination of the following properties in the request body:
- `on` (boolean): Power state
- `brightness` (integer 0-100): Brightness level
- `temperature` (integer 2900-7000): Color temperature in Kelvin

### Power Control

Turn a light on:
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true}' \
  http://localhost:9123/api/v1/lights/Elgato%20Key%20Light%20ABC1._elg._tcp.local./state
```

Turn a light off:
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": false}' \
  http://localhost:9123/api/v1/lights/Elgato%20Key%20Light%20ABC1._elg._tcp.local./state
```

### Brightness Control

Set brightness to 75%:
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"brightness": 75}' \
  http://localhost:9123/api/v1/lights/Elgato%20Key%20Light%20ABC1._elg._tcp.local./state
```

### Color Temperature Control

Set color temperature to 4500K:
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"temperature": 4500}' \
  http://localhost:9123/api/v1/lights/Elgato%20Key%20Light%20ABC1._elg._tcp.local./state
```

### Multiple Properties

Set multiple properties at once:
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true, "brightness": 80, "temperature": 3200}' \
  http://localhost:9123/api/v1/lights/Elgato%20Key%20Light%20ABC1._elg._tcp.local./state
```

## Response Formats

### Success Response

Successful state changes return:
```json
{
  "status": "ok"
}
```

### Error Responses

Error responses include appropriate HTTP status codes with JSON error messages:

```json
{
  "error": "Light not found"
}
```

Common status codes:
- `200` - Success
- `400` - Bad Request (invalid parameters)
- `401` - Unauthorized (invalid API key)
- `404` - Light not found
- `500` - Internal Server Error

## Light Properties

### Controllable Properties
- **on**: Power state (boolean)
- **brightness**: Brightness level (integer 0-100)
- **temperature**: Color temperature in Kelvin (integer 2900-7000)

### Read-only Properties
- **id**: Unique light identifier
- **name**: Human-readable name
- **ip**: IP address of the light
- **port**: Port number (usually 9123)
- **productname**: Product name from the device
- **hardwareboardtype**: Hardware board type
- **firmwareversion**: Firmware version
- **firmwarebuild**: Firmware build number
- **serialnumber**: Device serial number
- **lastseen**: Timestamp when the light was last seen

## URL Encoding

Light IDs often contain special characters and should be URL-encoded when used in URLs:

```bash
# Original ID: Elgato Key Light ABC1._elg._tcp.local.
# URL-encoded: Elgato%20Key%20Light%20ABC1._elg._tcp.local.

curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:9123/api/v1/lights/Elgato%20Key%20Light%20ABC1._elg._tcp.local.
```