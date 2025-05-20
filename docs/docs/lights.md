# Controlling Lights

This guide explains how to control individual Elgato Key Light devices using KeylightD.

## Light Properties

Each Key Light has the following controllable properties:

- **Power State**: Turn the light on or off
- **Brightness**: Adjust brightness from 0% (off) to 100% (maximum brightness)
- **Color Temperature**: Set the color temperature from 2900K (warm/yellow) to 7000K (cool/blue)

## Discovering Lights

KeylightD automatically discovers Elgato Key Light devices on your network using mDNS (Bonjour). You can view all discovered lights using the CLI:

```bash
keylightctl light list
```

This will show all discovered lights with their IDs, names, IP addresses, and current state.

## Controlling Lights via CLI

The `keylightctl` command provides a simple interface for controlling lights.

### Getting Light Status

To view the status of a specific light:

```bash
keylightctl light get --id light-1
```

### Turning Lights On/Off

To turn a light on:

```bash
keylightctl light set --id light-1 --on
```

To turn a light off:

```bash
keylightctl light set --id light-1 --off
```

### Adjusting Brightness

Set the brightness (0-100):

```bash
keylightctl light set --id light-1 --brightness 75
```

### Changing Color Temperature

Set the color temperature in Kelvin (2900-7000):

```bash
keylightctl light set --id light-1 --temperature 4500
```

### Multiple Settings at Once

You can combine multiple settings in a single command:

```bash
keylightctl light set --id light-1 --on --brightness 80 --temperature 3200
```

## Controlling Lights via API

You can also control lights using the REST API directly.

### List All Lights

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/lights
```

### Get a Specific Light

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/lights/light-1
```

### Update Light State

```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"on": true, "brightness": 80, "temperature": 3200}' \
  http://localhost:8080/api/v1/lights/light-1/state
```

You can include any combination of the `on`, `brightness`, and `temperature` properties in the request body.

## Light States and Device Information

When you retrieve a light's information, you'll receive detailed data about the device:

```json
{
  "id": "light-1",
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

## Handling Multiple Lights

To apply the same settings to multiple lights, consider creating a [group](groups.md).

## Light Discovery and Monitoring

KeylightD continuously monitors the network for new devices and tracks the online status of discovered lights. If a light becomes unavailable, it will be marked as such but kept in the list of known devices.

The `lastseen` timestamp indicates when the light was last seen online.

## Troubleshooting

### Light Not Responding

If a light isn't responding to commands:

1. Check if the light is powered on and connected to your network
2. Verify that the light is discoverable using `keylightctl light list`
3. Try rebooting the light by turning it off and on again
4. Check if the light is accessible via its web interface (http://LIGHT_IP:9123)

### Wrong Settings Applied

If the wrong settings seem to be applied:

1. Check the current state using `keylightctl light get`
2. Ensure you're using the correct light ID
3. Try setting explicit values for all properties (on/off, brightness, temperature)

### Light Not Discovered

If KeylightD isn't discovering your light:

1. Ensure the light is on the same network as your KeylightD instance
2. Check if your network allows mDNS/Bonjour traffic
3. Verify the light is functioning correctly using the Elgato Control Center app
4. Try restarting KeylightD with debug logging: `keylightd --log-level debug`

## Next Steps

- [Create and manage groups](groups.md) to control multiple lights together
- [Explore the API reference](api/index.md) for detailed API documentation