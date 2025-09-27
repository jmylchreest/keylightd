"use strict";

import St from "gi://St";
import Clutter from "gi://Clutter";
import * as PopupMenu from "resource:///org/gnome/shell/ui/popupMenu.js";
import { Slider } from "resource:///org/gnome/shell/ui/slider.js";
import { filteredLog } from "../utils.js";
import {
  SYSTEM_ICON_POWER,
  SYSTEM_ICON_BRIGHTNESS,
  SYSTEM_ICON_TEMPERATURE,
} from "../icon-names.js";
import { getIcon } from "../icons.js";
import GLib from "gi://GLib";

// Debounce function to limit function calls
function debounce(func, delay, timeoutTracker = null) {
  let timeoutId;
  return function (...args) {
    const context = this;
    if (timeoutId) {
      GLib.source_remove(timeoutId);
      if (timeoutTracker) {
        timeoutTracker.delete(timeoutId);
      }
      timeoutId = null;
    }
    timeoutId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, delay, () => {
      func.apply(context, args);
      if (timeoutTracker) {
        timeoutTracker.delete(timeoutId);
      }
      timeoutId = null;
      return GLib.SOURCE_REMOVE;
    });
    if (timeoutTracker) {
      timeoutTracker.add(timeoutId);
    }
  };
}

/**
 * Slider configuration presets
 */
const SLIDER_CONFIGS = {
  brightness: {
    min: 0,
    max: 100,
    format: (value) => `${Math.round(value)}%`,
    normalize: (value) => value / 100,
    denormalize: (value) => value * 100,
  },
  temperature: {
    min: 2900,
    max: 7000,
    format: (value) => `${Math.round(value)}K`,
    normalize: (value) => (value - 2900) / (7000 - 2900),
    denormalize: (value) => 2900 + value * (7000 - 2900),
  },
};

export class UIBuilder {
  constructor(extensionPath, settings) {
    this._extensionPath = extensionPath;
    this._settings = settings;
    this._iconTheme = null;
    this._cachedIcons = new Map();
    this._timeoutIds = new Set();
    this._initIconTheme();
  }

  /**
   * Initialize the icon theme with extension icons
   */
  _initIconTheme() {
    try {
      // First try to get the theme from the stage
      const stage = global.stage;
      if (stage) {
        const themeContext = St.ThemeContext.get_for_stage(stage);
        if (themeContext) {
          this._iconTheme = themeContext.get_theme();
          filteredLog("debug", "Initialized icon theme from stage theme context");
        }
      }

      // Skip Gtk/Gdk imports as they're not allowed in GNOME Shell process

      // Last resort - try to get theme directly
      if (!this._iconTheme) {
        try {
          this._iconTheme = St.ThemeContext.get_for_stage(
            global.stage,
          ).get_theme();
          filteredLog("debug", "Initialized icon theme from fallback method");
        } catch (error) {
          filteredLog(
            "warn",
            "Failed to initialize icon theme from fallback method:",
            error,
          );
        }
      }

      // Add the extension's icon path if we have a theme
      if (this._iconTheme && this._extensionPath) {
        const actionsPath =
          this._extensionPath + "/icons/hicolor/scalable/actions/";
        try {
          // Different API depending on GNOME version
          if (this._iconTheme.append_search_path) {
            this._iconTheme.append_search_path(actionsPath);
            filteredLog("debug", `Added icon search path: ${actionsPath}`);
          } else if (this._iconTheme.add_search_path) {
            this._iconTheme.add_search_path(actionsPath);
            filteredLog("debug", `Added icon search path: ${actionsPath}`);
          } else if (this._iconTheme.set_search_path) {
            // For older versions that use set_search_path
            const currentPaths = this._iconTheme.get_search_path();
            this._iconTheme.set_search_path([...currentPaths, actionsPath]);
            filteredLog("debug", `Added icon search path: ${actionsPath}`);
          } else {
            filteredLog(
              "warn",
              "Could not add icon search path - no supported method found",
            );
          }
        } catch (error) {
          filteredLog("error", "Error adding icon search path:", error);
        }
      }

      // If we still don't have a theme, create a basic one
      if (!this._iconTheme) {
        filteredLog("warn", "No icon theme available, creating basic theme");
        this._iconTheme = new St.Theme();
      }
    } catch (error) {
      filteredLog("error", "Error in icon theme initialization:", error);
      // Create a basic theme as fallback
      this._iconTheme = new St.Theme();
    }
  }

