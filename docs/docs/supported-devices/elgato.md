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
to connect the CLI to the daemon. Use a loopback URL or HTTPS when supplying
Wi-Fi credentials. Put the light in pairing mode and select its Bluetooth
address on the machine running keylightd. On Linux, the Bluetooth adapter
must have discovered the peripheral in BlueZ. On macOS, supply the
CoreBluetooth peripheral UUID and grant Bluetooth permission.

Start setup with the selected Bluetooth peripheral. The daemon flashes the light
and waits for confirmation before installing certificates or sending Wi-Fi
credentials:

```sh
keylightctl light setup start nearby --peripheral "<address>" \
  --ssid "<network>" --password-stdin
keylightctl light setup status nearby
keylightctl light setup confirm nearby "<confirmationId>"
keylightctl light setup status nearby
```

If you missed the flash, run `keylightctl light setup blink nearby "<confirmationId>"`
or choose **Blink again** in GNOME. Confirm only if the intended light flashed.
Once it joins Wi-Fi, the daemon uses
the IPv4 address reported over BLE to find its mDNS advertisement. It verifies
the light with mutual TLS and saves its credentials under the discovered device ID,
which the session reports as `configuredId`. This also works when the light has
no mDNS entry before setup. Use `cancel nearby` to cancel or
`forget <configuredId>` to remove saved credentials before re-pairing.

In GNOME Preferences, choose **Lights → Add light via Bluetooth (BLE)**. Enter the Bluetooth
address manually, provide Wi-Fi settings and confirm that the intended light
flashed. GNOME supports WPA2 Personal (AES) and open networks. Use a loopback API
URL or HTTPS when supplying credentials.

Linux uses BlueZ with `CGO_ENABLED=0`. macOS uses CoreBluetooth and the native
macOS toolchain with CGO enabled.


In the tray, use **Settings → Add light via Bluetooth (BLE)**. A light that
already appears with **Setup required** has a setup button in place of its
power button. GNOME Preferences also has a setup button on the light’s row.
When you start from a row, keylightd checks that the selected Bluetooth
peripheral reports the same network identity as that light. Enter the Bluetooth address manually, confirm the blink, and then enter the
Wi-Fi settings. The tray can use either
the local socket or its configured HTTP connection.
