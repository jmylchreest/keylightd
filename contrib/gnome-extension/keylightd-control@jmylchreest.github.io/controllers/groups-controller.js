'use strict';

import { fetchAPI, getGroupLights } from '../utils.js';
import { convertKelvinToDevice } from './lights-controller.js';
import { log } from '../utils.js';

export class GroupsController {
    constructor(settings, stateManager, lightsController) {
        this._settings = settings;
        this._stateManager = stateManager;
        this._cachedGroups = null;
        this._lastFetchTime = 0;
        this._cacheTTL = 10000; // 10 seconds cache TTL
        this._lightsController = lightsController;
    }

    /**
     * Get all groups from the API, with caching
     * @param {boolean} forceRefresh - Force a refresh from API
     * @returns {Promise<Array>} - Promise resolving to array of groups
     */
    async getGroups(forceRefresh = false) {
        const now = Date.now();
        if (!forceRefresh && this._cachedGroups && (now - this._lastFetchTime) < this._cacheTTL) {
            return this._cachedGroups;
        }

        try {
            const groups = await fetchAPI('groups');
            
            // Sort groups by name
            groups.sort((a, b) => {
                const nameA = (a.name || '').toLowerCase();
                const nameB = (b.name || '').toLowerCase();
                return nameA.localeCompare(nameB);
            });
            
            // Update cache
            this._cachedGroups = groups;
            this._lastFetchTime = now;
            
            // Update state manager with the groups
            if (this._stateManager) {
                groups.forEach(group => {
                    this._stateManager.updateGroup(group.id, group);
                });
            }
            
            return groups;
        } catch (error) {
            log('error', `GroupsController: Error fetching groups: ${error}`);
            throw error;
        }
    }

    /**
     * Get visible groups (filtered by settings)
     * @param {boolean} forceRefresh - Force a refresh from API
     * @returns {Promise<Array>} - Promise resolving to array of visible groups
     */
    async getVisibleGroups(forceRefresh = false) {
        const groups = await this.getGroups(forceRefresh);
        const visibleGroupIds = this._settings.get_strv('visible-groups') || [];
        
        if (visibleGroupIds.length === 0) {
            return [];
        }
        
        return groups.filter(group => visibleGroupIds.includes(group.id));
    }

    /**
     * Get a specific group by ID
     * @param {string} groupId - The group ID
     * @returns {Promise<Object>} - Promise resolving to group object
     */
    async getGroup(groupId) {
        try {
            const group = await fetchAPI(`groups/${groupId}`);
            
            if (this._stateManager) {
                this._stateManager.updateGroup(groupId, group);
            }
            
            return group;
        } catch (error) {
            log('error', `GroupsController: Error fetching group ${groupId}: ${error}`);
            throw error;
        }
    }

    /**
     * Set the state (on/off) of a group
     * @param {string} groupId - The group ID
     * @param {boolean} state - The new state (true for on, false for off)
     * @returns {Promise<Object>} - Promise resolving to API response
     */
    async setGroupState(groupId, state) {
        try {
            const response = await fetchAPI(`groups/${groupId}/state`, 'PUT', { on: state });
            
            if (this._stateManager) {
                this._stateManager.updateGroup(groupId, { on: state });
                
                // Get the group to update the states of its lights
                const group = await this.getGroup(groupId);
                if (group.lights && Array.isArray(group.lights)) {
                    group.lights.forEach(lightId => {
                        this._stateManager.updateLight(lightId, { on: state });
                    });
                }
            }
            
            return response;
        } catch (error) {
            log('error', `GroupsController: Error setting group ${groupId} state to ${state}: ${error}`);
            throw error;
        }
    }

    /**
     * Set the brightness of a group
     * @param {string} groupId - The group ID
     * @param {number} brightness - The new brightness (3-100)
     * @returns {Promise<Object>} - Promise resolving to API response
     */
    async setGroupBrightness(groupId, brightness) {
        // Ensure brightness is within valid range
        brightness = Math.max(3, Math.min(100, brightness));
        
        try {
            const response = await fetchAPI(`groups/${groupId}/state`, 'PUT', { brightness: brightness });
            
            if (this._stateManager) {
                this._stateManager.updateGroup(groupId, { brightness: brightness });
                
                // Get the group to update the states of its lights
                const group = await this.getGroup(groupId);
                if (group.lights && Array.isArray(group.lights)) {
                    group.lights.forEach(lightId => {
                        this._stateManager.updateLight(lightId, { brightness: brightness });
                    });
                }
            }
            
            return response;
        } catch (error) {
            log('error', `GroupsController: Error setting group ${groupId} brightness to ${brightness}: ${error}`);
            throw error;
        }
    }

