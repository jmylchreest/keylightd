'use strict';

import GLib from 'gi://GLib';
import Soup from 'gi://Soup';

// Global settings instance - set by the extension
let _settingsInstance = null;
let _erroring = true;

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
 * Helper function to make API requests to the keylightd API
 * This automatically uses the API URL and key from extension settings
 * 
 * @param {string} path - API path to call 
 * @param {string} method - HTTP method (GET, POST, PUT, DELETE)
 * @param {Object|null} body - Optional request body
 * @returns {Promise<Object>} - Promise resolving to the API response
 * @throws {Error} If the API URL or key is not configured, or if the request fails
 */
export async function fetchAPI(path, method = 'GET', body = null) {
    console.log(`[keylightd] API Request: ${method} ${path}`);
    
    if (!_settingsInstance) {
        console.error('[keylightd] Settings have not been initialized');
        throw new Error('Settings have not been initialized');
    }
    
    const endpoint = _settingsInstance.get_string('api-url');
    const apiKey = _settingsInstance.get_string('api-key');
    
    // Check if required settings are available
    if (!endpoint || !apiKey) {
        console.error('[keylightd] API URL or API Key not configured');
        // Update erroring state for GET requests to groups or lights
        if (method === 'GET' && (path === 'groups' || path.startsWith('groups/') || 
                                path === 'lights' || path.startsWith('lights/'))) {
            _erroring = true;
        }
        throw new Error('API URL or API Key not configured');
    }
    
    let url = `${endpoint}/api/v1/${path}`;
    console.log(`[keylightd] Full URL: ${url}`);
    
    let session = new Soup.Session();
    let message = Soup.Message.new(method, url);
    message.get_request_headers().append('X-API-Key', apiKey);
    
    if (body) {
        let bodyStr = JSON.stringify(body);
        console.log(`[keylightd] Request body: ${bodyStr}`);
        let bytes = new GLib.Bytes(bodyStr);
        message.set_request_body_from_bytes('application/json', bytes);
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
                        console.log(`[keylightd] Response status: ${statusCode}`);
                        
                        if (statusCode >= 200 && statusCode < 300) {
                            let decoder = new TextDecoder('utf-8');
                            let text = decoder.decode(bytes.get_data());
                            console.log(`[keylightd] Response body: ${text}`);
                            
                            // Update erroring state for GET requests to groups or lights
                            if (method === 'GET' && (path === 'groups' || path.startsWith('groups/') || 
                                                     path === 'lights' || path.startsWith('lights/'))) {
                                _erroring = false;
                            }
                            
                            // Handle empty responses (like DELETE with 204 No Content)
                            if (!text || text.trim() === '') {
                                if (statusCode === 204) {
                                    resolve({success: true, statusCode: 204});
                                } else {
                                    console.log(`[keylightd] Empty response with ${statusCode} status`);
                                    resolve({});
                                }
                            } else {
                                try {
                                    resolve(JSON.parse(text));
                                } catch (parseError) {
                                    console.warn(`[keylightd] Could not parse JSON response: ${parseError.message}`);
                                    // Return the raw text if we can't parse it as JSON
                                    resolve({text: text, statusCode: statusCode});
                                }
                            }
                        } else {
                            let errorBody = '';
                            try {
                                let decoder = new TextDecoder('utf-8');
                                errorBody = decoder.decode(bytes.get_data());
                                console.error(`[keylightd] Error response body: ${errorBody}`);
                            } catch (decodeError) {
                                console.error(`[keylightd] Failed to decode error response: ${decodeError}`);
                            }
                            
                            // Update erroring state for GET requests to groups or lights
                            if (method === 'GET' && (path === 'groups' || path.startsWith('groups/') || 
                                                     path === 'lights' || path.startsWith('lights/'))) {
                                _erroring = true;
                            }
                            
                            reject(new Error(`HTTP error ${statusCode}: ${errorBody}`));
                        }
                    } catch (e) {
                        console.error(`[keylightd] Error processing response: ${e.message}`);
                        reject(e);
                    }
                }
            );
        });
    } catch (e) {
        console.error(`[keylightd] Request failed: ${e.message}`);
        
        // Update erroring state for GET requests to groups or lights
        if (method === 'GET' && (path === 'groups' || path.startsWith('groups/') || 
                                path === 'lights' || path.startsWith('lights/'))) {
            _erroring = true;
        }
        
        throw e;
    }
} 