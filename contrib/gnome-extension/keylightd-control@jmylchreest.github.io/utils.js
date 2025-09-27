"use strict";

import GLib from "gi://GLib";
import Soup from "gi://Soup";
import GObject from "gi://GObject";

// Global settings instance - set by the extension
let _settingsInstance = null;
let _erroring = true;

// Signal emitter helper for keylightd-state-update
const KeylightdStateUpdateEmitter = GObject.registerClass(
  {
    Signals: {
      "keylightd-state-update": {
        param_types: [],
      },
    },
  },
  class KeylightdStateUpdateEmitter extends GObject.Object {},
);

/**
 * Set the settings instance to be used by utility functions
 * @param {Gio.Settings} settings The extension settings instance
 */
export function setSettings(settings) {
  _settingsInstance = settings;
}

/**
 * Get the current erroring state
 * Returns true if the last GET request to groups or lights failed
 * @returns {boolean} The current erroring state
 */
export function getErroring() {
  return _erroring;
}

/**
 * Centralized logging function
 * @param {string} level - Log level (log, debug, info, warn, error)
 * @param {string} message - Message to log
 * @param {...any} args - Additional arguments to log
 */
export function filteredLog(level, message, ...args) {
  // Normalize level to lowercase
  level = level.toLowerCase();

  // Define log level hierarchy
  const LOG_LEVELS = Object.freeze({
    debug: 0,
    info: 1,
    warn: 2,
    warning: 2,
    error: 3,
  });

  // Always log errors regardless of settings
  if (level === "error") {
    console.error(
      `[keylightd-control (${level.toUpperCase()})] ${message}`,
      ...args,
    );
    return;
  }

  // For other levels, check if logging is enabled and level is high enough
  if (!_settingsInstance || !_settingsInstance.get_boolean("enable-logging")) {
    return;
  }

  // Get configured log level
  const configuredLevel = _settingsInstance
    .get_string("log-level")
    .toLowerCase();
  const configuredLevelValue =
    configuredLevel in LOG_LEVELS
      ? LOG_LEVELS[configuredLevel]
      : LOG_LEVELS["warning"];
  const messageLevelValue =
    level in LOG_LEVELS ? LOG_LEVELS[level] : LOG_LEVELS["info"];

  // Only log if message level is at or above configured level
  if (messageLevelValue >= configuredLevelValue) {
    const prefix = `[keylightd-control (${level.toUpperCase()})]`;
    const consoleMethod =
      level === "debug" && typeof console.debug === "function"
        ? "debug"
        : level === "info" && typeof console.info === "function"
          ? "info"
          : (level === "warn" || level === "warning") &&
              typeof console.warn === "function"
            ? "warn"
            : "log";
    console[consoleMethod](`${prefix} ${message}`, ...args);
  }
}

/**
 * Normalize lights data from API to ensure consistent format
 * @param {Object|Array} lightsData - Response from the lights API
 * @returns {Array} - Normalized array of light objects
 */
export function normalizeApiLights(lightsData) {
  if (!lightsData) {
    return [];
  }

  let lightsArray = [];

  if (Array.isArray(lightsData)) {
    lightsArray = lightsData;
  } else if (typeof lightsData === "object") {
    // Convert object of lights to array
    lightsArray = Object.entries(lightsData).map(([id, lightData]) => {
      return {
        id: id,
        name: lightData.name || id,
        ...lightData,
      };
    });
  }

  // Ensure all lights have required properties
  return lightsArray.map((light) => ({
    id: light.id || "",
    name: light.name || light.id || "Unknown Light",
    on: !!light.on,
    brightness: light.brightness || 50,
    temperature: light.temperature || 200,
    ...light,
  }));
}

/**
 * Check if any light in a collection is on
 * @param {Array} lights - Array of light objects
 * @returns {boolean} - True if any light is on
 */
export function isAnyLightOn(lights) {
  if (!lights || !Array.isArray(lights)) {
    return false;
  }

  return lights.some((light) => light.on === true);
}

/**
 * Helper function to make API requests to the keylightd API
 * This automatically uses the API URL and key from extension settings
 *
 * @param {string} path - API path to call
 * @param {string} method - HTTP method (GET, POST, PUT, DELETE)
 * @param {Object|null} body - Optional request body
 * @param {KeylightdStateUpdateEmitter|null} stateUpdateEmitter - Optional state update emitter
 * @returns {Promise<Object>} - Promise resolving to the API response
 * @throws {Error} If the API URL or key is not configured, or if the request fails
 */
