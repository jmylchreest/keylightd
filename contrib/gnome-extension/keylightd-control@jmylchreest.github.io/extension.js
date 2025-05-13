// https://gjs.guide/extensions/
// https://gjs-docs.gnome.org/
// https://github.com/eonpatapon/gnome-shell-extension-caffeine/blob/master/caffeine%40patapon.info/extension.js

import GLib from 'gi://GLib';
import Gio from 'gi://Gio';
import GObject from 'gi://GObject';
import St from 'gi://St';
import Shell from 'gi://Shell';
import Clutter from 'gi://Clutter';


import * as Config from 'resource:///org/gnome/shell/misc/config.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as QuickSettings from 'resource:///org/gnome/shell/ui/quickSettings.js';
import * as ExtensionUtils from 'resource:///org/gnome/shell/misc/extensionUtils.js';
import { Extension } from 'resource:///org/gnome/shell/extensions/extension.js';
import * as PopupMenu from 'resource:///org/gnome/shell/ui/popupMenu.js';
import { PopupAnimation } from 'resource:///org/gnome/shell/ui/boxpointer.js';
import { Slider } from 'resource:///org/gnome/shell/ui/slider.js';

// Import shared utilities
import { fetchAPI, setSettings } from './utils.js';

const QuickSettingsMenu = Main.panel.statusArea.quickSettings;
const ShellVersion = parseFloat(Config.PACKAGE_VERSION);


const ActionsPath = '/icons/hicolor/scalable/actions/';
const UnknownIcon = 'light-single-symbolic';
const EnabledIcon = 'light-single-lit-symbolic';
const DisabledIcon = 'light-single-symbolic';

// Temperature conversion constants
const MIN_MIREDS = 143;
const MAX_MIREDS = 344;
const MIN_KELVIN = 2900;
const MAX_KELVIN = 7000;

// Convert device mireds to Kelvin (same as in Go code)
function convertDeviceToKelvin(mireds) {
    if (mireds < MIN_MIREDS) {
        mireds = MIN_MIREDS;
    } else if (mireds > MAX_MIREDS) {
        mireds = MAX_MIREDS;
    }
    return Math.round(1000000 / mireds);
}

