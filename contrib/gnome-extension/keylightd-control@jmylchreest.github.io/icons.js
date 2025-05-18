'use strict';

import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import { log } from './utils.js';
import St from 'gi://St';

// Icon filenames
export const ICON_ENABLED = 'light-enabled.svg';
export const ICON_DISABLED = 'light-disabled.svg';
export const ICON_UNKNOWN = 'light-unknown.svg';

// Relative path to actions icons (from extension root)
export const ACTIONS_PATH = '/icons/hicolor/scalable/actions/';

// System icon names
export const SYSTEM_ICON_POWER = 'system-shutdown-symbolic';
export const FALLBACK_SYSTEM_ICON = 'dialog-question-symbolic';

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
    log('debug', `icons.js initialized with extensionPath: ${_extensionPath}`);
}

/**
 * Get the full path for an icon
 * @param {string} iconFile - The icon filename
 * @returns {string} - Full path to the icon SVG
 */
export function getIconPath(iconFile) {
    if (!_extensionPath) {
        log('error', 'getIconPath called before icons.js was initialized!');
        return '';
    }
    const path = `${_extensionPath}${ACTIONS_PATH}${iconFile}`;
    log('debug', `Icon path: ${path}`);
    return path;
}

/**
 * Check if an icon file exists
 * @param {string} iconPath - Full path to the icon
 * @returns {boolean}
 */
export function iconExists(iconPath) {
    const exists = GLib.file_test(iconPath, GLib.FileTest.EXISTS);
    log('debug', `Icon exists at ${iconPath}: ${exists}`);
    return exists;
}

/**
 * Get a GIcon for a light state, with debug logging and fallback
 * @param {string} state - 'ENABLED', 'DISABLED', or 'UNKNOWN'
 * @returns {Gio.Icon}
 */
export function getLightStateIcon(state) {
    if (!_extensionPath) {
        log('error', 'getLightStateIcon called before icons.js was initialized!');
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
            log('debug', `Loaded icon for state ${state}: ${iconPath}`);
            return icon;
        } catch (e) {
            log('warn', `Failed to load icon for state ${state} at ${iconPath}: ${e}`);
        }
    } else {
        log('warn', `Icon file for state ${state} does not exist: ${iconPath}`);
    }
    // Fallback to a system icon
    log('warn', `Falling back to system icon for state ${state}`);
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
 * @returns {Gio.Icon|St.Icon}
 */
export function getIcon(iconName, params = {}) {
    const cacheKey = iconName + JSON.stringify(params);
    if (_iconCache.has(cacheKey)) {
        return _iconCache.get(cacheKey);
    }

    // 1. Try to resolve as a custom icon first
    const customIconPath = getIconPath(iconName);
    if (iconExists(customIconPath)) {
        try {
            const icon = Gio.icon_new_for_string(customIconPath);
            log('debug', `Using cached custom icon: ${customIconPath}`);
            _iconCache.set(cacheKey, icon);
            return icon;
        } catch (e) {
            log('warn', `Failed to load custom icon at ${customIconPath}: ${e}`);
        }
    }

    // 2. Try to resolve as a system themed icon
    try {
        let icon;
        if (params.forGIcon === true) {
            icon = Gio.ThemedIcon.new(iconName);
        } else {
            icon = new St.Icon({
                icon_name: iconName,
                ...params
            });
        }
        _iconCache.set(cacheKey, icon);
        return icon;
    } catch (error) {
        log('warn', `Error creating system icon ${iconName}:`, error);
    }

    // 3. Fallback to the fallback system icon
    log('warn', `Falling back to fallback system icon for ${iconName}`);
    let fallbackIcon;
    if (params.forGIcon === true) {
        fallbackIcon = Gio.ThemedIcon.new(FALLBACK_SYSTEM_ICON);
    } else {
        fallbackIcon = new St.Icon({
            icon_name: FALLBACK_SYSTEM_ICON,
            style_class: params.style_class || 'popup-menu-icon',
            icon_size: params.icon_size || 16
        });
    }
    _iconCache.set(cacheKey, fallbackIcon);
    return fallbackIcon;
} 