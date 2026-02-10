---
sidebar_position: 1
---

# keylightd

**keylightd** is a lightweight daemon that automatically discovers and controls [Elgato Key Light](https://www.elgato.com/key-light) devices on your network via mDNS. It exposes a CLI, HTTP API, and Unix socket so you can manage your lights from scripts, desktop apps, or anything that speaks HTTP or JSON.

## Quick Start

### 1. Install

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
  <TabItem value="homebrew" label="Homebrew" default>

```bash
brew tap jmylchreest/keylightd && brew install keylightd
```

  </TabItem>
  <TabItem value="aur" label="Arch (AUR)">

```bash
paru -S keylightd-bin
```

  </TabItem>
  <TabItem value="binary" label="Binary Release">

Download the latest release from [GitHub Releases](https://github.com/jmylchreest/keylightd/releases).

  </TabItem>
</Tabs>

### 2. Start the daemon

```bash
keylightd
```

The daemon will begin discovering Key Lights on your local network automatically.

### 3. Create an API key

This is required for HTTP API access. The CLI and Unix socket don't need keys.

```bash
keylightctl api-key add my-key
```

### 4. Control a light

```bash
# List discovered lights
keylightctl light list

# Set brightness to 80%
keylightctl light set <LIGHT_ID> brightness 80
```

See the [Getting Started](./getting-started) guide for the full setup walkthrough.

## Ways to Control Your Lights

keylightd gives you several interfaces — pick whichever fits your workflow.

| Interface | Description | Docs |
|-----------|-------------|------|
| **CLI** (`keylightctl`) | Command-line tool for scripting and quick adjustments | [Lights CLI](./lights/cli) · [Groups CLI](./groups/cli) |
| **HTTP REST API** | Remote control on port `9123` with Bearer token auth and an OpenAPI spec | [Lights HTTP](./lights/http) · [Groups HTTP](./groups/http) · [API Reference](./api/rest/keylightd-api) |
| **Unix Socket** | Low-latency local control via `$XDG_RUNTIME_DIR/keylightd.sock` | [Lights Socket](./lights/socket) · [Groups Socket](./groups/socket) · [Socket Reference](./api/unix-socket) |
| **System Tray App** | Desktop GUI built with Wails for point-and-click control | [Tray App](./desktop-apps/tray) |
| **GNOME Extension** | Native GNOME Shell integration | [GNOME Extension](./desktop-apps/gnome-extension) |
| **Waybar Module** | Status bar widget for Wayland compositors | [Waybar](./desktop-apps/waybar) |

## Key Concepts

- **Auto-discovery** — The daemon finds Key Lights on your network via mDNS. No manual IP configuration needed.
- **Groups** — Organize lights into named groups to control multiple lights with a single command. See [Groups CLI](./groups/cli).
- **API Keys** — HTTP access is secured with Bearer token authentication. Generate keys with `keylightctl api-key add`.
- **WebSocket Events** — Subscribe to real-time state changes over the HTTP API for reactive integrations.

## Learn More

- **[Getting Started](./getting-started)** — Installation, first run, and initial configuration
- **[Lights CLI](./lights/cli)** — Full CLI reference for individual light control
- **[Groups CLI](./groups/cli)** — Create and manage light groups
- **[HTTP REST API Reference](./api/rest/keylightd-api)** — OpenAPI spec and endpoint documentation
- **[Unix Socket API Reference](./api/unix-socket)** — Protocol reference for local socket communication
- **[Desktop Apps](./desktop-apps/tray)** — Tray app, GNOME extension, and Waybar integration

---

Found an issue or want to contribute? Visit the [GitHub repository](https://github.com/jmylchreest/keylightd).