// Convert Kelvin to device mireds (same as in Go code)
function convertKelvinToDevice(kelvin) {
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

// Function to create a debounced version of a function
function debounce(func, delay) {
    let timeoutId;
    return function(...args) {
        const context = this;
        clearTimeout(timeoutId);
        timeoutId = setTimeout(() => {
            func.apply(context, args);
        }, delay);
    };
}

const lightStates = Object.freeze({
    UNKNOWN: Symbol('unknown'),
    ENABLED: Symbol('enabled'),
    DISABLED: Symbol('disabled'),
});

const KeylightdControlToggle = GObject.registerClass({
    Signals: {
        // 'timer-clicked': {}
    }
}, class KeylightdControlToggle extends QuickSettings.QuickMenuToggle {
    _init(Me) {
        super._init({
            'title': _('Keylightd'),
            toggleMode: false
        });

        this._settings = Me._settings;
        this._path = Me.path;
        
        // Icons
        this._lightState = lightStates.UNKNOWN;
        this._updateIcon(this._lightState);

        // Populate the contents of the menu with a custom layout
        this.menu.setHeader(this._getIcon(), _('Groups'), _('Group Control'));
        
        // Create main content box - with flexible width
        this._mainBox = new St.BoxLayout({
            vertical: true,
            style_class: 'keylightd-menu-box',
            style: 'width: auto; min-width: 160px; max-width: 200px;'
        });
        
        // // Title section
        // const titleBox = new St.BoxLayout({
        //     style_class: 'keylightd-title-box',
        //     x_expand: true,
        //     y_align: Clutter.ActorAlign.CENTER
        // });
        // const titleLabel = new St.Label({
        //     text: _('Keylight'),
        //     style_class: 'keylightd-title-label',
        //     x_expand: true
        // });
        
        // titleBox.add_child(titleLabel);
        // this._mainBox.add_child(titleBox);
        
        // Container for light controls
        this._lightsContainer = new St.BoxLayout({
            vertical: true,
            style_class: 'keylightd-lights-container'
        });
        
        this._mainBox.add_child(this._lightsContainer);
        
        // Create a menu item to hold our content
        let contentItem = new PopupMenu.PopupBaseMenuItem({
            activate: false,
            can_focus: false,
        });
        contentItem.add_child(this._mainBox);
        this.menu.addMenuItem(contentItem);

        // Set up the menu opening event to refresh contents
        this.menu.connect('open-state-changed', (menu, isOpen) => {
            if (isOpen) {
                this._loadControls();
            }
        });

        // Load groups and create control widgets
        this._loadControls();
        
        // Set up refresh timer
        this._refreshInterval = 0;
        this._refreshTimeoutId = 0;
        this._setupRefreshTimer();

        // Connect settings changes
        this._settings.connect('changed::api-url', () => {
            this._loadControls();
        });
        
        this._settings.connect('changed::api-key', () => {
            this._loadControls();
        });
        
        this._settings.connect('changed::refresh-interval', () => {
            this._setupRefreshTimer();
        });
        
        // Listen for visible groups changes
        this._settings.connect('changed::visible-groups', () => {
            this._loadControls();
        });

        // Preferences
        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());
        // Refresh action
        const refreshAction = this.menu.addAction(_('Refresh'), () => this._loadControls());


        const settingsItem = this.menu.addAction(_('Settings'), () => Me._openPreferences());
        settingsItem.visible = Main.sessionMode.allowSettings;
        this.menu._settingsActions[Me.uuid] = settingsItem;

        this.connect('destroy', () => {
            this._iconUnknown = null;
            this._iconActivated = null;
            this._iconDeactivated = null;
            this._iconTheme = null;
            this._mainBox = null;
            this._lightsContainer = null;
            this._settings = null;
            this._path = null;
            this._lightState = null;
            this.gicon = null;
            this._clearRefreshTimer();
        });
        
        // Connect clicked to toggle all lights
        this.connect('clicked', () => this._toggleLights());
    }
    
    _setupRefreshTimer() {
        // Clear any existing timer
        this._clearRefreshTimer();
        
        // Get the refresh interval from settings (in seconds)
        this._refreshInterval = this._settings.get_int('refresh-interval');
        
        if (this._refreshInterval > 0) {
            // Convert seconds to milliseconds for timeout
            const timeout = this._refreshInterval * 1000;
            
            // Set up a new timer
            this._refreshTimeoutId = GLib.timeout_add(
                GLib.PRIORITY_DEFAULT,
                timeout,
                () => {
                    this._loadControls();
                    return GLib.SOURCE_CONTINUE; // Keep the timer running
                }
            );
            
            log(`[keylightd-control] Refresh timer set to ${this._refreshInterval} seconds`);
        }
    }
    
    _clearRefreshTimer() {
        if (this._refreshTimeoutId > 0) {
            GLib.source_remove(this._refreshTimeoutId);
            this._refreshTimeoutId = 0;
            log('[keylightd-control] Refresh timer cleared');
        }
    }

    async _loadControls() {
        // Store any existing control sections for reuse
        const existingSections = new Map();
        
        // If there are existing children, store references to them
        let child = this._lightsContainer.get_first_child();
        while (child) {
            // Get the nameLabel from the header to identify the section
            if (child instanceof St.BoxLayout && child.get_first_child()) {
                const headerBox = child.get_first_child();
                if (headerBox && headerBox.get_first_child()) {
                    const nameLabel = headerBox.get_first_child();
                    if (nameLabel instanceof St.Label) {
                        existingSections.set(nameLabel.text, child);
                    }
                }
            }
            
            let next = child.get_next_sibling();
            // Remove separators, but keep actual control sections for now
            if (!(child instanceof St.BoxLayout) || child.style_class === 'popup-separator-menu-item-separator') {
                this._lightsContainer.remove_child(child);
            }
            child = next;
        }
        
        let anyLightOn = false;
        let visibleGroupCount = 0;
        let totalGroupCount = 0;
        
        try {
            // Load groups first
            const groups = await fetchAPI('groups');
            totalGroupCount = groups.length;
            
            // Get visible groups from settings
            const visibleGroupIds = this._settings.get_strv('visible-groups');
            console.log('[keylightd-control] Visible group IDs:', visibleGroupIds);
            
            // Filter groups by visibility setting
            let visibleGroups = [];
            
            if (visibleGroupIds.length > 0) {
                // Show only groups that are explicitly marked as visible
                visibleGroups = groups.filter(group => visibleGroupIds.includes(group.id));
                console.log('[keylightd-control] Filtered to visible groups:', visibleGroups.map(g => g.name));
            } else {
                // If no visibility set yet, show all groups
                visibleGroups = groups;
                console.log('[keylightd-control] Showing all groups, no visibility filter set');
            }
            
            visibleGroupCount = visibleGroups.length;
            
            // First, remove all sections that aren't in visibleGroups anymore
            for (const [name, section] of existingSections.entries()) {
                const groupStillExists = visibleGroups.some(g => g.name === name);
                if (!groupStillExists) {
                    this._lightsContainer.remove_child(section);
                    existingSections.delete(name);
                }
            }
            
            // Add/Update groups
            if (visibleGroups && visibleGroups.length > 0) {
                // Add title showing visible group count
                const groupCountLabel = new St.Label({
                    text: _('%d of %d groups shown').format(visibleGroupCount, totalGroupCount),
                    style_class: 'keylightd-group-count',
                    x_align: Clutter.ActorAlign.CENTER,
                    y_align: Clutter.ActorAlign.CENTER,
                    style: 'font-size: 0.9em; font-style: italic; padding: 4px; color: rgba(255,255,255,0.7);'
                });
                this._lightsContainer.add_child(groupCountLabel);
                
                // Add each group with controls
                for (const group of visibleGroups) {
                    try {
                        // Use the group endpoint to get group details instead of a non-existent /status endpoint
                        const groupDetails = await fetchAPI(`groups/${group.id}`);
                        
                        // Determine if the group is on by checking if any of its lights are on
                        let isOn = false;
                        let brightness = 50;  // Default values
                        let temperature = 200;
                        
                        // If there are lights in the group, check their status
                        if (groupDetails.lights && groupDetails.lights.length > 0) {
                            try {
                                // Try to fetch one light to determine group status
                                const lightId = groupDetails.lights[0];
                                const lightDetails = await fetchAPI(`lights/${lightId}`);
                                
                                isOn = lightDetails.on === true;
                                brightness = lightDetails.brightness || 50;
                                temperature = lightDetails.temperature || 200;
                            } catch (lightError) {
                                console.error(`[keylightd-control] Error getting light status: ${lightError}`);
                            }
                        }
                        
                        // If any light is on, update the main icon
                        if (isOn) {
                            anyLightOn = true;
                        }
                        
                        // Check if we have an existing section for this group
                        let controlSection;
                        if (existingSections.has(group.name)) {
                            // Update the existing section
                            controlSection = existingSections.get(group.name);
                            this._updateControlSection(controlSection, {
                                on: isOn,
                                brightness: brightness,
                                temperature: temperature
                            });
                            
                            // Remove from map to track which ones we've processed
                            existingSections.delete(group.name);
                        } else {
                            // Create a new control section
                            controlSection = this._createLightControlSection(
                                group.name,
                                isOn,
                                brightness,
                                temperature,
                                (state) => this._toggleGroupState(group.id, state),
                                (value) => this._setGroupBrightness(group.id, value),
                                (value) => this._setGroupTemperature(group.id, value)
                            );
                            this._lightsContainer.add_child(controlSection);
                        }
                        
                        // Add separator except for the last item
                        if (visibleGroups.indexOf(group) < visibleGroups.length - 1) {
                            const separator = new St.BoxLayout({
                                style_class: 'popup-separator-menu-item-separator',
                                x_expand: true,
                                y_expand: false,
                                style: 'margin: 1px 0;'
                            });
                            this._lightsContainer.add_child(separator);
                        }
                    } catch (error) {
                        console.error(`[keylightd-control] Error loading group details: ${error}`);
                    }
                }
            } else {
                // If no groups, load individual lights
                const lights = await fetchAPI('lights');
                
                if (lights && lights.length > 0) {
                    for (const light of lights) {
                        try {
                            // Get light status
                            const lightData = await fetchAPI(`lights/${light.id}`);
                            const isOn = lightData.on === true;
                            
                            // If any light is on, update the main icon
                            if (isOn) {
                                anyLightOn = true;
                            }
                            
                            // Check if we have an existing section for this light
                            let controlSection;
                            if (existingSections.has(light.name)) {
                                // Update the existing section
                                controlSection = existingSections.get(light.name);
                                this._updateControlSection(controlSection, {
                                    on: isOn,
                                    brightness: lightData.brightness || 50,
                                    temperature: lightData.temperature || 200
                                });
                                
                                // Remove from map to track which ones we've processed
                                existingSections.delete(light.name);
                            } else {
                                // Create a new control section
                                controlSection = this._createLightControlSection(
                                    light.name,
                                    isOn,
                                    lightData.brightness || 50,
                                    lightData.temperature || 200,
                                    (state) => this._setLightState(light.id, state),
                                    (value) => this._setLightBrightness(light.id, value),
                                    (value) => this._setLightTemperature(light.id, value)
                                );
                                this._lightsContainer.add_child(controlSection);
                            }
                            
                            // Add separator except for the last item
                            if (lights.indexOf(light) < lights.length - 1) {
                                const separator = new St.BoxLayout({
                                    style_class: 'popup-separator-menu-item-separator',
                                    x_expand: true,
                                    y_expand: false,
                                    style: 'margin: 1px 0;'
                                });
                                this._lightsContainer.add_child(separator);
                            }
                        } catch (error) {
                            console.error(`[keylightd-control] Error loading light status: ${error}`);
                        }
                    }
                } else {
                    // No groups or lights found
                    const noLightsLabel = new St.Label({
                        text: _('No lights or groups configured'),
                        style_class: 'keylightd-no-lights-label'
                    });
                    this._lightsContainer.add_child(noLightsLabel);
                }
            }
            
            // Update the main icon based on light status
            this._lightState = anyLightOn ? lightStates.ENABLED : lightStates.DISABLED;
            this._updateIcon(this._lightState);
            
        } catch (error) {
            console.error(`[keylightd-control] Error loading controls: ${error}`);
            
            // Show error message
            const errorLabel = new St.Label({
                text: _('Error loading lights'),
                style_class: 'keylightd-error-label'
            });
            this._lightsContainer.add_child(errorLabel);
            
            // Update icon to unknown state on error
            this._lightState = lightStates.UNKNOWN;
            this._updateIcon(this._lightState);
        }
        
        // Remove any remaining sections that weren't reused
        for (const section of existingSections.values()) {
            this._lightsContainer.remove_child(section);
        }
        
        // Force a relayout to ensure sliders are sized correctly
        this._lightsContainer.queue_relayout();
    }
    
    _createLightControlSection(name, isOn, brightness, temperature, toggleCallback, brightnessCallback, temperatureCallback) {
        // Create container for this light/group
        const container = new St.BoxLayout({
            vertical: true,
            style_class: 'keylightd-control-section',
            style: 'margin: 2px 0; padding: 2px;'
        });
        
        // Header row with name and power toggle
        const headerBox = new St.BoxLayout({
            style_class: 'keylightd-header-box',
            x_expand: true,
            style: 'margin-bottom: 2px;'
        });
        
        // Light/group name
        const nameLabel = new St.Label({
            text: name,
            style_class: 'keylightd-light-name',
            x_expand: true,
            y_align: Clutter.ActorAlign.CENTER
        });
        
        // Store the current state in the button's user_data so we can access it when clicked
        const powerButton = new St.Button({
            style_class: isOn 
                ? 'keylightd-power-button keylightd-power-on' 
                : 'keylightd-power-button keylightd-power-off',
            child: new St.Icon({
                icon_name: 'system-shutdown-symbolic',
                style_class: 'popup-menu-icon',
                icon_size: 14
            }),
            x_expand: false
        });
        
        // Store the current state for reference
        powerButton.isOn = isOn;
        
        powerButton.connect('clicked', () => {
            // Toggle the opposite of the current state
            const newState = !powerButton.isOn;
            toggleCallback(newState);
            
            // Update the button's internal state
            powerButton.isOn = newState;
            
            // Update the button's appearance immediately for better responsiveness 
            if (newState) {
                powerButton.remove_style_class_name('keylightd-power-off');
                powerButton.add_style_class_name('keylightd-power-on');
            } else {
                powerButton.remove_style_class_name('keylightd-power-on');
                powerButton.add_style_class_name('keylightd-power-off');
            }
        });
        
        headerBox.add_child(nameLabel);
        headerBox.add_child(powerButton);
        
        container.add_child(headerBox);
        
        // Brightness slider row
        container.add_child(this._createSliderRow(
            'display-brightness-symbolic',
            brightness,
            brightnessCallback
        ));
        
        // Temperature slider row
        container.add_child(this._createSliderRow(
            'color-select-symbolic',
            temperature,
            temperatureCallback,
            true
        ));
        
        return container;
    }
    
    _createSliderRow(iconName, value, callback, isTemperature = false) {
        const container = new St.BoxLayout({
            style_class: 'keylightd-slider-row',
            x_expand: true,
            y_align: Clutter.ActorAlign.CENTER,
            style: 'spacing: 4px; padding: 2px 4px;'
        });
        
        // Icon on the left
        const icon = new St.Icon({
            icon_name: iconName,
            style_class: 'popup-menu-icon keylightd-slider-icon',
            icon_size: 14,
        });
        
        // Slider
        const sliderValue = isTemperature 
            ? this._normalizeTemperatureValue(value) 
            : value / 100;
        
        // Create a container for the slider to ensure consistent sizing
        const sliderContainer = new St.Bin({
            style_class: 'keylightd-slider-container',
            x_expand: true,
            y_expand: false,
            x_align: Clutter.ActorAlign.FILL,
            y_align: Clutter.ActorAlign.CENTER,
        });
        
        // Create and configure the slider
        const slider = new Slider(sliderValue);
        slider.accessible_name = isTemperature ? 'Temperature' : 'Brightness';
        slider.add_style_class_name('keylightd-slider');
        slider.add_style_pseudo_class('active');
        slider.x_expand = true;
        
        // Add the slider to its container
        sliderContainer.set_child(slider);
        
        // Value label on the right (shows percentage or Kelvin)
        const valueLabel = new St.Label({
            text: isTemperature 
                ? `${convertDeviceToKelvin(value)}K` 
                : `${Math.round(value)}%`,
            style_class: 'keylightd-value-label',
            x_align: Clutter.ActorAlign.END,
            width: 55,
            style: 'min-width: 55px; width: 55px; max-width: 55px; text-align: end; font-size: 0.9em;'
        });
        
        // Get the debounce delay from settings
        const debounceDelay = this._settings.get_int('debounce-delay');
        
        // Create debounced version of the callback
        const debouncedCallback = debounce((newValue) => {
            callback(newValue);
        }, debounceDelay);
        
        slider.connect('notify::value', () => {
            let newValue;
            if (isTemperature) {
                // Map 0-1 to mireds range (MIN_MIREDS to MAX_MIREDS)
                newValue = Math.round(MIN_MIREDS + (slider.value * (MAX_MIREDS - MIN_MIREDS)));
                // Update the label with Kelvin value immediately
                valueLabel.text = `${convertDeviceToKelvin(newValue)}K`;
            } else {
                // Brightness is 0-100
                newValue = Math.round(slider.value * 100);
                // Update the percentage label immediately
                valueLabel.text = `${newValue}%`;
            }
            // Use the debounced callback for the API call
            debouncedCallback(newValue);
        });
        
        container.add_child(icon);
        container.add_child(sliderContainer);
        container.add_child(valueLabel);
        
        return container;
    }
    
    _normalizeTemperatureValue(temp) {
        // Map temperature (MIN_MIREDS to MAX_MIREDS) to slider range (0-1)
        return (temp - MIN_MIREDS) / (MAX_MIREDS - MIN_MIREDS);
    }
    
    async _toggleGroupState(groupId, state) {
        try {
            console.log(`[keylightd-control] Toggling group ${groupId} to ${state}`);
            
            // Wait for the API call to complete
            const response = await fetchAPI(
                `groups/${groupId}/state`,
                'PUT',
                { on: state }
            );
            
            console.log(`[keylightd-control] API response for toggling group ${groupId}:`, response);
            
            // Force a full update to ensure we get the current state
            this._loadControls();
        } catch (error) {
            console.error(`[keylightd-control] Error toggling group: ${error}`);
            // Refresh to show actual state from server
            this._loadControls();
        }
    }
    
    async _setGroupBrightness(groupId, brightness) {
        try {
            console.log(`[keylightd-control] Setting group ${groupId} brightness to ${brightness}`);
            
            // Wait for the API call to complete
            const response = await fetchAPI(
                `groups/${groupId}/state`,
                'PUT',
                { brightness: brightness }
            );
            
            console.log(`[keylightd-control] API response for setting group ${groupId} brightness:`, response);
            
            // Update UI only after successful API call
            this._updateGroupUI(groupId, { brightness: brightness });
        } catch (error) {
            console.error(`[keylightd-control] Error setting group brightness: ${error}`);
            // Refresh to show actual state from server
            this._loadControls();
        }
    }
    
    async _setGroupTemperature(groupId, temperature) {
        try {
            // Convert from device temperature (mireds) to Kelvin for the API
            const kelvinTemp = convertDeviceToKelvin(temperature);
            console.log(`[keylightd-control] Setting group ${groupId} temperature to ${kelvinTemp}K (${temperature} mireds)`);
            
            // Wait for the API call to complete
            const response = await fetchAPI(
                `groups/${groupId}/state`,
                'PUT',
                { temperature: kelvinTemp }
            );
            
            console.log(`[keylightd-control] API response for setting group ${groupId} temperature:`, response);
            
            // Update UI only after successful API call
            this._updateGroupUI(groupId, { temperature: temperature });
        } catch (error) {
            console.error(`[keylightd-control] Error setting group temperature: ${error}`);
            // Refresh to show actual state from server
            this._loadControls();
        }
    }
    
    async _setLightState(lightId, state) {
        try {
            console.log(`[keylightd-control] Setting light ${lightId} state to ${state}`);
            
            // Wait for the API call to complete
            const response = await fetchAPI(
                `lights/${lightId}/state`,
                'POST',
                { on: state }
            );
            
            console.log(`[keylightd-control] API response for setting light ${lightId} state:`, response);
            
            // Force a full update to ensure we get the current state
            this._loadControls();
        } catch (error) {
            console.error(`[keylightd-control] Error setting light state: ${error}`);
            // Refresh to show actual state from server
            this._loadControls();
        }
    }
    
    async _setLightBrightness(lightId, brightness) {
        try {
            console.log(`[keylightd-control] Setting light ${lightId} brightness to ${brightness}`);
            
            // Wait for the API call to complete
            const response = await fetchAPI(
                `lights/${lightId}/state`,
                'POST',
                { brightness: brightness }
            );
            
            console.log(`[keylightd-control] API response for setting light ${lightId} brightness:`, response);
            
            // Update UI only after successful API call
            this._updateLightUI(lightId, { brightness: brightness });
        } catch (error) {
            console.error(`[keylightd-control] Error setting light brightness: ${error}`);
            // Refresh to show actual state from server
            this._loadControls();
        }
    }
    
    async _setLightTemperature(lightId, temperature) {
        try {
            // Convert from device temperature (mireds) to Kelvin for the API
            const kelvinTemp = convertDeviceToKelvin(temperature);
            console.log(`[keylightd-control] Setting light ${lightId} temperature to ${kelvinTemp}K (${temperature} mireds)`);
            
            // Wait for the API call to complete
            const response = await fetchAPI(
                `lights/${lightId}/state`,
                'POST',
                { temperature: kelvinTemp }
            );
            
            console.log(`[keylightd-control] API response for setting light ${lightId} temperature:`, response);
            
            // Update UI only after successful API call
            this._updateLightUI(lightId, { temperature: temperature });
        } catch (error) {
            console.error(`[keylightd-control] Error setting light temperature: ${error}`);
            // Refresh to show actual state from server
            this._loadControls();
        }
    }
    
    // Update UI for a specific group without full refresh
    _updateGroupUI(groupId, updates) {
        // First try to find the section by name in the cached UI elements
        let groupName = null;
        let controlSection = null;
        
        // Get all visible groups to find the group's name
        fetchAPI('groups').then(groups => {
            if (!groups || !Array.isArray(groups)) {
                console.log('[keylightd-control] Could not fetch groups for updating UI');
                return;
            }
            
            // Find the group with the matching ID
            const group = groups.find(g => g.id === groupId);
            if (!group) {
                console.log(`[keylightd-control] Could not find group with ID ${groupId}`);
                return;
            }
            
            groupName = group.name;
            
            // Now find the control section with this name
            let child = this._lightsContainer.get_first_child();
            while (child) {
                // Skip labels and separators, look only for control sections
                if (!(child instanceof St.BoxLayout) || !child.get_first_child()) {
                    child = child.get_next_sibling();
                    continue;
                }
                
                // Check the name label
                const headerBox = child.get_first_child();
                if (headerBox && headerBox.get_first_child()) {
                    const nameLabel = headerBox.get_first_child();
                    if (nameLabel instanceof St.Label && nameLabel.text === groupName) {
                        controlSection = child;
                        break;
                    }
                }
                
                child = child.get_next_sibling();
            }
            
            if (!controlSection) {
                console.log(`[keylightd-control] Could not find control section for group ${groupName}, doing full refresh`);
                this._loadControls();
                return;
            }
            
            // Update the section with the new values
            this._updateControlSection(controlSection, updates);
            
        }).catch(error => {
            console.error(`[keylightd-control] Error updating group UI: ${error}`);
            // Don't trigger a full refresh on error
        });
    }
    
    // Update UI for a specific light without full refresh
    _updateLightUI(lightId, updates) {
        // First try to find the light's name in the cached UI elements
        fetchAPI(`lights/${lightId}`).then(light => {
            if (!light || !light.name) {
                console.log(`[keylightd-control] Could not fetch light with ID ${lightId}`);
                return;
            }
            
            const lightName = light.name;
            let controlSection = null;
            
            // Find the control section with this name
            let child = this._lightsContainer.get_first_child();
            while (child) {
                // Skip labels and separators, look only for control sections
                if (!(child instanceof St.BoxLayout) || !child.get_first_child()) {
                    child = child.get_next_sibling();
                    continue;
                }
                
                // Check the name label
                const headerBox = child.get_first_child();
                if (headerBox && headerBox.get_first_child()) {
                    const nameLabel = headerBox.get_first_child();
                    if (nameLabel instanceof St.Label && nameLabel.text === lightName) {
                        controlSection = child;
                        break;
                    }
                }
                
                child = child.get_next_sibling();
            }
            
            if (!controlSection) {
                console.log(`[keylightd-control] Could not find control section for light ${lightName}, doing full refresh`);
                this._loadControls();
                return;
            }
            
            // Update the section with the new values
            this._updateControlSection(controlSection, updates);
            
        }).catch(error => {
            console.error(`[keylightd-control] Error updating light UI: ${error}`);
            // Don't trigger a full refresh on error
        });
    }
    
    // Update a specific control section with new values
    _updateControlSection(section, updates) {
        try {
            // Find the power button
            if ('on' in updates) {
                const headerBox = section.get_first_child();
                if (headerBox) {
                    const powerButton = headerBox.get_last_child();
                    if (powerButton && powerButton instanceof St.Button) {
                        // Update the power button style
                        if (updates.on) {
                            powerButton.remove_style_class_name('keylightd-power-off');
                            powerButton.add_style_class_name('keylightd-power-on');
                        } else {
                            powerButton.remove_style_class_name('keylightd-power-on');
                            powerButton.add_style_class_name('keylightd-power-off');
                        }
                    }
                }
            }
            
            // Find and update the brightness slider if needed
            if ('brightness' in updates) {
                const brightnessRow = section.get_child_at_index(1);
                if (brightnessRow) {
                    const sliderBin = brightnessRow.get_child_at_index(1);
                    if (sliderBin) {
                        const slider = sliderBin.get_child();
                        if (slider) {
                            slider.value = updates.brightness / 100;
                        }
                    }
                    
                    const valueLabel = brightnessRow.get_last_child();
                    if (valueLabel && valueLabel instanceof St.Label) {
                        valueLabel.text = `${Math.round(updates.brightness)}%`;
                    }
                }
            }
            
            // Find and update the temperature slider if needed
            if ('temperature' in updates) {
                const temperatureRow = section.get_child_at_index(2);
                if (temperatureRow) {
                    const sliderBin = temperatureRow.get_child_at_index(1);
                    if (sliderBin) {
                        const slider = sliderBin.get_child();
                        if (slider) {
                            slider.value = this._normalizeTemperatureValue(updates.temperature);
                        }
                    }
                    
                    const valueLabel = temperatureRow.get_last_child();
                    if (valueLabel && valueLabel instanceof St.Label) {
                        valueLabel.text = `${convertDeviceToKelvin(updates.temperature)}K`;
                    }
                }
            }
        } catch (error) {
            console.error('[keylightd-control] Error updating control section:', error);
        }
    }

    _getIcon(lightState) {
        this._iconUnknown = Gio.icon_new_for_string(`${this._path}${ActionsPath}${UnknownIcon}.svg`);
        this._iconActivated = Gio.icon_new_for_string(`${this._path}${ActionsPath}${EnabledIcon}.svg`);
        this._iconDeactivated = Gio.icon_new_for_string(`${this._path}${ActionsPath}${DisabledIcon}.svg`);

        switch (lightState) {
            case lightStates.ENABLED:
                return this._iconActivated;
            case lightStates.DISABLED:
                return this._iconDeactivated;
            case lightStates.UNKNOWN:
            default:
                return this._iconUnknown;
       }
    }

    _updateIcon(lightState) {
        this.gicon = this._getIcon(lightState);
    }

    async _toggleLights() {
        try {
            // Get visible groups
            const groups = await fetchAPI('groups');
            const visibleGroupIds = this._settings.get_strv('visible-groups');
            
            if (groups && groups.length > 0 && visibleGroupIds.length > 0) {
                // Filter to visible groups
                const visibleGroups = groups.filter(group => visibleGroupIds.includes(group.id));
                
                if (visibleGroups.length > 0) {
                    // Check if any light in any visible group is on
                    let anyLightOn = false;
                    
                    for (const group of visibleGroups) {
                        // Get group details
                        const groupDetails = await fetchAPI(`groups/${group.id}`);
                        
                        // Check if any light in this group is on
                        if (groupDetails.lights && groupDetails.lights.length > 0) {
                            try {
                                const lightId = groupDetails.lights[0];
                                const lightDetails = await fetchAPI(`lights/${lightId}`);
                                
                                if (lightDetails.on === true) {
                                    anyLightOn = true;
                                    break; // Found at least one light that's on
                                }
                            } catch (lightError) {
                                console.error(`[keylightd-control] Error checking light status: ${lightError}`);
                                continue;
                            }
                        }
                    }
                    
                    // Toggle to opposite state - turn all off if any is on, or all on if all are off
                    const newState = !anyLightOn;
                    console.log(`[keylightd-control] Toggling all visible groups to ${newState}`);
                    
                    // Apply to all visible groups concurrently
                    const promises = visibleGroups.map(group => 
                        fetchAPI(`groups/${group.id}/state`, 'PUT', { on: newState })
                    );
                    
                    // Wait for all API calls to complete
                    await Promise.all(promises);
                    console.log('[keylightd-control] All group toggle operations completed successfully');
                    
                    // Refresh to show updated states
                    this._loadControls();
                    return;
                }
            }
            
            // Fallback to toggling individual lights if no groups are available/visible
            const lights = await fetchAPI('lights');
            
            if (lights && lights.length > 0) {
                // Get the first light's state to determine the toggle action
                const firstLight = await fetchAPI(`lights/${lights[0].id}`);
                const newState = !(firstLight.on === true); // Toggle to opposite state
                
                console.log(`[keylightd-control] Toggling all lights to ${newState}`);
                
                // Apply to all lights
                const promises = lights.map(light => 
                    fetchAPI(`lights/${light.id}/state`, 'POST', { on: newState })
                );
                
                // Wait for all API calls to complete
                await Promise.all(promises);
                console.log('[keylightd-control] All light toggle operations completed successfully');
                
                // Refresh to show updated states
                this._loadControls();
            }
        } catch (error) {
            console.error(`[keylightd-control] Error toggling lights/groups: ${error}`);
            // Refresh to ensure accurate state
            this._loadControls();
        }
    }
});