  /**
   * Create a light/group control section
   * @param {object} config - Configuration object
   * @returns {St.BoxLayout} - Box containing control widgets
   */
  createControlSection(config) {
    const {
      name,
      isOn,
      brightness = 50,
      temperature = 4000,
      toggleCallback,
      brightnessCallback,
      temperatureCallback,
      id,
      type = "light",
      powerIconName = SYSTEM_ICON_POWER,
      brightnessIconName = SYSTEM_ICON_BRIGHTNESS,
      temperatureIconName = SYSTEM_ICON_TEMPERATURE,
    } = config;
    filteredLog(
      "debug",
      `[UIBuilder] Creating control section for ${name} (id: ${id}, type: ${type}) with icons: power=${powerIconName}, brightness=${brightnessIconName}, temperature=${temperatureIconName}`,
    );

    // Create section container
    const section = new St.BoxLayout({
      vertical: true,
      style_class: "keylightd-control-section",
      x_expand: true,
    });

    // Add data attributes for identification
    section._keylightdId = id;
    section._keylightdType = type;

    // Create header with title and toggle
    const headerBox = new St.BoxLayout({
      style_class: "keylightd-header-box",
      x_expand: true,
    });

    // Power button (circular with icon) - MOVED TO LEFT OF TITLE
    const powerButton = new St.Button({
      style_class: isOn
        ? "keylightd-power-button keylightd-power-on"
        : "keylightd-power-button keylightd-power-off",
      x_align: Clutter.ActorAlign.START,
      can_focus: true,
    });

    // Always use the system power icon regardless of state
    // Only the style class of the button changes based on state
    const powerIcon = getIcon(powerIconName, {
      icon_size: 14,
      style_class: "keylightd-power-icon",
    });
    if (!powerIcon)
      filteredLog(
        "error",
        `[UIBuilder] Power icon failed to load for ${powerIconName}`,
      );

    // Create a fixed-size container for the power icon to ensure it's visible
    const powerIconContainer = new St.Bin({
      style_class: "keylightd-power-icon-container",
      width: 18,
      height: 18,
      x_align: Clutter.ActorAlign.CENTER,
      y_align: Clutter.ActorAlign.CENTER,
    });
    powerIconContainer.set_child(powerIcon);

    // Set the container as the child of the button
    powerButton.set_child(powerIconContainer);

    // Title (no icon anymore)
    const titleLabel = new St.Label({
      text: name,
      style_class: "keylightd-title-label",
      y_align: Clutter.ActorAlign.CENTER,
      x_expand: true, // Make title take up remaining space
    });

    // Hidden toggle switch for state tracking (this will not be visible)
    const toggle = new PopupMenu.Switch(isOn);
    toggle.visible = false;

    // Flag to prevent processing multiple click events in quick succession
    section._isToggling = false;

    // Debounce time from settings
    const debounceTime = this._settings.get_int("debounce-delay");

    // Connect power button click to toggle state
    powerButton.connect("clicked", () => {
      // Only process if not already toggling
      if (!section._isToggling && typeof toggleCallback === "function") {
        section._isToggling = true;

        // Update the toggle state immediately for better UI feedback
        const newState = !toggle.state;
        toggle.state = newState;

        // Update button visual state immediately
        powerButton.style_class = newState
          ? "keylightd-power-button keylightd-power-on"
          : "keylightd-power-button keylightd-power-off";

        // Call the toggle callback with the new state
        toggleCallback(newState);

        // Set a timeout to allow toggling again after debounce period
        const timeoutId = GLib.timeout_add(
          GLib.PRIORITY_DEFAULT,
          debounceTime,
          () => {
            section._isToggling = false;
            this._timeoutIds.delete(timeoutId);
            return GLib.SOURCE_REMOVE;
          },
        );
        this._timeoutIds.add(timeoutId);
      }
    });

    // Store a reference to the toggle callback
    section._toggleCallback = toggleCallback;

    // Connect toggle state change to update visual appearance
    toggle.connect("notify::state", () => {
      const state = toggle.state;

      // Update power button style
      if (!powerButton.is_finalized) {
        powerButton.style_class = state
          ? "keylightd-power-button keylightd-power-on"
          : "keylightd-power-button keylightd-power-off";
      }

      // Dim sliders instead of hiding them when light is off
      if (section._brightnessRow && !section._brightnessRow.is_finalized) {
        try {
          section._brightnessRow.opacity = state ? 255 : 120;
          if (
            section._brightnessRow._slider &&
            !section._brightnessRow._slider.is_finalized
          ) {
            section._brightnessRow._slider.reactive = state;
          }
        } catch (e) {
          filteredLog("error", "Error updating brightness row state:", e);
        }
      }
      if (section._temperatureRow && !section._temperatureRow.is_finalized) {
        try {
          section._temperatureRow.opacity = state ? 255 : 120;
          if (
            section._temperatureRow._slider &&
            !section._temperatureRow._slider.is_finalized
          ) {
            section._temperatureRow._slider.reactive = state;
          }
        } catch (e) {
          filteredLog("error", "Error updating temperature row state:", e);
        }
      }
    });

    // Add in the correct order for layout
    headerBox.add_child(powerButton);
    headerBox.add_child(titleLabel);
    headerBox.add_child(toggle); // Hidden but functional for state tracking

    section.add_child(headerBox);

    // ALWAYS add sliders regardless of light state - they'll just be disabled if off
    // Brightness slider
    const brightnessRow = this.createSliderRow({
      iconName: brightnessIconName,
      value: SLIDER_CONFIGS.brightness.normalize(brightness),
      callback: brightnessCallback
        ? debounce(
            (value) => brightnessCallback(Math.round(value)),
            debounceTime,
            this._timeoutIds,
          )
        : null,
      type: "brightness",
    });

    // Temperature slider
    const temperatureRow = this.createSliderRow({
      iconName: temperatureIconName,
      value: SLIDER_CONFIGS.temperature.normalize(temperature),
      callback: temperatureCallback
        ? debounce(
            (value) => temperatureCallback(Math.round(value)),
            debounceTime,
            this._timeoutIds,
          )
        : null,
      type: "temperature",
    });

    section.add_child(brightnessRow);
    section.add_child(temperatureRow);

    // Store slider rows as properties for later access
    section._brightnessRow = brightnessRow;
    section._temperatureRow = temperatureRow;

    // Set initial slider states based on whether light is on
    if (!isOn) {
      brightnessRow.opacity = 120;
      brightnessRow._slider.reactive = false;
      temperatureRow.opacity = 120;
      temperatureRow._slider.reactive = false;
    }

    // Store important elements as properties for later access
    section._toggle = toggle;
    section._titleLabel = titleLabel;
    section._powerButton = powerButton;
    section._powerIcon = powerIcon;
    section._powerIconContainer = powerIconContainer;

    return section;
  }

