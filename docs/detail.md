# Keylightd Details

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

When setting the temperature through the API:
1. The input Kelvin value is clamped to the valid range (2900K-7000K)
2. The clamped Kelvin value is converted to mireds using the formula above
3. The mireds value is clamped to the device's valid range (143-344)
4. The resulting mireds value is sent to the device

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