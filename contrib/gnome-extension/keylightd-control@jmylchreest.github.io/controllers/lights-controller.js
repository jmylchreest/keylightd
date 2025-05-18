'use strict';

import { fetchAPI } from '../utils.js';
import { log } from '../utils.js';

// Temperature conversion constants
const MIN_MIREDS = 143;
const MAX_MIREDS = 344;
const MIN_KELVIN = 2900;
const MAX_KELVIN = 7000;

// Convert device mireds to Kelvin
export function convertDeviceToKelvin(mireds) {
    if (mireds < MIN_MIREDS) {
        mireds = MIN_MIREDS;
    } else if (mireds > MAX_MIREDS) {
        mireds = MAX_MIREDS;
    }
    return Math.round(1000000 / mireds);
}

// Convert Kelvin to device mireds
export function convertKelvinToDevice(kelvin) {
    if (kelvin < MIN_KELVIN) {
        kelvin = MIN_KELVIN;
    } else if (kelvin > MAX_KELVIN) {
        kelvin = MAX_KELVIN;
    }
    let mireds = Math.round(1000000 / kelvin);
    if (mireds > MAX_MIREDS) {
        mireds = MAX_MIREDS;
    } else if (mireds < MIN_MIREDS) {
        mireds = MIN_MIREDS;
    }
    return mireds;
}

export class LightsController {
    constructor(settings, stateManager) {
        this._settings = settings;
        this._stateManager = stateManager;
        this._cachedLights = null;
        this._lastFetchTime = 0;
        this._cacheTTL = 10000; // 10 seconds cache TTL
    }

    /**
     * Get all lights from the API, with caching
     * @param {boolean} forceRefresh - Force a refresh from API
     * @returns {Promise<Array>} - Promise resolving to array of lights
     */
    async getLights(forceRefresh = false) {
        const now = Date.now();
        if (!forceRefresh && this._cachedLights && (now - this._lastFetchTime) < this._cacheTTL) {
            return this._cachedLights;
        }

        try {
            const lightsResponse = await fetchAPI('lights');
            
            // Convert response to array if it's an object
            let lightsArray = [];
            if (Array.isArray(lightsResponse)) {
                lightsArray = lightsResponse;
            } else if (lightsResponse && typeof lightsResponse === 'object') {
                // Convert object of lights to array
                lightsArray = Object.entries(lightsResponse).map(([id, lightData]) => {
                    return {
                        id: id,
                        name: lightData.name || id,
                        ...lightData
                    };
                });
            }
            
            // Sort lights by name
            lightsArray.sort((a, b) => {
                const nameA = (a.name || '').toLowerCase();
                const nameB = (b.name || '').toLowerCase();
                return nameA.localeCompare(nameB);
            });
            
            // Update cache
            this._cachedLights = lightsArray;
            this._lastFetchTime = now;
            
            // Update state manager with the lights
            if (this._stateManager) {
                lightsArray.forEach(light => {
                    this._stateManager.updateLight(light.id, light);
                });
            }
            
            return lightsArray;
        } catch (error) {
            log('error', `LightsController: Error fetching lights: ${error}`);
            throw error;
        }
    }

    /**
     * Get visible lights (filtered by settings)
     * @param {boolean} forceRefresh - Force a refresh from API
     * @returns {Promise<Array>} - Promise resolving to array of visible lights
     */
    async getVisibleLights(forceRefresh = false) {
        const lights = await this.getLights(forceRefresh);
        const visibleLightIds = this._settings.get_strv('visible-lights') || [];
        
        if (visibleLightIds.length === 0) {
            return [];
        }
        
        return lights.filter(light => visibleLightIds.includes(light.id));
    }

    /**
     * Get a specific light by ID
     * @param {string} lightId - The light ID
     * @returns {Promise<Object>} - Promise resolving to light object
     */
    async getLight(lightId) {
        try {
            const light = await fetchAPI(`lights/${lightId}`);
            
            if (this._stateManager) {
                this._stateManager.updateLight(lightId, light);
            }
            
            return light;
        } catch (error) {
            log('error', `LightsController: Error fetching light ${lightId}: ${error}`);
            throw error;
        }
    }

