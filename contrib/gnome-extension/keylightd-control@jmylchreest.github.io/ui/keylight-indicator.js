'use strict';

import GObject from 'gi://GObject';
import Shell from 'gi://Shell';
import * as QuickSettings from 'resource:///org/gnome/shell/ui/quickSettings.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as Config from 'resource:///org/gnome/shell/misc/config.js';
import { KeylightdControlToggle } from './keylight-toggle.js';
import Gio from 'gi://Gio';
import St from 'gi://St';
import { getLightStateIcon, LightState, determineLightState } from '../icons.js';
import { log } from '../utils.js';

// Parse GNOME Shell version for compatibility
const ShellVersion = parseFloat(Config.PACKAGE_VERSION);

/**
 * KeylightdControl is the main system indicator for keylightd
 * It adds the toggle to the QuickSettings menu and manages the indicator icon
 */
export const KeylightdControl = GObject.registerClass(
    class KeylightdControl extends QuickSettings.SystemIndicator {
        _init(extension) {
            super._init();
            
            this._extension = extension;
            this._settings = extension._settings;
            this._appSystem = Shell.AppSystem.get_default();
            this._indicator = this._addIndicator();
            
            // Create toggle menu
            this._keylightdToggle = new KeylightdControlToggle(extension);
            
            // Connect to toggle's notify::gicon signal to update the indicator icon
            this._keylightdToggle.connect('notify::gicon', () => {
                if (this._keylightdToggle.gicon) {
                    this._indicator.gicon = this._keylightdToggle.gicon;
                    this._indicator.visible = true;
                }
            });
            
            // Connect to menu open/close events to sync icon state
            this._keylightdToggle.menu.connect('open-state-changed', (menu, isOpen) => {
                // When menu closes, ensure our icon is synced with the toggle
                if (!isOpen && this._keylightdToggle && this._keylightdToggle.gicon && this._indicator) {
                    this._indicator.gicon = this._keylightdToggle.gicon;
                    log('debug', 'Synced indicator icon after menu close');
                }
            });
            
            // Set initial icon - initially set to UNKNOWN but will be updated shortly
            this._indicator.gicon = getLightStateIcon(LightState.UNKNOWN);
            this._indicator.visible = true;
            
            // Add to quick settings
            this.quickSettingsItems.push(this._keylightdToggle);
            
            // Add indicator and toggle to QuickSettings
            this._addToQuickSettings();
            
            // Schedule a delayed check to update the icon after initial setup
            // Run it sooner since we want the icon to be correct quickly
            setTimeout(() => {
                this._updateIndicatorVisibility();
            }, 1000);
            
            // Schedule another update after a longer time to ensure we have data
            setTimeout(() => {
                this._updateIndicatorVisibility();
            }, 3000);
        }
        
        /**
         * Add the indicator to QuickSettings
         */
        _addToQuickSettings() {
            const QuickSettingsMenu = Main.panel.statusArea.quickSettings;
            
            // Add external indicator
            QuickSettingsMenu.addExternalIndicator(this);
            
            // GNOME Shell 46+ has different API for removing/adding children
            if (ShellVersion >= 46) {
                QuickSettingsMenu._indicators.remove_child(this);
            } else {
                QuickSettingsMenu._indicators.remove_actor(this);
            }
            
            // Insert at the specific index
            QuickSettingsMenu._indicators.insert_child_at_index(this, this.indicatorIndex);
        }
        
        /**
         * Update the indicator icon
         * @param {string} state - The state to use ('ENABLED', 'DISABLED', or 'UNKNOWN')
         */
        _updateIndicatorIcon(state) {
            if (!this._indicator) return;
            
            // Use constants from LightState
            const actualState = state === LightState.ENABLED ? LightState.ENABLED :
                               state === LightState.DISABLED ? LightState.DISABLED : 
                               LightState.UNKNOWN;
                               
            this._indicator.gicon = getLightStateIcon(actualState);
            log('debug', `Indicator icon set to state: ${actualState}`);
            this._indicator.visible = true;
        }
        
        /**
         * Update the indicator visibility based on state
         * This will be called manually when needed
         */
        _updateIndicatorVisibility() {
            if (!this._indicator) return;
            
            // Ensure visibility
            this._indicator.visible = true;
            
            // Get state manager
            const stateManager = this._extension._stateManager;
            
            // Get all lights and groups
            if (stateManager) {
                const allLights = stateManager.getAllLights();
                const allGroups = stateManager.getAllGroups();
                
                // Get visible IDs
                const visibleLightIds = this._settings.get_strv('visible-lights');
                const visibleGroupIds = this._settings.get_strv('visible-groups');
                
                // Determine state
                const state = determineLightState(allLights, allGroups, visibleLightIds, visibleGroupIds);
                
                // Set the icon
                this._indicator.gicon = getLightStateIcon(state);
                log('debug', `Updating indicator icon to state: ${state}`);
            } else {
                // Default to unknown icon if no state manager
                this._indicator.gicon = getLightStateIcon(LightState.UNKNOWN);
                log('debug', 'Setting indicator to UNKNOWN state (no state manager)');
            }
        }
        
        /**
         * Destroy the indicator and clean up
         */
        destroy() {
            // Clean up toggle
            this.quickSettingsItems.forEach((item) => item.destroy());
            
            // Clean up references
            this._keylightdToggle = null;
            this._indicator = null;
            this._appSystem = null;
            this._settings = null;
            this._extension = null;
            
            // Call parent destroy
            super.destroy();
        }
    }
); 