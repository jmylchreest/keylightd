# Keylight Package: API Integration Details

## Elgato Key Light API Endpoints

The keylight package interacts with Elgato Key Light devices using their documented HTTP API:

- **Base URL:** `http://<ip>:9123/elgato`

### Endpoints Used

| Endpoint                | Method | Purpose                                 |
|------------------------|--------|-----------------------------------------|
| `/accessory-info`       | GET    | Get device info (model, serial, etc)    |
| `/lights`               | GET    | Get current light state                 |
| `/lights`               | PUT    | Set light state (on/off, brightness, temp) |

#### Example: `/lights` GET Response
```json
{
  "numberOfLights": 1,
  "lights": [
    {
      "on": 1,
      "brightness": 50,
      "temperature": 143
    }
  ]
}
```

#### Example: `/lights` PUT Request
```json
{
  "numberOfLights": 1,
  "lights": [
    {
      "on": 1,
      "brightness": 50,
      "temperature": 143
    }
  ]
}
```

- `on`: 0 (off) or 1 (on)
- `brightness`: 3–100
- `temperature`: 143–344 (7000K–2900K, lower = cooler)

#### Example: `/accessory-info` GET Response
```json
{
  "productName": "Elgato Key Light",
  "hardwareBoardType": 53,
  "firmwareBuildNumber": 194,
  "firmwareVersion": "1.0.3",
  "serialNumber": "ABC123456",
  "displayName": "Office Key Light",
  "features": ["lights"]
}
```

---

## How the `keylight` Package Works

### 1. **Discovery**
- Uses mDNS to find devices advertising `_elg._tcp`.
- For each discovered device, creates a `Light` struct with its IP and port.
- Fetches accessory info and initial state via HTTP.

### 2. **State Management**
- Maintains a map of discovered lights (`map[string]Light`).
- Each light has a `KeyLightClient` for HTTP communication.
- State changes (on/off, brightness, temperature) are sent via HTTP PUT to `/lights`.
- State is refreshed from the device via HTTP GET to `/lights`.

### 3. **Go Type Mapping**

| Go Type/Field         | API Field           | Notes                                  |
|----------------------|---------------------|----------------------------------------|
| `Light.ID`           | mDNS Name           | Unique per device                      |
| `Light.IP`           | mDNS AddrV4         | `net.IP` type                          |
| `Light.Port`         | mDNS Port           |                                        |
| `Light.ProductName`  | `productName`       | From `/accessory-info`                 |
| `Light.SerialNumber` | `serialNumber`      | From `/accessory-info`                 |
| `Light.State`        | `lights[0]`         | From `/lights`                         |
| `Light.On`           | `on`                | 0/1 in API, bool in Go                 |
| `Light.Brightness`   | `brightness`        | 3–100                                  |
| `Light.Temperature`  | `temperature`       | 143–344 (device units, see below)      |

#### Temperature Conversion
- The API uses device units (143–344). The Go code provides helpers to convert to/from Kelvin (2900–7000K).

---

## References
- [Elgato Key Light API Documentation](https://www.postman.com/apihandyman/hacking-elgato-key-light/documentation/fun9qrm/elgato-key-light)

---

For further details, see the code in `pkg/keylight/client.go`, `pkg/keylight/manager.go`, and `pkg/keylight/types.go`. 