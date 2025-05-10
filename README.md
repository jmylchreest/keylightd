# Keylightd

Keylightd is a daemon service that provides a RESTful API for managing Elgato Keylights. It supports automatic discovery of Keylights via mDNS, grouping of lights, and management of API keys.

## Features

- Automatic discovery of Elgato Keylights via mDNS
- RESTful API for controlling lights
- Unix socket for local administration
- API key authentication
- Light grouping
- Configurable discovery interval
- Debug logging

## Building

### Prerequisites

- Go 1.24 or later
- Make
- Git

### Build Steps

1. Clone the repository:
   ```bash
   git clone https://github.com/jmylchreest/keylightd.git
   cd keylightd
   ```

2. Build the application:
   ```bash
   make build
   ```

The binary will be created in the `bin` directory.

## Running

### Command Line Options

- `-v`: Increase verbosity level (can be specified multiple times)

### Example

```bash
./bin/keylightd -v
```

## Docker

You can also run keylightd using Docker:

```bash
docker build -t keylightd .
docker run -d --name keylightd keylightd
```

## Development

### Installing Development Tools

```bash
make tools
```

### Running Tests

```bash
make test
```

### Running Linters

```bash
make lint
```

## License

This project is licensed under the MIT License - see the LICENSE file for details. 