  /**
   * Create a slider row
   * @param {object} config - Configuration object
   * @returns {St.BoxLayout} - Box containing slider widget
   */
  createSliderRow(config) {
    const {
      iconName,
      value = 0.5,
      callback,
      type = "brightness", // 'brightness' or 'temperature'
    } = config;
    filteredLog(
      "debug",
      `[UIBuilder] Creating slider row (type: ${type}) with icon: ${iconName}`,
    );

    const sliderConfig = SLIDER_CONFIGS[type];
    if (!sliderConfig) {
      filteredLog("error", `Invalid slider type: ${type}`);
      return null;
    }

    const row = new St.BoxLayout({
      style_class: "keylightd-slider-row",
      x_expand: true,
    });

    // Create a container for the slider
    const sliderBox = new St.BoxLayout({
      style_class: "keylightd-slider-box",
      x_expand: true,
      min_width: 0,
    });

    // Ensure the icon is properly created and visible
    const icon = getIcon(iconName, {
      style_class: "keylightd-slider-icon",
      icon_size: 18,
    });
    if (!icon)
      filteredLog(
        "error",
        `[UIBuilder] Slider icon failed to load for ${iconName} (type: ${type})`,
      );

    // Force a specific size for the icon container to ensure it displays
    const iconContainer = new St.Bin({
      style_class: "keylightd-icon-container",
      width: 24,
      height: 24,
      x_align: Clutter.ActorAlign.CENTER,
      y_align: Clutter.ActorAlign.CENTER,
    });
    iconContainer.set_child(icon);

    // Slider
    const slider = new Slider(value);
    slider.x_expand = true;
    slider.min_width = 0;
    slider.style_class = "keylightd-slider";

    // Value label (always visible now)
    let valueLabel = new St.Label({
      text: sliderConfig.format(sliderConfig.denormalize(value)),
      style_class: "keylightd-value-label",
    });

    // Connect slider value changes
    slider.connect("notify::value", () => {
      const newValue = sliderConfig.denormalize(slider.value);
      valueLabel.text = sliderConfig.format(newValue);
      if (typeof callback === "function") {
        callback(newValue);
      }
    });

    // Add the slider to the sliderBox
    sliderBox.add_child(slider);

    // Add all elements to the row - use iconContainer instead of direct icon
    row.add_child(iconContainer);
    row.add_child(sliderBox);
    row.add_child(valueLabel);

    // Store references for later updates
    row._slider = slider;
    row._valueLabel = valueLabel;
    row._icon = icon;
    row._iconContainer = iconContainer;
    row._type = type;
    row._config = sliderConfig;

    return row;
  }

