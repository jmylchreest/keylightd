'use strict';

import { log } from '../utils.js';

// Event types for state changes
export const StateEvents = Object.freeze({
    LIGHT_UPDATED: 'light-updated',
    GROUP_UPDATED: 'group-updated',
    STATE_CHANGED: 'state-changed',
    ERROR: 'error'
});

/**
 * StateManager provides centralized state management with pub/sub pattern
 * for the extension. It stores light and group states and notifies subscribers
 * of changes.
 */
export class StateManager {
    constructor() {
        // Maps to store current state
        this._lights = new Map();
        this._groups = new Map();
        
        // Event listeners for different event types
        this._listeners = new Map();
        
        // Error state
        this._erroring = false;
        
        // Initialize event types
        Object.values(StateEvents).forEach(event => {
            this._listeners.set(event, new Set());
        });
    }
    
    /**
     * Subscribe to a state event
     * @param {string} event - Event type from StateEvents
     * @param {Function} callback - Callback function
     * @returns {Function} - Unsubscribe function
     */
    subscribe(event, callback) {
        if (!this._listeners.has(event)) {
            log('warn', `StateManager: Unknown event type: ${event}`);
            return () => {};
        }
        
        const listeners = this._listeners.get(event);
        listeners.add(callback);
        
        // Return unsubscribe function
        return () => {
            listeners.delete(callback);
        };
    }
    
    /**
     * Emit an event to all subscribers
     * @param {string} event - Event type from StateEvents
     * @param {any} data - Event data
     * @private
     */
    _emit(event, data) {
        if (!this._listeners.has(event)) {
            log('warn', `StateManager: Trying to emit unknown event type: ${event}`);
            return;
        }
        
        const listeners = this._listeners.get(event);
        listeners.forEach(callback => {
            if (typeof callback !== 'function') return;
            try {
                callback(data);
            } catch (error) {
                log('error', `StateManager: Error in event listener for ${event}:`, error);
            }
        });
        
        // Also emit the general STATE_CHANGED event
        if (event !== StateEvents.STATE_CHANGED) {
            const generalListeners = this._listeners.get(StateEvents.STATE_CHANGED);
            generalListeners.forEach(callback => {
                if (typeof callback !== 'function') return;
                try {
                    callback({ type: event, data });
                } catch (error) {
                    log('error', `StateManager: Error in general state change listener:`, error);
                }
            });
        }
    }
    
    /**
     * Set error state
     * @param {boolean} isErroring - Whether there's an error
     * @param {Error} error - Optional error object
     */
    setErroring(isErroring, error = null) {
        const oldErroring = this._erroring;
        this._erroring = isErroring;
        
        if (oldErroring !== isErroring) {
            this._emit(StateEvents.ERROR, { 
                isErroring,
                error
            });
        }
    }
    
    /**
     * Get current error state
     * @returns {boolean} - Whether there's an error
     */
    isErroring() {
        return this._erroring;
    }
    
    /**
     * Update a light's state
     * @param {string} lightId - Light ID
     * @param {Object} updates - State updates
     */
    updateLight(lightId, updates) {
        if (!lightId || typeof lightId !== 'string' || !updates) {
            log('warn', `StateManager: Attempted to update light with invalid ID:`, updates);
            return;
        }
        
        // Get current state or initialize
        const currentState = this._lights.get(lightId) || {};
        
        // Merge updates with current state
        const newState = { ...currentState, ...updates };
        
        // Store updated state
        this._lights.set(lightId, newState);
        
        // Emit event
        this._emit(StateEvents.LIGHT_UPDATED, {
            id: lightId,
            state: newState,
            updates
        });
    }
    
    /**
     * Update a group's state
     * @param {string} groupId - Group ID
     * @param {Object} updates - State updates
     */
    updateGroup(groupId, updates) {
        if (!groupId || typeof groupId !== 'string' || !updates) {
            log('warn', `StateManager: Attempted to update group with invalid ID:`, updates);
            return;
        }
        
        // Get current state or initialize
        const currentState = this._groups.get(groupId) || {};
        
        // Merge updates with current state
        const newState = { ...currentState, ...updates };
        
        // Store updated state
        this._groups.set(groupId, newState);
        
        // Emit event
        this._emit(StateEvents.GROUP_UPDATED, {
            id: groupId,
            state: newState,
            updates
        });
    }
    
    /**
     * Get a light's state
     * @param {string} lightId - Light ID
     * @returns {Object|null} - Light state or null if not found
     */
    getLight(lightId) {
        return this._lights.get(lightId) || null;
    }
    
    /**
     * Get a group's state
     * @param {string} groupId - Group ID
     * @returns {Object|null} - Group state or null if not found
     */
    getGroup(groupId) {
        return this._groups.get(groupId) || null;
    }
    
    /**
     * Get all lights
     * @returns {Map} - Map of light IDs to states
     */
    getAllLights() {
        return new Map(this._lights);
    }
    
    /**
     * Get all groups
     * @returns {Map} - Map of group IDs to states
     */
    getAllGroups() {
        return new Map(this._groups);
    }
    
    /**
     * Check if any tracked light is on
     * @returns {boolean} - True if any light is on
     */
    isAnyLightOn() {
        for (const light of this._lights.values()) {
            if (light.on === true) {
                return true;
            }
        }
        return false;
    }
    
    /**
     * Clear all state
     */
    clearState() {
        this._lights.clear();
        this._groups.clear();
        this._emit(StateEvents.STATE_CHANGED, { type: 'clear' });
    }
} 