# Authentication

This guide explains how to authenticate with the KeylightD API using API keys.

## API Key Authentication

The KeylightD API uses API keys for authentication. All requests to the API must include a valid API key using one of these methods:

### Method 1: HTTP Bearer Authentication (Recommended)

Include the API key in the `Authorization` header with the `Bearer` prefix:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" http://localhost:8080/api/v1/lights
```

### Method 2: Custom Header

Alternatively, you can use the custom `X-API-Key` header:

```bash
curl -H "X-API-Key: YOUR_API_KEY" http://localhost:8080/api/v1/lights
```

## Managing API Keys

KeylightD provides commands for managing API keys through the `keylightctl` command-line interface.

### Creating API Keys

To create a new API key:

```bash
keylightctl api-key add my-key-name
```

You can also set an expiration time:

```bash
# Create a key that expires in 30 days
keylightctl api-key add my-key-name 30d

# Create a key that expires in 24 hours
keylightctl api-key add my-key-name 24h
```

When you create a key, the full key value will be displayed **only once**. Make sure to save it securely.

### Listing API Keys

To view all API keys:

```bash
keylightctl api-key list
```

This will show all keys with their names, creation dates, expiration dates, and disabled status, but it will not show the full key values for security reasons.

### Disabling and Enabling API Keys

To disable an API key temporarily:

```bash
keylightctl api-key set-enabled YOUR_KEY_OR_NAME false
```

To re-enable a disabled key:

```bash
keylightctl api-key set-enabled YOUR_KEY_OR_NAME true
```

### Deleting API Keys

To permanently delete an API key:

```bash
keylightctl api-key delete YOUR_KEY
```

If you don't specify a key, you'll be shown a list of keys to choose from.

## API Key Management via the API

You can also manage API keys using the API itself. This is useful for integrations and automation.

### Creating a Key via API

```bash
curl -X POST \
  -H "Authorization: Bearer EXISTING_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "new-api-key", "expires_in": "720h"}' \
  http://localhost:8080/api/v1/apikeys
```

### Listing Keys via API

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/apikeys
```

### Disabling a Key via API

```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"disabled": true}' \
  http://localhost:8080/api/v1/apikeys/KEY_TO_DISABLE/disabled
```

### Deleting a Key via API

```bash
curl -X DELETE \
  -H "Authorization: Bearer YOUR_API_KEY" \
  http://localhost:8080/api/v1/apikeys/KEY_TO_DELETE
```

## Security Considerations

- Store API keys securely and treat them as sensitive credentials
- Use HTTPS if exposing the API to the internet
- Create separate keys for different applications or users
- Set expiration times for keys used for temporary access
- Regularly audit and rotate API keys
- If a key is compromised, delete it immediately and create a new one

## API Key Structure

API keys are random strings generated using a cryptographically secure random number generator. They cannot be recovered if lost - you'll need to create a new key.

## Troubleshooting

### Unauthorized Error

If you receive a `401 Unauthorized` error, check:

- That you're using the correct API key
- The key hasn't expired
- The key isn't disabled
- You're formatting the header correctly (check for typos)

### API Key Not Working

If your key suddenly stops working:

1. List all keys to check if it's been disabled or expired
2. Try creating a new key and use that instead
3. Check the KeylightD logs for any authentication errors

For persistent issues, check the server logs or run KeylightD with debug logging enabled:

```bash
keylightd --log-level debug
```