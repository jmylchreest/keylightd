---
sidebar_position: 1
---

# Supported Devices

keylightd currently supports the following devices:

## Elgato Key Light Series

- **Elgato Key Light** - The original professional lighting solution for content creators
- **Elgato Key Light MK.2** - Updated Key Light with 5 GHz Wi-Fi and Bluetooth support
- **Elgato Key Light Air** - Compact wireless version of the Key Light

## Elgato Ring Light

- **Elgato Ring Light** - Ring-format light that exposes the same HTTP API as the Key Light series

These devices are automatically discovered on your network using mDNS/Bonjour and can be controlled through keylightd's CLI, HTTP API, or Unix socket interface.

## Device-Specific Information

For detailed technical information about supported devices, including API endpoints, data formats, and implementation notes:

- [Elgato Key Light Series](elgato.md) - Technical details and API specifications

## Experimental devices

{/* BEGIN GENERATED EXPERIMENTAL DEVICES */}
- **Elgato Key Light Mini** (board 202)
- **Elgato Key Light Neo** (board 210)
{/* END GENERATED EXPERIMENTAL DEVICES */}

These models have passed simulated tests. Reports from owners using real hardware
help us verify compatibility. If you have one, try discovery, power, brightness,
and colour temperature, and [open an issue](https://github.com/jmylchreest/keylightd/issues/new)
with what worked or failed. Include your model, firmware version, keylightd version,
and platform. If you can see battery or power information, include whether it
looks right.

You can also just use keylightd with any listed device. Everyday use is welcome,
and reporting your results is optional.

## Request support for another device

If your light is missing from the list, [open a device support request](https://github.com/jmylchreest/keylightd/issues/new).
Include the manufacturer and exact model, and link any public protocol documentation
you know of.
