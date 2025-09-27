'use strict';

import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import { filteredLog } from './utils.js';
import St from 'gi://St';
import { ICON_ENABLED, ICON_DISABLED, ICON_UNKNOWN, SYSTEM_ICON_POWER, SYSTEM_ICON_BRIGHTNESS, SYSTEM_ICON_TEMPERATURE, FALLBACK_SYSTEM_ICON, SYSTEM_PREFS_GENERAL_ICON, SYSTEM_PREFS_GROUPS_ICON, SYSTEM_PREFS_LIGHTS_ICON, SYSTEM_PREFS_UI_ICON } from './icon-names.js';

// Relative path to actions icons (from extension root)
export const ACTIONS_PATH = '/icons/hicolor/scalable/actions/';

// Light states for semantic use
export const LightState = {
    ENABLED: 'ENABLED',
    DISABLED: 'DISABLED',
    UNKNOWN: 'UNKNOWN'
};

let _extensionPath = null;
const _iconCache = new Map();

/**
 * Initialize the icons module with the extension path
 * @param {string} extensionPath - The extension base path
 */
export function initIcons(extensionPath) {
    _extensionPath = extensionPath;
    filteredLog('debug', `icons.js initialized with extensionPath: ${_extensionPath}`);
}

/**
 * Get the full path for an icon
 * @param {string} iconFile - The icon filename
 * @returns {string} - Full path to the icon SVG
 */
export function getIconPath(iconFile) {
    if (!_extensionPath) {
        filteredLog('error', 'getIconPath called before icons.js was initialized!');
        return '';
    }
    // Always append .svg if not present
    if (!iconFile.endsWith('.svg')) {
        iconFile += '.svg';
    }
    const path = `${_extensionPath}${ACTIONS_PATH}${iconFile}`;
    filteredLog('debug', `Icon path: ${path}`);
    return path;
}

/**
 * Check if an icon file exists
 * @param {string} iconPath - Full path to the icon
 * @returns {boolean}
 */
export function iconExists(iconPath) {
    const exists = GLib.file_test(iconPath, GLib.FileTest.EXISTS);
    filteredLog('debug', `Icon exists at ${iconPath}: ${exists}`);
    return exists;
}

/**
 * Get a GIcon for a light state, with debug logging and fallback
 * @param {string} state - 'ENABLED', 'DISABLED', or 'UNKNOWN'
 * @returns {Gio.Icon}
 */
export function getLightStateIcon(state) {
    if (!_extensionPath) {
        filteredLog('error', 'getLightStateIcon called before icons.js was initialized!');
        return Gio.ThemedIcon.new(FALLBACK_SYSTEM_ICON);
    }
    let iconFile;
    switch (state) {
        case LightState.ENABLED:
            iconFile = ICON_ENABLED;
            break;
        case LightState.DISABLED:
            iconFile = ICON_DISABLED;
            break;
        case LightState.UNKNOWN:
        default:
            iconFile = ICON_UNKNOWN;
            break;
    }
    const iconPath = getIconPath(iconFile);
    if (iconExists(iconPath)) {
        try {
            const icon = Gio.icon_new_for_string(iconPath);
            return icon;
        } catch (e) {
            filteredLog('warn', `Failed to load icon for state ${state} at ${iconPath}: ${e}`);
        }
    } else {
        filteredLog('warn', `Icon file for state ${state} does not exist: ${iconPath}`);
    }
    // Fallback to a system icon
    filteredLog('warn', `Falling back to system icon for state ${state}`);
    return Gio.ThemedIcon.new(FALLBACK_SYSTEM_ICON);
}

/**
 * Determine the light state based on visible lights and groups
 * @param {Map} lights - Map of light IDs to light states
 * @param {Map} groups - Map of group IDs to group states
 * @param {string[]} visibleLightIds - Array of visible light IDs
 * @param {string[]} visibleGroupIds - Array of visible group IDs
 * @returns {string} - Light state ('ENABLED', 'DISABLED', or 'UNKNOWN')
 */
export function determineLightState(lights, groups, visibleLightIds, visibleGroupIds) {
    // If no data is provided, return UNKNOWN
    if (!lights || !groups || !visibleLightIds || !visibleGroupIds) {
        return LightState.UNKNOWN;
    }

    // If no visible lights or groups, return UNKNOWN
    if (visibleLightIds.length === 0 && visibleGroupIds.length === 0) {
        return LightState.UNKNOWN;
    }

    let anyOn = false;
    let totalChecked = 0;

    // Check visible lights
    for (const lightId of visibleLightIds) {
        const light = lights.get(lightId);
        if (light) {
            totalChecked++;
            if (light.on === true) {
                anyOn = true;
                break;
            }
        }
    }

    // Only check groups if we haven't found an 'on' light yet
    if (!anyOn) {
        for (const groupId of visibleGroupIds) {
            const group = groups.get(groupId);
            if (group && group.lights && group.lights.length > 0) {
                for (const lightId of group.lights) {
                    const light = lights.get(lightId);
                    if (light) {
                        totalChecked++;
                        if (light.on === true) {
                            anyOn = true;
                            break;
                        }
                    }
                }
                if (anyOn) break;
            }
        }
    }

    // If we checked at least one light/group, return appropriate state
    if (totalChecked > 0) {
        return anyOn ? LightState.ENABLED : LightState.DISABLED;
    }

    // If no lights/groups were checked, return UNKNOWN
    return LightState.UNKNOWN;
}

/**
 * Get a custom or system icon by name
 * @param {string} iconName - The icon name (custom or system)
 * @param {object} [params] - Optional parameters for St.Icon
 * @returns {St.Icon}
 */
export function getIcon(iconName, params = {}) {
    if (!iconName || typeof iconName !== 'string') {
        filteredLog('warn', `Invalid icon name: ${iconName}`);
        iconName = FALLBACK_SYSTEM_ICON;
    }

    // Prepare icon properties
    const iconProps = {
        style_class: params.style_class || 'popup-menu-icon',
        icon_size: params.icon_size || 16
    };
    if (params.x_align !== undefined) iconProps.x_align = params.x_align;
    if (params.y_align !== undefined) iconProps.y_align = params.y_align;

    // Always check for a custom icon first
    try {
        const customIconPath = getIconPath(iconName);
        if (iconExists(customIconPath)) {
            const gicon = Gio.icon_new_for_string(customIconPath);
            iconProps.gicon = gicon;
            filteredLog('debug', `Loaded custom icon as gicon: ${customIconPath}`);
            return new St.Icon(iconProps);
        }
    } catch (e) {
        filteredLog('warn', `Failed to load custom icon for ${iconName}: ${e}`);
    }

    // Fallback to system/themed icon
    iconProps.icon_name = iconName;
    filteredLog('debug', `getIcon(): using system/themed icon_name: ${iconName}`);
    return new St.Icon(iconProps);
} 