export async function fetchAPI(
  path,
  method = "GET",
  body = null,
  stateUpdateEmitter = null,
) {
  const requestId = Math.random().toString(36).substring(2, 8);

  if (!_settingsInstance) {
    filteredLog("error", `[${requestId}] Settings have not been initialized`);
    throw new Error("Settings have not been initialized");
  }

  const endpoint = _settingsInstance.get_string("api-url");
  const apiKey = _settingsInstance.get_string("api-key");

  // Check if required settings are available
  if (!endpoint || !apiKey) {
    filteredLog("error", `[${requestId}] API URL or API Key not configured`);
    // Update erroring state for GET requests to groups or lights
    if (
      method === "GET" &&
      ["groups", "lights"].some(
        (base) => path === base || path.startsWith(`${base}/`),
      )
    ) {
      _erroring = true;
    }
    throw new Error("API URL or API Key not configured");
  }

  let url = `${endpoint}/api/v1/${path}`;
  let session = new Soup.Session();
  let message = Soup.Message.new(method, url);
  message.get_request_headers().append("X-API-Key", apiKey);

  // Log request details
  filteredLog(
    "info",
    `[${requestId}] Request: method: ${method} URL: ${url} headers: X-API-Key: [REDACTED], Content-Type: application/json`,
  );

  if (body) {
    let bodyStr = JSON.stringify(body);
    filteredLog("info", `[${requestId}] Request body: ${bodyStr}`);
    let bytes = new GLib.Bytes(bodyStr);
    message.set_request_body_from_bytes("application/json", bytes);
  }

  try {
    // Using a Promise to handle the async operation properly
    return new Promise((resolve, reject) => {
      session.send_and_read_async(
        message,
        GLib.PRIORITY_DEFAULT,
        null,
        (session, result) => {
          try {
            let bytes = session.send_and_read_finish(result);
            let statusCode = message.get_status();

            // Log response status code and headers
            const responseHeaders = [];
            message.get_response_headers().foreach((name, value) => {
              responseHeaders.push(`${name}: ${value}`);
            });

            filteredLog(
              "info",
              `[${requestId}] Response status: ${statusCode} headers: ${responseHeaders.join(", ")}`,
            );

            if (statusCode >= 200 && statusCode < 300) {
              let decoder = new TextDecoder("utf-8");
              let text = decoder.decode(bytes.get_data());

              // Limit the response body logging to 1000 characters
              const maxLogLength = 1000;
              const logText =
                text.length > maxLogLength
                  ? text.substring(0, maxLogLength) + "... [truncated]"
                  : text;

              filteredLog(
                "info",
                `[${requestId}] Response body (${method} ${path}, ${statusCode}): ${logText}`,
              );

              // Update erroring state for GET requests to groups or lights
              if (
                method === "GET" &&
                ["groups", "lights"].some(
                  (base) => path === base || path.startsWith(`${base}/`),
                )
              ) {
                _erroring = false;
              }

              // Handle empty responses (like DELETE with 204 No Content)
              if (!text || text.trim() === "") {
                if (statusCode === 204) {
                  filteredLog(
                    "info",
                    `[${requestId}] Empty response with 204 status - this is expected`,
                  );
                  resolve({ success: true, statusCode: 204 });
                } else {
                  filteredLog(
                    "info",
                    `[${requestId}] Empty response with ${statusCode} status`,
                  );
                  resolve({});
                }
              } else {
                try {
                  const data = JSON.parse(text);
                  resolve(data);
                } catch (parseError) {
                  filteredLog(
                    "warn",
                    `[${requestId}] Could not parse JSON response: ${parseError.message}`,
                  );
                  // Return the raw text if we can't parse it as JSON
                  resolve({ text: text, statusCode: statusCode });
                }
              }

              // Emit state update event
              if (stateUpdateEmitter) {
                stateUpdateEmitter.emit("keylightd-state-update");
              }
            } else {
              let errorBody = "";
              try {
                let decoder = new TextDecoder("utf-8");
                errorBody = decoder.decode(bytes.get_data());
                filteredLog(
                  "error",
                  `[${requestId}] Error response (${method} ${path}, ${statusCode}): ${errorBody}`,
                );
              } catch (decodeError) {
                filteredLog(
                  "error",
                  `[${requestId}] Failed to decode error response for ${method} ${path}: ${decodeError}`,
                );
              }

              // Update erroring state for GET requests to groups or lights
              if (
                method === "GET" &&
                ["groups", "lights"].some(
                  (base) => path === base || path.startsWith(`${base}/`),
                )
              ) {
                _erroring = true;
              }

              reject(new Error(`HTTP error ${statusCode}: ${errorBody}`));
            }
          } catch (e) {
            filteredLog(
              "error",
              `[${requestId}] Error processing response for ${method} ${path}: ${e.message}`,
            );
            reject(e);
          }
        },
      );
    });
  } catch (e) {
    filteredLog(
      "error",
      `[${requestId}] Request failed for ${method} ${path}: ${e.message}`,
    );
    throw e;
  }
}

/**
 * Fetch all light objects for a group.
 * @param {GroupsController} groupsController
 * @param {LightsController} lightsController
 * @param {string} groupId
 * @returns {Promise<Array>} Array of light objects
 */
export async function getGroupLights(
  groupsController,
  lightsController,
  groupId,
) {
  const group = await groupsController.getGroup(groupId);
  if (!group.lights || !Array.isArray(group.lights)) return [];
  return await Promise.all(
    group.lights.map((id) => lightsController.getLight(id)),
  );
}