  /**
   * Update a control section with new values
   * @param {St.BoxLayout} section - Control section to update
   * @param {object} updates - Value updates
   */
  updateControlSection(section, updates) {
    if (!section) return;

    // Update toggle state
    if (updates.hasOwnProperty("on") && section._toggle) {
      // Only update if different to avoid loops
      if (section._toggle.state !== updates.on) {
        // This prevents interference with ongoing toggle operations
        const wasToggling = section._isToggling;

        // Update state directly
        section._toggle.state = updates.on;

        // Make sure toggling flag is maintained
        section._isToggling = wasToggling;
      }

      // ONLY update the power button's style class, never remove or replace the icon
      if (section._powerButton && !section._powerButton.is_finalized) {
        try {
          section._powerButton.style_class = updates.on
            ? "keylightd-power-button keylightd-power-on"
            : "keylightd-power-button keylightd-power-off";
        } catch (e) {
          filteredLog("error", "Error updating power button:", e);
        }
      }

      // Only dim sliders when light is off, but ALWAYS keep them visible
      if (section._brightnessRow && !section._brightnessRow.is_finalized) {
        try {
          section._brightnessRow.opacity = updates.on ? 255 : 120;
          if (
            section._brightnessRow._slider &&
            !section._brightnessRow._slider.is_finalized
          ) {
            section._brightnessRow._slider.reactive = updates.on;
          }
        } catch (e) {
          filteredLog("error", "Error updating brightness row state:", e);
        }
      }
      if (section._temperatureRow && !section._temperatureRow.is_finalized) {
        try {
          section._temperatureRow.opacity = updates.on ? 255 : 120;
          if (
            section._temperatureRow._slider &&
            !section._temperatureRow._slider.is_finalized
          ) {
            section._temperatureRow._slider.reactive = updates.on;
          }
        } catch (e) {
          filteredLog("error", "Error updating temperature row state:", e);
        }
      }
    }

    // Update brightness
    if (
      updates.hasOwnProperty("brightness") &&
      section._brightnessRow &&
      !section._brightnessRow.is_finalized &&
      section._brightnessRow._slider &&
      !section._brightnessRow._slider.is_finalized
    ) {
      try {
        const config = SLIDER_CONFIGS.brightness;
        const normalizedValue = config.normalize(updates.brightness);

        // Only update if different to avoid loops
        if (
          Math.abs(section._brightnessRow._slider.value - normalizedValue) >
          0.01
        ) {
          section._brightnessRow._slider.value = normalizedValue;

          // Update label if it exists
          if (
            section._brightnessRow._valueLabel &&
            !section._brightnessRow._valueLabel.is_finalized
          ) {
            section._brightnessRow._valueLabel.text = config.format(
              updates.brightness,
            );
          }
        }
      } catch (e) {
        filteredLog("error", "Error updating brightness slider:", e);
      }
    }

    // Update temperature
    if (
      (updates.hasOwnProperty("temperature") ||
        updates.hasOwnProperty("temperatureK")) &&
      section._temperatureRow &&
      !section._temperatureRow.is_finalized &&
      section._temperatureRow._slider &&
      !section._temperatureRow._slider.is_finalized
    ) {
      try {
        const config = SLIDER_CONFIGS.temperature;
        // Use temperatureK if available, otherwise convert from mireds
        const tempKelvin = updates.hasOwnProperty("temperatureK")
          ? updates.temperatureK
          : this._convertDeviceToKelvin(updates.temperature);

        const normalizedValue = config.normalize(tempKelvin);

        // Only update if different to avoid loops
        if (
          Math.abs(section._temperatureRow._slider.value - normalizedValue) >
          0.01
        ) {
          section._temperatureRow._slider.value = normalizedValue;

          // Update label if it exists
          if (
            section._temperatureRow._valueLabel &&
            !section._temperatureRow._valueLabel.is_finalized
          ) {
            section._temperatureRow._valueLabel.text =
              config.format(tempKelvin);
          }
        }
      } catch (e) {
        filteredLog("error", "Error updating temperature slider:", e);
      }
    }

    // Update name
    if (
      updates.hasOwnProperty("name") &&
      section._titleLabel &&
      !section._titleLabel.is_finalized
    ) {
      try {
        section._titleLabel.text = updates.name;
      } catch (e) {
        filteredLog("error", "Error updating title label:", e);
      }
    }
  }

