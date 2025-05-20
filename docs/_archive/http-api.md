# keylightd HTTP API

The `keylightd` daemon exposes an HTTP/S RESTful API for remote control and management of Elgato Keylights and the daemon itself.

## Base URL

All API endpoints are prefixed with `/api/v1`. The default listen address for the HTTP server is `http://localhost:9123`.

## Authentication

All requests to the HTTP API (except potentially a future status/health check endpoint) require authentication via an API key. The API key must be provided in one of the following ways:

1.  **Bearer Token in `Authorization` Header:**
    ```
    Authorization: Bearer <your_api_key>
    ```
2.  **`X-API-Key` Header:**
    ```
    X-API-Key: <your_api_key>
    ```

If an API key is missing, invalid, disabled, or expired, the server will respond with an `HTTP 401 Unauthorized` status.

API keys are managed via `keylightctl api-key ...` commands or the API endpoints themselves (see below).

## Common Responses

-   `200 OK`: The request was successful (typically for GET or PUT operations that return data).
-   `201 Created`: The resource was successfully created (typically for POST operations). The response body usually contains the created resource, and a `Location` header might be present.
-   `204 No Content`: The request was successful, but there is no response body to return (typically for DELETE operations or PUT operations that don't return data).
-   `400 Bad Request`: The request was malformed, such as invalid JSON or missing required parameters. The response body will usually contain an error message.
-   `401 Unauthorized`: Authentication failed (missing, invalid, or insufficient API key).
-   `404 Not Found`: The requested resource does not exist.
-   `405 Method Not Allowed`: The HTTP method used is not supported for the requested resource.
-   `500 Internal ServerError`: An unexpected error occurred on the server.

Error responses will typically be in JSON format:
```json
{
    "error": "A descriptive error message"
}
```

## API Endpoints

### 1. API Keys

These endpoints manage API authentication keys.

#### 1.1 Create API Key

*   **Endpoint:** `POST /api/v1/apikeys`
*   **Description:** Creates a new API key. The full key is only returned on creation.
*   **Request Body:**
    ```json
    {
        "name": "My New Web Key",
        "expires_in_seconds": 0 
    }
    ```
    -   `name` (string, required): A user-friendly name for the key.
    -   `expires_in_seconds` (integer, optional): Duration in seconds until the key expires. If `0` or not provided, the key never expires.
*   **Response:** `201 Created`
    ```json
    {
        "key": "generated_api_key_string_long_and_random",
        "name": "My New Web Key",
        "created_at": "2023-10-27T10:00:00Z",
        "expires_at": "0001-01-01T00:00:00Z", // Will be actual expiry if set
        "disabled": false,
        "id": "generated_api_key_string_long_and_random"
    }
    ```
*   **Example:**
    ```bash
    curl -X POST -H "Authorization: Bearer <your_admin_key>" -H "Content-Type: application/json" \\
         -d '{"name": "My Web App Key", "expires_in_seconds": 86400}' \\
         http://localhost:9123/api/v1/apikeys
    ```

#### 1.2 List API Keys

*   **Endpoint:** `GET /api/v1/apikeys`
*   **Description:** Retrieves a list of all API keys. The full API key string is **not** included for security. The `id` field will contain the key string.
*   **Response:** `200 OK`
    ```json
    [
        {
            "id": "some_api_key_string",
            "name": "Key For Script A",
            "created_at": "2023-10-26T12:00:00Z",
            "expires_at": "2024-10-25T12:00:00Z",
            "last_used_at": "2023-10-27T09:15:00Z",
            "disabled": false
        },
        {
            "id": "another_api_key_string",
            "name": "Never Expiring Key",
            "created_at": "2023-01-01T00:00:00Z",
            "expires_at": "0001-01-01T00:00:00Z", // Indicates never expires
            "last_used_at": "0001-01-01T00:00:00Z", // Indicates never used
            "disabled": false
        }
    ]
    ```
*   **Example:**
    ```bash
    curl -X GET -H "Authorization: Bearer <your_admin_key>" http://localhost:9123/api/v1/apikeys
    ```

#### 1.3 Delete API Key

*   **Endpoint:** `DELETE /api/v1/apikeys/{key}`
*   **Description:** Deletes a specific API key.
*   **Path Parameters:**
    -   `key` (string, required): The API key string to delete.
*   **Response:** `204 No Content`
*   **Example:**
    ```bash
    curl -X DELETE -H "Authorization: Bearer <your_admin_key>" http://localhost:9123/api/v1/apikeys/key_string_to_delete
    ```

### 2. Lights

These endpoints manage individual Elgato Keylights.

#### 2.1 List Discovered Lights

*   **Endpoint:** `GET /api/v1/lights`
*   **Description:** Retrieves a list of all currently discovered Keylights and their state. The structure of each light object matches `keylight.Light`.
*   **Response:** `200 OK`
    ```json
    [
        {
            "id": "elgato-key-light-AB12", // Typically MAC address or unique identifier
            "name": "Key Light Studio",     // User-assigned name if available, otherwise product name
            "productname": "Elgato Key Light",
            "serialnumber": "SG12X3Y45678",
            "firmwareversion": "1.0.3",
            "firmwarebuild": 194,
            "ip": "192.168.1.101",
            "port": 9123,
            "on": true,
            "brightness": 50,
            "temperature": 4500, // In Kelvin
            "lastseen": "2023-10-27T10:30:00Z"
        }
        // ... other lights
    ]
    ```
*   **Example:**
    ```bash
    curl -X GET -H "X-API-Key: <your_api_key>" http://localhost:9123/api/v1/lights
    ```

#### 2.2 Get Light Details

*   **Endpoint:** `GET /api/v1/lights/{id}`
*   **Description:** Retrieves the current state and details of a specific Keylight.
*   **Path Parameters:**
    -   `id` (string, required): The ID of the light (e.g., `elgato-key-light-AB12`).
*   **Response:** `200 OK` (Same structure as one item in the list lights response)
*   **Example:**
    ```bash
    curl -X GET -H "X-API-Key: <your_api_key>" http://localhost:9123/api/v1/lights/elgato-key-light-AB12
    ```

#### 2.3 Set Light State

*   **Endpoint:** `PUT /api/v1/lights/{id}/state`
*   **Description:** Updates the state (on/off, brightness, temperature) of a specific Keylight. You can provide one or more properties to update.
*   **Path Parameters:**
    -   `id` (string, required): The ID of the light.
*   **Request Body:**
    ```json
    {
        "on": false,
        "brightness": 20,
        "temperature": 3200 
    }
    ```
    -   `on` (boolean, optional): `true` to turn on, `false` to turn off.
    -   `brightness` (integer, optional): Brightness percentage (0-100).
    -   `temperature` (integer, optional): Color temperature in Kelvin (typically 2900-7000).
*   **Response:** `200 OK`
    ```json
    {
        "status": "ok"
    }
    ```
*   **Example:**
    ```bash
    curl -X PUT -H "X-API-Key: <your_api_key>" -H "Content-Type: application/json" \\
         -d '{"on": true, "brightness": 75}' \\
         http://localhost:9123/api/v1/lights/elgato-key-light-AB12/state
    ```

### 3. Groups

These endpoints manage groups of Keylights.

#### 3.1 List Groups

*   **Endpoint:** `GET /api/v1/groups`
*   **Description:** Retrieves a list of all configured light groups.
*   **Response:** `200 OK`
    ```json
    [
        {
            "id": "group-1678886400000000000",
            "name": "My Main Setup",
            "lights": ["elgato-key-light-AB12", "elgato-key-light-CD34"]
        }
        // ... other groups
    ]
    ```
*   **Example:**
    ```bash
    curl -X GET -H "X-API-Key: <your_api_key>" http://localhost:9123/api/v1/groups
    ```

#### 3.2 Create Group

*   **Endpoint:** `POST /api/v1/groups`
*   **Description:** Creates a new light group.
*   **Request Body:**
    ```json
    {
        "name": "Streaming Background",
        "light_ids": ["elgato-key-light-EF56"]
    }
    ```
    -   `name` (string, required): The name for the new group.
    -   `light_ids` (array of strings, optional): A list of light IDs to initially include in the group.
*   **Response:** `201 Created` (Returns the newly created group object, similar to the Get Group response)
    ```json
    {
        "id": "group-1678886400000000001", // Generated ID
        "name": "Streaming Background",
        "lights": ["elgato-key-light-EF56"]
    }
    ```
*   **Example:**
    ```bash
    curl -X POST -H "X-API-Key: <your_api_key>" -H "Content-Type: application/json" \\
         -d '{"name": "Video Call Lights", "light_ids": ["elgato-key-light-AB12", "elgato-key-light-CD34"]}' \\
         http://localhost:9123/api/v1/groups
    ```

#### 3.3 Get Group Details

*   **Endpoint:** `GET /api/v1/groups/{id}`
*   **Description:** Retrieves details of a specific light group.
*   **Path Parameters:**
    -   `id` (string, required): The ID of the group.
*   **Response:** `200 OK` (Same structure as one item in the list groups response)
*   **Example:**
    ```bash
    curl -X GET -H "X-API-Key: <your_api_key>" http://localhost:9123/api/v1/groups/group-1678886400000000000
    ```

#### 3.4 Delete Group

*   **Endpoint:** `DELETE /api/v1/groups/{id}`
*   **Description:** Deletes a specific light group.
*   **Path Parameters:**
    -   `id` (string, required): The ID of the group to delete.
*   **Response:** `204 No Content`
*   **Example:**
    ```bash
    curl -X DELETE -H "X-API-Key: <your_api_key>" http://localhost:9123/api/v1/groups/group-1678886400000000000
    ```

#### 3.5 Set Lights in Group

*   **Endpoint:** `PUT /api/v1/groups/{id}/lights`
*   **Description:** Updates the list of lights belonging to a specific group. This replaces the entire list of lights for the group.
*   **Path Parameters:**
    -   `id` (string, required): The ID of the group.
*   **Request Body:**
    ```json
    {
        "light_ids": ["elgato-key-light-AB12", "elgato-key-light-XY78"]
    }
    ```
    -   `light_ids` (array of strings, required): The new list of light IDs for the group.
*   **Response:** `200 OK`
    ```json
    {
        "status": "ok"
    }
    ```
*   **Example:**
    ```bash
    curl -X PUT -H "X-API-Key: <your_api_key>" -H "Content-Type: application/json" \\
         -d '{"light_ids": ["elgato-key-light-AB12"]}' \\
         http://localhost:9123/api/v1/groups/group-1678886400000000000/lights
    ```

#### 3.6 Set Group State

*   **Endpoint:** `PUT /api/v1/groups/{id}/state`
*   **Description:** Updates the state (on/off, brightness, temperature) for all lights within one or more groups. The `{id}` path parameter can be:
    - A group ID (e.g., `group-1678886400000000000`)
    - A group name (e.g., `office`)
    - A comma-separated list of group IDs and/or names (e.g., `group-1,office,group-2`)
    - If a name matches multiple groups, all are updated.
*   **Path Parameters:**
    -   `id` (string, required): The group ID, group name, or comma-separated list of IDs/names.
*   **Request Body:** (Same structure as Set Light State)
    ```json
    {
        "on": true,
        "brightness": 60 
    }
    ```
    -   `on` (boolean, optional): `true` to turn on, `false` to turn off.
    -   `brightness` (integer, optional): Brightness percentage (0-100).
    -   `temperature` (integer, optional): Color temperature in Kelvin (typically 2900-7000).
*   **Response:**
    -   `200 OK` if all groups were updated successfully.
    -   `207 Multi-Status` if some groups failed, with a JSON body listing errors.
    ```json
    {
        "status": "partial",
        "errors": [
            "group group-1: error message",
            "group group-2: error message"
        ]
    }
    ```
*   **Example:**
    ```bash
    # Update a single group by ID
    curl -X PUT -H "X-API-Key: <your_api_key>" -H "Content-Type: application/json" \
         -d '{"on": true, "brightness": 30}' \
         http://localhost:9123/api/v1/groups/group-1678886400000000000/state

    # Update all groups named "office"
    curl -X PUT -H "X-API-Key: <your_api_key>" -H "Content-Type: application/json" \
         -d '{"on": false}' \
         http://localhost:9123/api/v1/groups/office/state

    # Update multiple groups by ID and name
    curl -X PUT -H "X-API-Key: <your_api_key>" -H "Content-Type: application/json" \
         -d '{"brightness": 50}' \
         http://localhost:9123/api/v1/groups/group-1,office,group-2/state
    ```

// Note: The socket API wraps light and group objects in a 'light' or 'group' field, but the HTTP API returns flat objects. The CLI client automatically unwraps these for consistency.
```json
{
    "productname": "Elgato Key Light",
    "serialnumber": "ABC123456",
    "firmwareversion": "1.0.3",
    "firmwarebuild": 194,
    "on": true,
    "brightness": 50,
    "temperature": 5000,
    "ip": "192.168.1.100",
    "port": 9123,
    "lastseen": "2024-03-20T10:00:00Z"
}
```
```json
{
    "id": "group-1",
    "name": "Office Lights",
    "lights": ["light-id-1", "light-id-2"]
}
```
```json
{
    "id": "group-1",
    "name": "Office Lights",
    "lights": []
} 