    /**
     * Set the temperature of a group
     * @param {string} groupId - The group ID
     * @param {number} temperature - The new temperature in Kelvin (2900-7000)
     * @returns {Promise<Object>} - Promise resolving to API response
     */
    async setGroupTemperature(groupId, temperature) {
        // Send the Kelvin value directly to the API (no conversion to mireds)
        try {
            // Ensure temperature is within valid range
            temperature = Math.max(2900, Math.min(7000, temperature));
            
            // Log the temperature value being sent
            log('debug', `GroupsController: Setting group ${groupId} temperature to ${temperature}K`);
            
            const response = await fetchAPI(`groups/${groupId}/state`, 'PUT', { temperature: temperature });
            
            if (this._stateManager) {
                // For UI compatibility, calculate the equivalent mireds value
                const mireds = convertKelvinToDevice(temperature);
                
                this._stateManager.updateGroup(groupId, { 
                    temperature: mireds, // Keep this for compatibility
                    temperatureK: temperature
                });
                
                // Get the group to update the states of its lights
                const group = await this.getGroup(groupId);
                if (group.lights && Array.isArray(group.lights)) {
                    group.lights.forEach(lightId => {
                        this._stateManager.updateLight(lightId, { 
                            temperature: mireds, // Keep this for compatibility
                            temperatureK: temperature
                        });
                    });
                }
            }
            
            return response;
        } catch (error) {
            log('error', `GroupsController: Error setting group ${groupId} temperature to ${temperature}K: ${error}`);
            throw error;
        }
    }

    /**
     * Check if a group's lights are on.
     * @param {string} groupId
     * @param {'any'|'all'} mode - 'any' (default): true if any light is on; 'all': true if all lights are on.
     * @returns {Promise<boolean>}
     */
    async isGroupOn(groupId, mode = 'any') {
        try {
            const lights = await getGroupLights(this, this._lightsController, groupId);
            if (!lights || lights.length === 0) return false;
            if (mode === 'all') return lights.every(light => light.on === true);
            return lights.some(light => light.on === true);
        } catch (error) {
            log('error', `GroupsController: Error determining if group ${groupId} is on: ${error}`);
            return false;
        }
    }

    /**
     * Check if any visible group is on (any or all, depending on mode)
     * @param {'any'|'all'} mode
     * @returns {Promise<boolean>}
     */
    async isAnyVisibleGroupOn(mode = 'any') {
        try {
            const visibleGroups = await this.getVisibleGroups();
            for (const group of visibleGroups) {
                if (await this.isGroupOn(group.id, mode)) return true;
            }
            return false;
        } catch (error) {
            log('error', `GroupsController: Error checking visible groups: ${error}`);
            return false;
        }
    }

    /**
     * Toggle all visible groups
     * @param {boolean} state - Optional target state, if not provided will toggle to opposite of current state
     * @returns {Promise<void>} - Promise resolving when all groups are toggled
     */
    async toggleAllGroups(state = null) {
        try {
            const visibleGroups = await this.getVisibleGroups();
            
            if (visibleGroups.length === 0) {
                log('info', 'GroupsController: No visible groups to toggle');
                return;
            }
            
            // If state is not provided, determine it by checking if any light is on
            if (state === null) {
                const anyLightOn = await this.isAnyVisibleGroupOn();
                state = !anyLightOn;
            }
            
            log('info', `GroupsController: Toggling all visible groups to ${state}`);
            
            // Toggle all groups in parallel
            const promises = visibleGroups.map(group => 
                this.setGroupState(group.id, state)
            );
            
            await Promise.all(promises);
            log('debug', 'GroupsController: All group toggle operations completed');
        } catch (error) {
            log('error', `GroupsController: Error toggling all groups: ${error}`);
            throw error;
        }
    }
} 