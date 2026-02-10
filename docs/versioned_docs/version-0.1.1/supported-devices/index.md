---
sidebar_position: 1
---

# Supported Devices

keylightd currently supports the following devices:

## Elgato Key Light Series

- **Elgato Key Light** - The original professional lighting solution for content creators
- **Elgato Key Light Air** - Compact wireless version of the Key Light

These devices are automatically discovered on your network using mDNS/Bonjour and can be controlled through keylightd's CLI, HTTP API, or Unix socket interface.

## Device-Specific Information

For detailed technical information about supported devices, including API endpoints, data formats, and implementation notes:

- [Elgato Key Light Series](elgato.md) - Technical details and API specifications

## Adding Support for New Devices

If you have a similar HTTP-based lighting device that you'd like to see supported, please [open an issue](https://github.com/jmylchreest/keylightd/issues) with:

- Device model and manufacturer
- Network discovery method (mDNS service type, if applicable)
- Available API endpoints and documentation
- Sample API requests and responses

We welcome contributions to expand device compatibility!