  /**
   * Get a section element by ID and type
   * @param {St.BoxLayout} container - Container to search in
   * @param {string} id - Element ID
   * @param {string} type - Element type ('light' or 'group')
   * @returns {St.BoxLayout|null} - Found section or null
   */
  getSectionById(container, id, type) {
    let child = container.get_first_child();
    while (child) {
      if (child._keylightdId === id && child._keylightdType === type) {
        return child;
      }
      child = child.get_next_sibling();
    }
    return null;
  }

  /**
   * Convert device mireds to Kelvin
   * @param {number} mireds - Temperature in mireds
   * @returns {number} - Temperature in Kelvin
   */
  _convertDeviceToKelvin(mireds) {
    const MIN_MIREDS = 143;
    const MAX_MIREDS = 344;

    if (mireds < MIN_MIREDS) {
      mireds = MIN_MIREDS;
    } else if (mireds > MAX_MIREDS) {
      mireds = MAX_MIREDS;
    }
    return Math.round(1000000 / mireds);
  }

  /**
   * Convert Kelvin to device mireds
   * @param {number} kelvin - Temperature in Kelvin
   * @returns {number} - Temperature in mireds
   */
  _convertKelvinToDevice(kelvin) {
    const MIN_KELVIN = 2900;
    const MAX_KELVIN = 7000;

    if (kelvin < MIN_KELVIN) {
      kelvin = MIN_KELVIN;
    } else if (kelvin > MAX_KELVIN) {
      kelvin = MAX_KELVIN;
    }

    // Mireds = 1,000,000 / Kelvin
    return Math.round(1000000 / kelvin);
  }

  getSectionHeight(section) {
    // Returns the rendered height of a control section (St.BoxLayout)
    // Only valid after the section is allocated (added to stage/container and layout is complete)
    return section.height;
  }

  /**
   * Clean up resources and timeouts
   */
  destroy() {
    // Clean up all timeouts
    this._timeoutIds.forEach((timeoutId) => {
      if (timeoutId) {
        GLib.source_remove(timeoutId);
      }
    });
    this._timeoutIds.clear();

    // Clear cached icons
    this._cachedIcons.clear();

    // Clear references
    this._iconTheme = null;
    this._settings = null;
    this._extensionPath = null;
  }
}
