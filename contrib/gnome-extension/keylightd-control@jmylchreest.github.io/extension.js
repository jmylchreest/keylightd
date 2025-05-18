'use strict';

import GLib from 'gi://GLib';
import GObject from 'gi://GObject';
import St from 'gi://St';
import { Extension } from 'resource:///org/gnome/shell/extensions/extension.js';
import { PopupAnimation } from 'resource:///org/gnome/shell/ui/boxpointer.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';

// Import shared utilities
import { setSettings, log } from './utils.js';

// Import our modules
import { StateManager } from './managers/state-manager.js';
import { LightsController } from './controllers/lights-controller.js';
import { GroupsController } from './controllers/groups-controller.js';
import { UIBuilder } from './ui/ui-builder.js';
import { KeylightdControl } from './ui/keylight-indicator.js';
import { initIcons } from './icons.js';

// Define the emitter code locally here, not exported
const KeylightdStateUpdateEmitter = GObject.registerClass({
    Signals: {
        'keylightd-state-update': {
            param_types: [], // Use [] if you don't pass parameters, or [GObject.TYPE_BOXED] if you do
        },
    },
}, class KeylightdStateUpdateEmitter extends GObject.Object {});

const keylightdStateUpdateEmitter = new KeylightdStateUpdateEmitter();

function iconExists(iconName) {
    try {
        let icon = new St.Icon({ icon_name: iconName });
        return true;
    } catch (e) {
        log('warn', `Icon "${iconName}" could not be created: ${e}`);
        return false;
    }
}

function checkAllIcons() {
    const ICONS_TO_CHECK = [
        'emblem-system-symbolic',
        'user-group-symbolic',
        'display-brightness-symbolic',
        'preferences-desktop-display-symbolic',
        // Add any custom or extension-specific icon names here
        'keylightd-general-symbolic',
        'keylightd-groups-symbolic',
        'keylightd-lights-symbolic',
        'keylightd-ui-symbolic',
    ];
    ICONS_TO_CHECK.forEach(iconName => {
        if (!iconExists(iconName)) {
            log('warn', `Icon "${iconName}" not found in current icon theme!`);
        }
    });
}

/**
 * Main extension class
 */
export default class KeylightdControlExtension extends Extension {
    /**
     * Enable the extension
     */
    enable() {
        // Initialize settings
        this._settings = this.getSettings();
        
        // Set the settings reference in utils.js
        setSettings(this._settings);
        // Initialize icons.js with the extension path
        initIcons(this.path);
        
        // Create the state manager
        this._stateManager = new StateManager();
        
        // Create controllers
        this._lightsController = new LightsController(this._settings, this._stateManager);
        this._groupsController = new GroupsController(this._settings, this._stateManager);
        
        // Create UI builder
        this._uiBuilder = new UIBuilder(this.path, this._settings);
        
        // Create and add the indicator
        this._keylightdIndicator = new KeylightdControl(this);
        
        // Add indicator to panel
        Main.panel.statusArea.quickSettings.add_child(this._keylightdIndicator);
        
        // Set up background refresh based on preferences
        this._setupBackgroundRefresh();
        
        // Connect to settings changes for refresh interval
        this._settings.connect('changed::refresh-interval', () => {
            this._setupBackgroundRefresh();
        });
        
        // Connect to settings changes for debounce delay
        this._settings.connect('changed::debounce-delay', () => {
            if (this._uiBuilder) {
                this._uiBuilder._debounceDelay = this._settings.get_int('debounce-delay');
            }
        });
        
        // Connect to visibility setting changes to ensure UI and preferences stay in sync
        this._settings.connect('changed::visible-groups', () => {
            this._syncVisibilityState();
        });
        
        this._settings.connect('changed::visible-lights', () => {
            this._syncVisibilityState();
        });
        
        // Listen for state updates from API calls
        this._stateUpdateEmitter = keylightdStateUpdateEmitter;
        this._stateUpdateId = this._stateUpdateEmitter.connect('keylightd-state-update', (emitter, event) => {
            this._handleStateUpdate(event);
        });
        
        // Perform an initial refresh of light/group states
        this._performInitialRefresh();

        checkAllIcons();
    }

    /**
     * Disable the extension
     */
    disable() {
        // Remove state update listener
        if (this._stateUpdateId && typeof global.off === 'function') {
            global.off(this._stateUpdateId);
            this._stateUpdateId = null;
        }
        
        // Remove background refresh timer
        this._clearBackgroundRefresh();
        
        // Remove the indicator
        if (this._keylightdIndicator) {
            Main.panel.statusArea.quickSettings.remove_child(this._keylightdIndicator);
            this._keylightdIndicator.destroy();
            this._keylightdIndicator = null;
        }
        
        // Clean up controllers and managers
        this._lightsController = null;
        this._groupsController = null;
        this._uiBuilder = null;
        this._stateManager = null;
        
        // Clear the settings reference in utils
        setSettings(null);
        this._settings = null;
    }

    /**
     * Set up background refresh timer based on preferences
     */
    _setupBackgroundRefresh() {
        // Clear any existing timer
        this._clearBackgroundRefresh();
        
        // Get refresh interval from settings (in seconds)
        const refreshInterval = this._settings.get_int('refresh-interval');
        
        if (refreshInterval > 0) {
            // Convert to milliseconds for the timer
            const timeoutMs = refreshInterval * 1000;
            
            // Set up the timer for regular background refresh
            this._refreshTimeoutId = GLib.timeout_add(
                GLib.PRIORITY_DEFAULT,
                timeoutMs,
                () => {
                    this._performBackgroundRefresh();
                    return GLib.SOURCE_CONTINUE; // Keep timer running
                }
            );
        }
    }
    
