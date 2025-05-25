# keylightd Documentation

Welcome to the keylightd API documentation. This guide provides comprehensive information about the keylightd API, which allows you to control Elgato Key Light devices programmatically.

## What is keylightd?

keylightd is a daemon service that discovers, monitors, and controls Elgato Key Light devices on your network. It provides:

- A Unix socket interface for local control
- A REST API for remote control
- Group management for controlling multiple lights together
- API key authentication for secure access

## Key Features

- **Light Control**: Turn lights on/off, adjust brightness and color temperature
- **Group Management**: Create and manage groups of lights
- **Discovery**: Automatically find Key Light devices on your network
- **Authentication**: Secure API with key-based authentication
- **Unix Socket**: Local control without network overhead

## Getting Started

If you're new to keylightd, start with the [Getting Started](getting-started.md) guide to learn how to install and configure the daemon.

## API Reference

For detailed information about available endpoints, request/response formats, and authentication, see the [API Reference](api/index.md).

## Example Use Cases

- Control lighting for video conferencing
- Create scene presets for different recording environments
- Integrate with home automation systems
- Build custom control interfaces
- Script lighting changes for specific events

## Quick Links

- [Lights](lights/cli.md) - Controlling individual lights
- [Groups](groups/cli.md) - Managing groups of lights

## Support

For issues, feature requests, or contributions, please visit the [GitHub repository](https://github.com/jmylchreest/keylightd).