const KeylightdControl = GObject.registerClass(
    class KeylightdControl extends QuickSettings.SystemIndicator {
        _init(Me) {
            super._init();
            
            this._appSystem = Shell.AppSystem.get_default(); // support window detection triggers
            this._settings = Me._settings;
            this._name = Me.metadata.name;
            this._indicator = this._addIndicator();

            this._keylightdToggle = new KeylightdControlToggle(Me);
            this._indicator.gicon = this._keylightdToggle._getIcon(lightStates.UNKNOWN);
            
            // Connect to toggle's notify::gicon signal to update the indicator icon
            this._keylightdToggle.connect('notify::gicon', () => {
                this._indicator.gicon = this._keylightdToggle.gicon;
            });
            
            this.quickSettingsItems.push(this._keylightdToggle);

            // Add indicator and toggle
            QuickSettingsMenu.addExternalIndicator(this);
            if (ShellVersion >= 46) {
                QuickSettingsMenu._indicators.remove_child(this);
            } else {
                QuickSettingsMenu._indicators.remove_actor(this);
            }
            QuickSettingsMenu._indicators.insert_child_at_index(this, this.indicatorIndex);

        }

        destroy() {
            this.quickSettingsItems.forEach((item) => item.destroy());
            this._keylightdToggle = null;
            this._indicator = null;
            this._appSystem = null;
            this._settings = null;
            this._name = null;
            super.destroy();
        }
});

export default class KeylightdControlExtension extends Extension {
    enable() {
        log('[keylightd-control] enable() called');
        this._settings = this.getSettings();
        
        // Set the settings in the utils module
        setSettings(this._settings);
        
        this._keylightdIndicator = new KeylightdControl(this);
        log('[keylightd-control] enable() finished');
    }

    disable() {
        log('[keylightd-control] disable() called');
        
        // Clear the settings reference in utils
        setSettings(null);
        
        this._settings = null;
        this._keylightdIndicator.destroy();
        this._keylightdIndicator = null;
        log('[keylightd-control] disable() finished');
    }

    _openPreferences() {
        this.openPreferences();
        QuickSettingsMenu.menu.close(PopupAnimation.FADE);
    }
}