    /**
     * Clear the background refresh timer
     */
    _clearBackgroundRefresh() {
        if (this._refreshTimeoutId) {
            GLib.source_remove(this._refreshTimeoutId);
            this._refreshTimeoutId = 0;
        }
    }
    
    /**
     * Perform background refresh of light/group states
     */
    async _performBackgroundRefresh() {
        try {
            // Get current states
            const previousLightStates = new Map(this._stateManager.getAllLights());
            const previousGroupStates = new Map(this._stateManager.getAllGroups());
            
            // Refresh groups
            try {
            await this._groupsController.getGroups(true);
            } catch (groupError) {
                console.error('Error refreshing groups during background refresh:', groupError);
                return; // Exit early on error to avoid inconsistent state comparison
            }
            
            // Refresh lights
            try {
            await this._lightsController.getLights(true);
            } catch (lightError) {
                console.error('Error refreshing lights during background refresh:', lightError);
                return; // Exit early on error to avoid inconsistent state comparison
            }
            
            // Check if any state has changed
            let stateChanged = false;
            
            // Compare light states
            const currentLights = this._stateManager.getAllLights();
            for (const [id, light] of currentLights.entries()) {
                const prevLight = previousLightStates.get(id);
                if (!prevLight || prevLight.on !== light.on || 
                    prevLight.brightness !== light.brightness || 
                    prevLight.temperature !== light.temperature) {
                    stateChanged = true;
                    break;
                }
            }
            
            // Compare group states if needed
            if (!stateChanged) {
                const currentGroups = this._stateManager.getAllGroups();
                for (const [id, group] of currentGroups.entries()) {
                    const prevGroup = previousGroupStates.get(id);
                    if (!prevGroup || prevGroup.on !== group.on) {
                        stateChanged = true;
                        break;
                    }
                }
            }
            
            // If state changed, update UI elements
            if (stateChanged) {
                // Update toggle state if menu is open
                if (this._keylightdIndicator && 
                    this._keylightdIndicator._keylightdToggle) {
                    
                    // Always update the toggle state even if menu isn't open
                    try {
                        const toggle = this._keylightdIndicator._keylightdToggle;
                        if (toggle.menu && toggle.menu.isOpen !== undefined) {
                            toggle._updateToggleState();
                        }
                    } catch (toggleError) {
                        console.error('Error updating toggle state during background refresh:', toggleError);
                    }
                    
                    // Also update the indicator itself
                    try {
                        this._keylightdIndicator._updateIndicatorVisibility();
                    } catch (indicatorError) {
                        console.error('Error updating indicator visibility during background refresh:', indicatorError);
                    }
                }
            }
            
        } catch (error) {
            console.error('Error during background refresh:', error);
        }
    }

    /**
     * Open the extension preferences window
     */
    _openPreferences() {
        try {
            // Use the ExtensionManager to open preferences in GNOME 48
            const ExtensionManager = Main.extensionManager;
            if (ExtensionManager) {
                ExtensionManager.openExtensionPrefs(this.uuid, '', {});
            } else {
                // Fallback to the old method if extensionManager is not available
                this.openPreferences?.();
            }
            
            // Close the QuickSettings menu
            const QuickSettingsMenu = Main.panel.statusArea.quickSettings;
            if (QuickSettingsMenu && QuickSettingsMenu.menu) {
                QuickSettingsMenu.menu.close();
            }
        } catch (error) {
            console.error(`Error opening preferences: ${error}`);
        }
    }

    /**
     * Perform initial refresh of light/group states
     * This loads data immediately when the extension starts
     */
    async _performInitialRefresh() {
        try {
            // Wait a bit to ensure the extension is fully initialized
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            // Refresh groups first (as they might contain lights)
            try {
                await this._groupsController.getGroups(true);
            } catch (groupError) {
                console.error('Error refreshing groups:', groupError);
            }
            
            // Then refresh lights
            try {
                await this._lightsController.getLights(true);
            } catch (lightError) {
                console.error('Error refreshing lights:', lightError);
            }
            
            // Update the indicator state - make sure all components exist
            if (this._keylightdIndicator) {
                try {
                    this._keylightdIndicator._updateIndicatorVisibility();
                } catch (indicatorError) {
                    console.error('Error updating indicator visibility:', indicatorError);
                }
                
                // Update toggle state too, but only if it exists and is properly initialized
                if (this._keylightdIndicator._keylightdToggle) {
                    try {
                        // Check if the toggle has a menu with a setHeader method before updating
                        const toggle = this._keylightdIndicator._keylightdToggle;
                        if (toggle.menu && toggle.menu.isOpen !== undefined) {
                            toggle._updateToggleState();
                        }
                    } catch (toggleError) {
                        console.error('Error updating toggle state:', toggleError);
                    }
                }
            }
            
            // Schedule another refresh after UI is more likely to be fully initialized
            setTimeout(() => {
                try {
            if (this._keylightdIndicator && this._keylightdIndicator._keylightdToggle) {
                this._keylightdIndicator._keylightdToggle._updateToggleState();
            }
                } catch (error) {
                    console.error('Error in delayed toggle update:', error);
                }
            }, 2000);
            
        } catch (error) {
            console.error('Error during initial refresh:', error);
        }
    }

    /**
     * Sync the visibility state between settings and UI
     */
    _syncVisibilityState() {
        try {
            // Update indicator visibility
            if (this._keylightdIndicator) {
                this._keylightdIndicator._updateIndicatorVisibility();
                
                // Update toggle state if it exists
                if (this._keylightdIndicator._keylightdToggle) {
                    this._keylightdIndicator._keylightdToggle._updateToggleState();
                }
            }
        } catch (error) {
            console.error('Error syncing visibility state:', error);
        }
    }
} 