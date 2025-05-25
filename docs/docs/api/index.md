# HTTP REST API

This section provides detailed information about the keylightd API endpoints, request/response formats, and authentication mechanisms.

## Overview

The keylightd HTTP API provides comprehensive control over Elgato Key Light devices through standard REST endpoints. The API supports:

- Device discovery and status monitoring
- Individual light control (power, brightness, temperature)
- Group management for controlling multiple lights
- API key management for authentication
- Real-time device state synchronization

## Base URL

The API is available at:
```
http://localhost:9123/api/v1
```

The default port is 9123, but this can be configured in the keylightd configuration file.

## OpenAPI Specification

The keylightd API follows the OpenAPI 3.1.0 specification. You can explore the interactive documentation below or [download the OpenAPI specification](spec/openapi.yaml) for use with other tools.

<div id="swagger-ui"></div>

<script>
  window.onload = function() {
    const ui = SwaggerUIBundle({
      url: "spec/openapi.yaml",
      dom_id: '#swagger-ui',
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIBundle.SwaggerUIStandalonePreset
      ],
      layout: "BaseLayout",
      deepLinking: true,
      displayOperationId: false,
      defaultModelsExpandDepth: 1,
      defaultModelExpandDepth: 1,
      defaultModelRendering: 'model',
      docExpansion: 'list',
      showExtensions: false,
      showCommonExtensions: false
    });
    window.ui = ui;
  };
</script>
