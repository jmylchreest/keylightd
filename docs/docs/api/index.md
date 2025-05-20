# API Reference

This section provides detailed information about the KeylightD API endpoints, request/response formats, and authentication mechanisms.

## OpenAPI Specification

The KeylightD API follows the OpenAPI 3.1.0 specification. You can explore the interactive documentation below or [download the OpenAPI specification](../../openapi.yaml) for use with other tools.

<div id="swagger-ui"></div>

<script>
  window.onload = function() {
    const ui = SwaggerUIBundle({
      url: "../../openapi.yaml",
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

## Authentication

The API uses API keys for authentication. All API requests must include a valid API key using one of these methods:

- HTTP Bearer Authentication (preferred): `Authorization: Bearer YOUR_API_KEY`
- Custom Header: `X-API-Key: YOUR_API_KEY`

For more details on API key management, see the [Authentication](../authentication.md) guide.

## Response Format

All API responses are returned in JSON format. Successful responses typically include a `200 OK` status code along with the requested data.

Error responses include an appropriate HTTP status code (4xx for client errors, 5xx for server errors) and a JSON body with an error message.

Example error response:

```json
{
  "error": "Light not found"
}
```

## Rate Limiting

The API implements rate limiting to prevent abuse. If you exceed the rate limit, you'll receive a `429 Too Many Requests` response. The response will include a `Retry-After` header indicating how long to wait before retrying.

## API Versioning

The current API version is v1, which is reflected in the base path: `/api/v1`. Future versions will use a different version identifier in the path.