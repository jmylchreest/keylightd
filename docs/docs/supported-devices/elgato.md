---
sidebar_position: 2
---

# Elgato Key Light Series

## Product registry

The registry recognizes the following models:

| Manufacturer | Model | Hardware board ID |
| --- | --- | --- |
| `elgato` | Elgato Key Light | 53 |
| `elgato` | Elgato Key Light Air | 200 |
| `elgato` | Elgato Key Light MK.2 | 205 |
| `elgato` | Elgato Ring Light | 201 |

Identifiers are scoped to the manufacturer. A registered board ID takes precedence
over the reported product name. If the board ID is absent or unrecognised, an exact
registered product name or alias is accepted, preserving existing discovery behaviour.
For example, board 205 resolves to Key Light MK.2 even if its product name is simply
`Elgato Key Light`.

Light responses retain the device's `productname` and add `manufacturer`, the canonical
`model`, and an `experimental` flag. Supported and experimental products use the same
registry structure and lookup. Key Light Mini (board 202) is experimental and tested with simulated responses. Battery telemetry is sampled during discovery and shown by `keylightctl light get` when available.
Key Light Neo (board 210) is experimental and tested with simulated responses. The API includes power information when the device reports it.

Discovery uses `_elg._tcp` over mDNS and the Elgato HTTP client.

## Temperature Control

The Elgato Key Light uses mireds (micro reciprocal degrees) for temperature control. Mireds are a unit of measurement used to express color temperature, calculated as:

```
mireds = 1,000,000 / Kelvin
```

The device accepts values in the range:
- 344 mireds (2900K, warm) to 143 mireds (7000K, cool)

### Temperature Conversion Table

| Kelvin | Mireds | Description |
|--------|--------|-------------|
| 2900K  | 344    | Warm (incandescent) |
| 3900K  | 256    | Warm white |
| 4950K  | 202    | Neutral |
| 5950K  | 168    | Cool white |
| 7000K  | 143    | Cool (daylight) |

When setting the temperature through the keylightd API:
1. The input Kelvin value is clamped to the valid range (2900K-7000K)
2. The clamped Kelvin value is converted to mireds using the formula above
3. The resulting mireds value is sent to the device

For example:
- Setting 2000K → clamped to 2900K → 1,000,000/2900 = 344 mireds
- Setting 8000K → clamped to 7000K → 1,000,000/7000 = 143 mireds

## Brightness Control

The brightness is controlled as a percentage:
- Range: 0-100%
- Values below 0% are clamped to 0%
- Values above 100% are clamped to 100%

## Power Control

The power state is controlled as a boolean:
- `true` = On
- `false` = Off

## API Endpoints

The device exposes the following HTTP endpoints:

- `GET /elgato/accessory-info` - Get device information
- `GET /elgato/lights` - Get current light state
- `PUT /elgato/lights` - Set light state

### Accessory Info Format

```json
{
  "productName": "Elgato Key Light",
  "hardwareBoardType": 53,
  "hardwareRevision": "1",
  "macAddress": "XX:XX:XX:XX:XX:XX",
  "firmwareBuildNumber": 222,
  "firmwareVersion": "1.0.3",
  "serialNumber": "CNXXXXXXXXXX",
  "displayName": "Elgato Key Light XXXX",
  "features": ["lights"],
  "wifi-info": {
    "ssid": "YourNetwork",
    "frequencyMHz": 2400,
    "rssi": -41
  }
}
```

### Light State Format

```json
{
  "numberOfLights": 1,
  "lights": [
    {
      "on": 1,           // 1 = on, 0 = off
      "brightness": 60,  // 0-100
      "temperature": 344 // 143-344 mireds
    }
  ]
}
```

## Key Light Air MK.2

The Key Light Air MK.2 (board 214) is experimental. The client and discovery
flow are tested with simulated devices. Physical interoperability is unverified.

Configure an already provisioned light with its address, the CA that verifies
its server certificate, and a client certificate and private key trusted by
the light. If the certificate identifies the device by DNS name and you
connect by IP, supply that certificate name with `server_name`.

```yaml
config:
  air_mk2:
    - address: "192.0.2.10:9123"
      server_name: "light.example"
      ca_file: /path/to/light-ca.pem
      cert_file: /path/to/client-chain.pem
      key_file: /path/to/client-key.pem
```

Restart keylightd after configuring the device. Discovery matches credentials by
the advertised IP address and port. An unconfigured discovery reports
`needsConfiguration` with `credentials_missing`. The client reads and updates
power, brightness, and white colour temperature over a TLS WebSocket.

For daemon-managed setup, set `KEYLIGHTCTL_API_URL` and `KEYLIGHTCTL_API_KEY`
to connect the CLI to the daemon's HTTP API. The CLI supports
`keylightctl light setup start <id>`, `status <id>`, `cancel <id>`, and
`forget <id>`. The GNOME extension links lights that need setup to its
Preferences, where you can start setup, check status, or cancel.
Forgetting removes the daemon's identity. Put the light into pairing mode
before attempting to configure it again.

Starting setup requires a pairing backend on the host running keylightd.
The experimental build currently supports pre-provisioned lights through
the TLS configuration above.

The native pairing transport targets BlueZ on Linux and CoreBluetooth on
macOS. Linux can build the transport with `CGO_ENABLED=0`. The macOS transport
requires the native macOS toolchain and Bluetooth permission. The transport
is not yet enabled for setup. The daemon needs a verified pairing sequence
and a way to select the intended peripheral before it can provision a light.