    /**
     * Set the state (on/off) of a light
     * @param {string} lightId - The light ID
     * @param {boolean} state - The new state (true for on, false for off)
     * @returns {Promise<Object>} - Promise resolving to API response
     */
    async setLightState(lightId, state) {
        try {
            const response = await fetchAPI(`lights/${lightId}/state`, 'POST', { on: state });
            
            if (this._stateManager) {
                this._stateManager.updateLight(lightId, { on: state });
            }
            
            return response;
        } catch (error) {
            log('error', `LightsController: Error setting light ${lightId} state to ${state}: ${error}`);
            throw error;
        }
    }

    /**
     * Set the brightness of a light
     * @param {string} lightId - The light ID
     * @param {number} brightness - The new brightness (0-100)
     * @returns {Promise<Object>} - Promise resolving to API response
     */
    async setLightBrightness(lightId, brightness) {
        // Ensure brightness is within valid range
        brightness = Math.max(0, Math.min(100, brightness));
        
        try {
            const response = await fetchAPI(`lights/${lightId}/state`, 'POST', { brightness: brightness });
            
            if (this._stateManager) {
                this._stateManager.updateLight(lightId, { brightness: brightness });
            }
            
            return response;
        } catch (error) {
            log('error', `LightsController: Error setting light ${lightId} brightness to ${brightness}: ${error}`);
            throw error;
        }
    }

    /**
     * Set the temperature of a light
     * @param {string} lightId - The light ID
     * @param {number} temperature - The new temperature in Kelvin (2900-7000)
     * @returns {Promise<Object>} - Promise resolving to API response
     */
    async setLightTemperature(lightId, temperature) {
        // Send the Kelvin value directly to the API (no conversion to mireds)
        try {
            // Ensure temperature is within valid range
            temperature = Math.max(2900, Math.min(7000, temperature));
            
            // Log the temperature value being sent
            log('debug', `LightsController: Setting light ${lightId} temperature to ${temperature}K`);
            
            const response = await fetchAPI(`lights/${lightId}/state`, 'POST', { temperature: temperature });
            
            if (this._stateManager) {
                // Store both the Kelvin value and the equivalent mireds value for the UI
                const mireds = convertKelvinToDevice(temperature);
                this._stateManager.updateLight(lightId, { 
                    temperature: mireds, // Keep this for compatibility
                    temperatureK: temperature
                });
            }
            
            return response;
        } catch (error) {
            log('error', `LightsController: Error setting light ${lightId} temperature to ${temperature}K: ${error}`);
            throw error;
        }
    }

    /**
     * Check if any visible light is on
     * @returns {Promise<boolean>} - Promise resolving to true if any light is on
     */
    async isAnyLightOn() {
        try {
            const visibleLights = await this.getVisibleLights();
            
            for (const light of visibleLights) {
                try {
                    const lightDetails = await this.getLight(light.id);
                    if (lightDetails.on === true) {
                        return true;
                    }
                } catch (error) {
                    log('error', `LightsController: Error checking light ${light.id} status: ${error}`);
                }
            }
            
            return false;
        } catch (error) {
            log('error', `LightsController: Error checking if any light is on: ${error}`);
            return false;
        }
    }

    /**
     * Toggle all visible lights
     * @param {boolean} state - Optional target state, if not provided will toggle to opposite of current state
     * @returns {Promise<void>} - Promise resolving when all lights are toggled
     */
    async toggleAllLights(state = null) {
        try {
            const visibleLights = await this.getVisibleLights();
            
            if (visibleLights.length === 0) {
                log('info', 'LightsController: No visible lights to toggle');
                return;
            }
            
            // If state is not provided, get current state of first light and toggle
            if (state === null) {
                const firstLight = await this.getLight(visibleLights[0].id);
                state = !(firstLight.on === true);
            }
            
            log('info', `LightsController: Toggling all visible lights to ${state}`);
            
            // Toggle all lights in parallel
            const promises = visibleLights.map(light => 
                this.setLightState(light.id, state)
            );
            
            await Promise.all(promises);
            log('debug', 'LightsController: All light toggle operations completed');
        } catch (error) {
            log('error', `LightsController: Error toggling all lights: ${error}`);
            throw error;
        }
    }
} 