"use strict";

import GObject from "gi://GObject";
import St from "gi://St";
import Clutter from "gi://Clutter";
import * as PopupMenu from "resource:///org/gnome/shell/ui/popupMenu.js";
import * as QuickSettings from "resource:///org/gnome/shell/ui/quickSettings.js";
import { StateEvents } from "../managers/state-manager.js";
import { log } from "../utils.js";
import GLib from "gi://GLib";
import Gio from "gi://Gio";
import {
  getLightStateIcon,
  LightState,
  determineLightState,
} from "../icons.js";
import {
  SYSTEM_ICON_POWER,
  SYSTEM_ICON_BRIGHTNESS,
  SYSTEM_ICON_TEMPERATURE,
} from "../icon-names.js";
import { getGroupLights } from "../utils.js";

// Light states enum
export const LightStates = Object.freeze({
  UNKNOWN: Symbol("unknown"),
  ENABLED: Symbol("enabled"),
  DISABLED: Symbol("disabled"),
  ACTIVE: Symbol("active"),
  INACTIVE: Symbol("inactive"),
});

export const KeylightdControlToggle = GObject.registerClass(
  {
    Signals: {
      "refresh-requested": {},
    },
  },
  class KeylightdControlToggle extends QuickSettings.QuickMenuToggle {
    _init(extension) {
      super._init({
        title: _("Keylightd"),
        toggleMode: true,
      });

      // Store references
      this._extension = extension;
      this._settings = extension._settings;
      this._stateManager = extension._stateManager;
      this._uiBuilder = extension._uiBuilder;
      this._lightsController = extension._lightsController;
      this._groupsController = extension._groupsController;

      // Animation settings
      this._useAnimations =
        this._settings.get_boolean("use-animations") ?? true;

      // Initial state
      this._lightState = LightStates.UNKNOWN;
      this._updateIcon(this._lightState);
      this.checked = false;

      // Add debounce to prevent rapid toggling
      this._isToggling = false;
      // Use the same debounce time from the UI builder that's used for sliders
      this._debounceTime = this._uiBuilder._debounceDelay || 500;

      // Add loading guards and debounce id
      this._isLoadingControls = false;
      this._loadControlsDebounceId = null;

      // Populate the menu
      this._setupMenu();

      // Set up the refresh timer
      this._refreshInterval = 0;
      this._refreshTimeoutId = 0;
      this._setupRefreshTimer();

      // Connect to clicked signal for toggle action
      this.connect("clicked", () => {
        if (!this._isToggling) {
          this._isToggling = true;
          this._toggleLights();

          // Prevent toggling again until timeout
          this._toggleTimeoutId = GLib.timeout_add(
            GLib.PRIORITY_DEFAULT,
            this._debounceTime,
            () => {
              this._isToggling = false;
              return GLib.SOURCE_REMOVE;
            },
          );
        }
      });

      // Connect to state manager events
      this._stateSubscriptions = [];
      this._subscribeToStateEvents();

      // Connect settings change signals
      this._connectSettings();
    }



    /**
     * Set up the menu structure
     */
    _setupMenu() {
      // Create main content box
      this._mainBox = new St.BoxLayout({
        vertical: true,
        style_class: "keylightd-menu-box",
      });

      // Create a scrollable container for light controls with a fixed height
      const scrollView = new St.ScrollView({
        style_class: "keylightd-scroll-view",
        hscrollbar_policy: St.PolicyType.NEVER,
        vscrollbar_policy: St.PolicyType.AUTOMATIC,
        overlay_scrollbars: true,
        enable_mouse_scrolling: true,
      });

      // Container for light controls
      this._lightsContainer = new St.BoxLayout({
        vertical: true,
        style_class: "keylightd-lights-container",
      });

      scrollView.set_child(this._lightsContainer);
      this._mainBox.add_child(scrollView);

      // Create a menu item for our content
      let contentItem = new PopupMenu.PopupBaseMenuItem({
        activate: false,
        can_focus: false,
      });
      contentItem.add_child(this._mainBox);
      this.menu.addMenuItem(contentItem);

      // Set up refresh on menu open
      this.menu.connect("open-state-changed", (menu, isOpen) => {
        if (isOpen) {
          // Save current light state before loading controls
          const currentState = this._lightState;

          // Update the header icon immediately with current state
          // Only set a basic header initially, avoiding subtitle which may not exist yet
          if (this.menu) {
            this.menu.setHeader(this._getIcon(currentState), _("Lights"));
          }

          // Then load controls (which will update state again once data is loaded)
          this._debouncedLoadControls();
        }
      });

      // Add a styled settings button at the bottom
      const settingsButtonBox = new St.BoxLayout({
        style_class: "keylightd-action-row",
        vertical: false,
        x_align: Clutter.ActorAlign.CENTER,
        y_align: Clutter.ActorAlign.CENTER,
        x_expand: true,
        y_expand: false,
        reactive: false,
        can_focus: false,
      });
      const settingsButton = new St.Button({
        style_class: "keylightd-action-button",
        label: _("Settings"),
        x_expand: true,
        y_expand: false,
        can_focus: true,
        reactive: true,
        height: 36,
        style: "padding: 4px 0; min-height: 28px; font-size: 14px;",
      });
      settingsButton.connect("clicked", () =>
        this._extension._openPreferences(),
      );
      settingsButtonBox.add_child(settingsButton);
      this._mainBox.add_child(settingsButtonBox);
    }

    /**
     * Set up the refresh timer based on settings
     */
    _setupRefreshTimer() {
      this._clearRefreshTimer();

      // Get interval from settings (in seconds)
      this._refreshInterval = this._settings.get_int("refresh-interval");

      // Only set up timer if interval is positive
      if (this._refreshInterval > 0) {
        this._refreshTimeoutId = setInterval(() => {
          // Only refresh if menu is open
          if (this.menu.isOpen) {
            this._debouncedLoadControls();
          }
        }, this._refreshInterval * 1000);
      }
    }

    /**
     * Clear the refresh timer
     */
    _clearRefreshTimer() {
      if (this._refreshTimeoutId) {
        clearInterval(this._refreshTimeoutId);
        this._refreshTimeoutId = 0;
      }
    }

    /**
     * Connect to settings change signals
     */
    _connectSettings() {
      this._settings.connect("changed::api-url", () => {
        this._debouncedLoadControls();
      });

      this._settings.connect("changed::api-key", () => {
        this._debouncedLoadControls();
      });

      this._settings.connect("changed::refresh-interval", () => {
        this._setupRefreshTimer();
      });

      this._settings.connect("changed::visible-groups", () => {
        this._debouncedLoadControls();
      });

      this._settings.connect("changed::visible-lights", () => {
        this._debouncedLoadControls();
      });

      // Animation settings
      this._settings.connect("changed::use-animations", () => {
        this._useAnimations = this._settings.get_boolean("use-animations");
      });

      // Update debounce time when it changes in settings
      this._settings.connect("changed::debounce-time", () => {
        this._debounceTime = this._settings.get_int("debounce-time");
      });

      // Listen for max-height-percent changes and refresh UI after debounce
      this._settings.connect("changed::max-height-percent", () => {
        if (this._maxHeightTimeoutId) {
          GLib.source_remove(this._maxHeightTimeoutId);
          this._maxHeightTimeoutId = null;
        }
        this._maxHeightTimeoutId = GLib.timeout_add(
          GLib.PRIORITY_DEFAULT,
          300,
          () => {
            this._debouncedLoadControls();
            this._maxHeightTimeoutId = null;
            return GLib.SOURCE_REMOVE;
          },
        );
      });

      // Subscribe to state events
      this._subscribeToStateEvents();
    }

    /**
     * Subscribe to state manager events
     */
    _subscribeToStateEvents() {
      // Light updates
      this._stateSubscriptions.push(
        this._stateManager.subscribe(StateEvents.LIGHT_UPDATED, (data) => {
          this._updateLightUI(data.id, data.updates);
        }),
      );

      // Group updates
      this._stateSubscriptions.push(
        this._stateManager.subscribe(StateEvents.GROUP_UPDATED, (data) => {
          this._updateGroupUI(data.id, data.updates);
        }),
      );

      // Error state
      this._stateSubscriptions.push(
        this._stateManager.subscribe(StateEvents.ERROR, (data) => {
          const anyOnStateChanged =
            data.isErroring !== this._stateManager.isErroring();

          if (anyOnStateChanged) {
            this._updateToggleState();
          }
        }),
      );
    }

    /**
     * Unsubscribe from all state events
     */
    _unsubscribeFromStateEvents() {
      this._stateSubscriptions.forEach((unsubscribe) => unsubscribe());
      this._stateSubscriptions = [];
    }

    /**
     * Debounced version of _loadControls
     */
    _debouncedLoadControls(delay = 100) {
      if (this._loadControlsDebounceId) {
        GLib.source_remove(this._loadControlsDebounceId);
        this._loadControlsDebounceId = null;
      }
      this._loadControlsDebounceId = GLib.timeout_add(
        GLib.PRIORITY_DEFAULT,
        delay,
        () => {
          this._loadControls();
          this._loadControlsDebounceId = null;
          return GLib.SOURCE_REMOVE;
        },
      );
    }

    /**
     * Clear all controls from the container
     */
    _clearControls() {
      if (!this._lightsContainer) return;

      // Only remove child elements that aren't needed anymore
      // This preserves power button icon state across refreshes

      // First, make a list of all existing sections to check later
      const existingSections = new Map();

      let child = this._lightsContainer.get_first_child();
      while (child) {
        // Only consider UI control sections (not separators etc.)
        if (child._keylightdId && child._keylightdType) {
          existingSections.set(
            `${child._keylightdType}:${child._keylightdId}`,
            child,
          );
        }
        child = child.get_next_sibling();
      }

      // We'll keep this map for _loadControls to use
      this._existingSections = existingSections;

      // Remove all children - sections will be reused in _loadControls
      while (this._lightsContainer.get_first_child()) {
        this._lightsContainer.remove_child(
          this._lightsContainer.get_first_child(),
        );
      }
    }

    /**
     * Load controls into the menu (with guard against parallel execution)
     */
    async _loadControls() {
      if (this._isLoadingControls) {
        log("debug", "Already loading controls, skipping duplicate call");
        return;
      }
      this._isLoadingControls = true;

      // Make a "loading" header while data is being loaded
      const headerTitle = _("Lights");
      const headerSubtitle = _("Loading...");

      // Keep the current icon state for the header while loading
      if (this.menu) {
        // Set the header without assuming anything about its current state
        this.menu.setHeader(
          this._getIcon(this._lightState),
          headerTitle,
          headerSubtitle,
        );
      }

      try {
        // Get existing sections and clear container
        this._clearControls();
        const existingSections = this._existingSections || new Map();
        this._existingSections = null; // Clear reference

        let itemCount = 0;
        let visibleGroups = [];
        let visibleLights = [];
        let totalGroups = 0;
        let totalLights = 0;

        // Try to get counts of all lights and groups
        try {
          const allGroups = await this._groupsController.getGroups();
          totalGroups = allGroups.length;

          const allLights = await this._lightsController.getLights();
          totalLights = allLights.length;

          // Get visible groups and lights in parallel
          [visibleGroups, visibleLights] = await Promise.all([
            this._groupsController.getVisibleGroups(),
            this._lightsController.getVisibleLights(),
          ]);
        } catch (error) {
          log("warn", "Could not get total counts:", error);
        }

        // Update toggle state first so the global icon is updated
        await this._updateToggleState();

        // Add visible groups if any
        if (visibleGroups.length > 0) {
          for (const group of visibleGroups) {
            try {
              // Get all light objects for the group in one call
              const groupLights = await getGroupLights(
                this._groupsController,
                this._lightsController,
                group.id,
              );
              if (!groupLights || groupLights.length === 0) {
                log("info", `Group ${group.id} has no lights, skipping`);
                continue;
              }
              // Determine group state: on if any light is on
              const anyOn = groupLights.some(
                (light) => light && light.on === true,
              );
              // Use the first light for brightness/temperature display
              const firstLight = groupLights[0];
              const groupSection = this._uiBuilder.createControlSection({
                id: group.id,
                type: "group",
                name: group.name,
                isOn: anyOn,
                brightness: firstLight.brightness || 50,
                temperature: this._uiBuilder._convertDeviceToKelvin(
                  firstLight.temperature || 200,
                ),
                powerIconName: SYSTEM_ICON_POWER,
                brightnessIconName: SYSTEM_ICON_BRIGHTNESS,
                temperatureIconName: SYSTEM_ICON_TEMPERATURE,
                toggleCallback: (state) =>
                  this._toggleGroupState(group.id, state),
                brightnessCallback: (value) =>
                  this._groupsController.setGroupBrightness(group.id, value),
                temperatureCallback: (value) =>
                  this._groupsController.setGroupTemperature(group.id, value),
              });
              this._lightsContainer.add_child(groupSection);
              itemCount++;
            } catch (error) {
              log("error", `Error loading group ${group.id}:`, error);
            }
          }
        }

        // Add visible individual lights
        if (visibleLights.length > 0) {
          // Create controls for each light
          for (const light of visibleLights) {
            try {
              // Get the latest state of the light
              const lightDetails = await this._lightsController.getLight(
                light.id,
              );

              // Check if we have an existing section for this light
              const sectionKey = `light:${light.id}`;
              const existingSection = existingSections.get(sectionKey);

              let lightSection;

              if (existingSection && !existingSection.is_finalized) {
                // Update existing section instead of creating new one
                this._uiBuilder.updateControlSection(existingSection, {
                  name: lightDetails.name || light.id,
                  on: lightDetails.on === true,
                  brightness: lightDetails.brightness || 50,
                  temperatureK: this._uiBuilder._convertDeviceToKelvin(
                    lightDetails.temperature || 200,
                  ),
                });

                lightSection = existingSection;
              } else {
                // Create new section
                lightSection = this._uiBuilder.createControlSection({
                  id: light.id,
                  type: "light",
                  name: lightDetails.name || light.id,
                  isOn: lightDetails.on === true,
                  brightness: lightDetails.brightness || 50,
                  temperature: this._uiBuilder._convertDeviceToKelvin(
                    lightDetails.temperature || 200,
                  ),
                  powerIconName: SYSTEM_ICON_POWER,
                  brightnessIconName: SYSTEM_ICON_BRIGHTNESS,
                  temperatureIconName: SYSTEM_ICON_TEMPERATURE,
                  toggleCallback: (state) =>
                    this._toggleLightState(light.id, state),
                  brightnessCallback: (value) =>
                    this._lightsController.setLightBrightness(light.id, value),
                  temperatureCallback: (value) =>
                    this._lightsController.setLightTemperature(light.id, value),
                });
              }

              this._lightsContainer.add_child(lightSection);
              itemCount++;
            } catch (error) {
              log("error", `Error loading light ${light.id}:`, error);
            }
          }
        }

        // Update menu header with counts
        const finalHeaderTitle = _("Lights");
        const finalHeaderSubtitle = _(
          "Showing %d/%d groups, %d/%d lights",
        ).format(
          visibleGroups.length,
          totalGroups,
          visibleLights.length,
          totalLights,
        );

        // Update toggle state again to ensure all UI is in sync
        await this._updateToggleStateWithData(visibleGroups, visibleLights);

        // Use the current light state icon for the header now that we've updated it
        if (this.menu) {
          // Safely set the header with updated data
          this.menu.setHeader(
            this._getIcon(this._lightState),
            finalHeaderTitle,
            finalHeaderSubtitle,
          );
        }

        // If neither groups nor lights loaded, show a message
        if (visibleGroups.length === 0 && visibleLights.length === 0) {
          const noLightsLabel = new St.Label({
            text: _("No visible lights or groups"),
            style_class: "keylightd-info-label",
            x_align: Clutter.ActorAlign.CENTER,
          });
          this._lightsContainer.add_child(noLightsLabel);
        }

        // Remove any previous signal connections for height recalculation
        if (this._lightsContainerSignals) {
          for (let id of this._lightsContainerSignals) {
            this._lightsContainer.disconnect(id);
          }
        }
        this._lightsContainerSignals = [];
        // Connect only to child-added and child-removed
        this._lightsContainerSignals.push(
          this._lightsContainer.connect("child-added", () =>
            this._recalculateScrollViewHeight(),
          ),
        );
        this._lightsContainerSignals.push(
          this._lightsContainer.connect("child-removed", () =>
            this._recalculateScrollViewHeight(),
          ),
        );

        // Update state manager's error state
        this._stateManager.setErroring(false);
      } catch (error) {
        log("error", "Error loading controls:", error);

        // Show error message
        const errorLabel = new St.Label({
          text: _("Error: Check API settings"),
          style_class: "keylightd-error-label",
          x_align: Clutter.ActorAlign.CENTER,
        });
        this._lightsContainer.add_child(errorLabel);

        // Update state manager's error state
        this._stateManager.setErroring(true, error);
      }

      // After controls are loaded, recalculate height
      this._recalculateScrollViewHeight();
      this._isLoadingControls = false;

      // Force an icon state update at the end to ensure everything is in sync
      this.notify("gicon");
    }

    _recalculateScrollViewHeight() {
      let totalHeight = 0;
      let child = this._lightsContainer.get_first_child();

      log("debug", "Starting scroll view height calculation");

      // Helper function to safely get numeric style property
      const getNumericStyleProperty = (actor, property) => {
        try {
          // Try to get the style property from the actor's style
          const style = actor.get_style();
          if (style && style[property]) {
            const value = parseFloat(style[property]);
            log("debug", `Style property ${property}: ${value}`);
            return isNaN(value) ? 0 : value;
          }
          return 0;
        } catch (e) {
          log("debug", `Error getting style property ${property}: ${e}`);
          return 0;
        }
      };

      // Helper function to get actor height
      const getActorHeight = (actor) => {
        if (!actor || actor.is_finalized) {
          log("debug", "Actor is null or finalized, returning 0 height");
          return 0;
        }

        let height = 0;
        try {
          // Get preferred size using the correct method
          const [minWidth, minHeight, natWidth, natHeight] =
            actor.get_preferred_size();
          if (natHeight > 0) {
            height = natHeight;
            log(
              "debug",
              `Child ${childIndex} Using natural height: ${height} (min: ${minHeight}, nat: ${natHeight})`,
            );
          } else if (minHeight > 0) {
            height = minHeight;
            log(
              "debug",
              `Child ${childIndex} Using minimum height: ${height} (min: ${minHeight}, nat: ${natHeight})`,
            );
          } else {
            // Fallback to height property
            height = actor.height || 0;
            log(
              "debug",
              `Child ${childIndex} Using direct height property: ${height}`,
            );
          }
        } catch (e) {
          log("debug", `Error getting height, using 0: ${e}`);
          height = 0;
        }
        return height;
      };

      // Calculate height of all light/group controls
      let childIndex = 0;
      while (child) {
        if (!child.is_finalized) {
          const height = getActorHeight(child);
          const marginTop = getNumericStyleProperty(child, "margin-top");
          const marginBottom = getNumericStyleProperty(child, "margin-bottom");
          const paddingTop = getNumericStyleProperty(child, "padding-top");
          const paddingBottom = getNumericStyleProperty(
            child,
            "padding-bottom",
          );

          const childTotal =
            height + marginTop + marginBottom + paddingTop + paddingBottom;
          totalHeight += childTotal;

          log(
            "debug",
            `Running total: ${totalHeight}; Child ${childIndex} total: ${childTotal} (height: ${height}, margins: ${marginTop + marginBottom}, padding: ${paddingTop + paddingBottom})`,
          );
        } else {
          log("debug", `Skipping finalized child ${childIndex}`);
        }
        child = child.get_next_sibling();
        childIndex++;
      }

      // Add height of settings button if it exists
      const settingsButton = this._mainBox.get_last_child();
      if (settingsButton && !settingsButton.is_finalized) {
        const buttonHeight = getActorHeight(settingsButton);
        const buttonMarginTop = getNumericStyleProperty(
          settingsButton,
          "margin-top",
        );
        const buttonMarginBottom = getNumericStyleProperty(
          settingsButton,
          "margin-bottom",
        );
        const buttonPaddingTop = getNumericStyleProperty(
          settingsButton,
          "padding-top",
        );
        const buttonPaddingBottom = getNumericStyleProperty(
          settingsButton,
          "padding-bottom",
        );

        const buttonTotal =
          buttonHeight +
          buttonMarginTop +
          buttonMarginBottom +
          buttonPaddingTop +
          buttonPaddingBottom;
        log(
          "debug",
          `Settings button total: ${buttonTotal} (height: ${buttonHeight}, margins: ${buttonMarginTop + buttonMarginBottom}, padding: ${buttonPaddingTop + buttonPaddingBottom})`,
        );

        totalHeight += buttonTotal;
        log("debug", `Final total height with settings button: ${totalHeight}`);
      } else {
        log("debug", "Settings button not found or finalized");
      }

      // Ensure we have a valid positive height
      totalHeight = Math.max(0, totalHeight);

      this._adjustScrollViewHeightWithActual(totalHeight);
    }

    /**
     * Adjust the scroll view's height based on the actual rendered content height
     * @param {number} actualContentHeight - The sum of the heights of each control section (including power button, label, sliders, etc)
     */
    _adjustScrollViewHeightWithActual(actualContentHeight) {
      // Find the first child that's a scroll view
      let scrollView = null;
      let child = this._mainBox.get_first_child();
      while (child && !scrollView) {
        if (child instanceof St.ScrollView) {
          scrollView = child;
        } else {
          child = child.get_next_sibling();
        }
      }
      if (!scrollView) return;
      // Get screen height to determine available space
      const monitor = global.display.get_current_monitor();
      const workArea = global.display
        .get_workspace_manager()
        .get_active_workspace()
        .get_work_area_for_monitor(monitor);
      const screenHeight = workArea.height;
      // Use setting for max height percent (default 0.5, min 0.25, max 0.75)
      let percent = this._settings.get_double("max-height-percent");
      if (typeof percent !== "number" || isNaN(percent)) percent = 0.5;
      percent = Math.max(0.25, Math.min(0.75, percent));
      const maxAvailableHeight = Math.floor(screenHeight * percent);
      // Set scrollview max-height to the smaller of content height and configured max
      const preferredHeight = Math.min(maxAvailableHeight, actualContentHeight);
      scrollView.style = `max-height: ${preferredHeight}px;`;
      scrollView.vscrollbar_policy = St.PolicyType.AUTOMATIC;
      scrollView.overlay_scrollbars = true;
      scrollView.queue_relayout();
    }

    /**
     * Update a group's UI section with new data
     * @param {string} groupId - Group ID to update
     * @param {object} updates - Properties to update
     */
    _updateGroupUI(groupId, updates) {
      const section = this._uiBuilder.getSectionById(
        this._lightsContainer,
        groupId,
        "group",
      );
      if (section) {
        this._uiBuilder.updateControlSection(section, updates);
      }
    }

    /**
     * Update a light's UI section with new data
     * @param {string} lightId - Light ID to update
     * @param {object} updates - Properties to update
     */
    _updateLightUI(lightId, updates) {
      const section = this._uiBuilder.getSectionById(
        this._lightsContainer,
        lightId,
        "light",
      );
      if (section) {
        this._uiBuilder.updateControlSection(section, updates);
      }
    }

    /**
     * Get icon for the current state
     * @param {Symbol} lightState - Current light state
     * @returns {Gio.ThemedIcon} - Icon for current state
     */
    _getIcon(lightState) {
      // Convert from LightStates enum to LightState string
      let stateStr = LightState.UNKNOWN;

      switch (lightState) {
        case LightStates.ENABLED:
          stateStr = LightState.ENABLED;
          break;
        case LightStates.DISABLED:
          stateStr = LightState.DISABLED;
          break;
        default:
          stateStr = LightState.UNKNOWN;
          break;
      }

      const icon = getLightStateIcon(stateStr);
      return icon;
    }

    /**
     * Update the icon based on state
     * @param {Symbol} lightState - New light state
     */
    _updateIcon(lightState) {
      this._lightState = lightState;
      this.gicon = this._getIcon(lightState);
      // Notify observers that the icon has changed
      this.notify("gicon");
    }

    /**
     * Refresh the state of all groups and their lights
     * This ensures consistent UI state across all controls
     */
    async _refreshAllGroupStates() {
      try {
        // Get all groups
        const allGroups = await this._groupsController.getGroups();
        const allLights = await this._lightsController.getLights();

        // Create a map of lightId -> light for faster lookup
        const lightMap = new Map();
        allLights.forEach((light) => lightMap.set(light.id, light));

        // For each group, update its state based on its lights
        for (const group of allGroups) {
          try {
            const groupDetails = await this._groupsController.getGroup(
              group.id,
            );
            if (!groupDetails.lights || groupDetails.lights.length === 0)
              continue;

            // Check if all lights in the group are on or off
            let allLightsOn = true;
            let allLightsOff = true;

            for (const lightId of groupDetails.lights) {
              const light =
                lightMap.get(lightId) ||
                (await this._lightsController.getLight(lightId));
              if (!light) continue;

              if (light.on) {
                allLightsOff = false;
              } else {
                allLightsOn = false;
              }
            }

            // Update group UI
            const groupState = allLightsOn ? true : allLightsOff ? false : null;
            if (groupState !== null) {
              const groupSection = this._uiBuilder.getSectionById(
                this._lightsContainer,
                group.id,
                "group",
              );
              if (groupSection && !groupSection.is_finalized) {
                if (
                  groupSection._toggle &&
                  !groupSection._toggle.is_finalized
                ) {
                  groupSection._toggle.state = groupState;
                }

                if (
                  groupSection._powerButton &&
                  !groupSection._powerButton.is_finalized
                ) {
                  groupSection._powerButton.style_class = groupState
                    ? "keylightd-power-button keylightd-power-on"
                    : "keylightd-power-button keylightd-power-off";
                }
              }
            }
          } catch (err) {
            log("error", `Error refreshing group ${group.id}:`, err);
          }
        }
      } catch (error) {
        log("error", "Error refreshing all group states:", error);
      }
    }

    /**
     * Toggle the state of a light with animation and update any groups containing it
     * @param {string} lightId - The light ID
     * @param {boolean} state - The new state
     */
    async _toggleLightState(lightId, state) {
      try {
        log("info", `Toggling light ${lightId} to ${state}`);

        // Find the section first to get immediate UI feedback
        const section = this._uiBuilder.getSectionById(
          this._lightsContainer,
          lightId,
          "light",
        );

        // Update UI immediately for responsive feedback
        if (section && !section.is_finalized) {
          // Update toggle state without triggering callbacks
          if (section._toggle && !section._toggle.is_finalized) {
            // Disconnect any existing handler to avoid loops
            if (section._toggle._notifyStateId) {
              section._toggle.disconnect(section._toggle._notifyStateId);
            }

            // Update state
            section._toggle.state = state;

            // Update power button appearance
            if (section._powerButton && !section._powerButton.is_finalized) {
              section._powerButton.style_class = state
                ? "keylightd-power-button keylightd-power-on"
                : "keylightd-power-button keylightd-power-off";
            }

            // Sliders should always be visible, just dimmed by the UI builder's updateControlSection
            // No need to change visibility here

            // Add animation if enabled
            if (this._useAnimations) {
              this._animateSection(section, state);
            }
          }
        }

        // Then update the backend
        await this._lightsController.setLightState(lightId, state);

        // Update global toggle state
        await this._updateToggleState();

        // Refresh all groups and their states to ensure UI consistency
        await this._refreshAllGroupStates();
      } catch (error) {
        log("error", `Error toggling light ${lightId}:`, error);
      }
    }

    /**
     * Toggle the state of a group with animation and update all lights in the group
     * @param {string} groupId - The group ID
     * @param {boolean} state - The new state
     */
    async _toggleGroupState(groupId, state) {
      try {
        log("info", `Toggling group ${groupId} to ${state}`);

        // Find the section first to get immediate UI feedback
        const section = this._uiBuilder.getSectionById(
          this._lightsContainer,
          groupId,
          "group",
        );

        // Update UI immediately for responsive feedback
        if (section && !section.is_finalized) {
          // Update toggle state without triggering callbacks
          if (section._toggle && !section._toggle.is_finalized) {
            // Update state
            section._toggle.state = state;

            // Update power button appearance
            if (section._powerButton && !section._powerButton.is_finalized) {
              section._powerButton.style_class = state
                ? "keylightd-power-button keylightd-power-on"
                : "keylightd-power-button keylightd-power-off";
            }

            // Sliders should always be visible, just dimmed by the UI builder's updateControlSection
            // No need to change visibility here

            // Add animation if enabled
            if (this._useAnimations) {
              this._animateSection(section, state);
            }
          }
        }

        // Get all lights in the group and update them visually first for immediate feedback
        try {
          const groupDetails = await this._groupsController.getGroup(groupId);
          if (
            groupDetails &&
            groupDetails.lights &&
            groupDetails.lights.length > 0
          ) {
            // Update all light controls that may be part of this group
            for (const lightId of groupDetails.lights) {
              const lightSection = this._uiBuilder.getSectionById(
                this._lightsContainer,
                lightId,
                "light",
              );
              if (lightSection && !lightSection.is_finalized) {
                // Update toggle state without triggering callbacks
                if (
                  lightSection._toggle &&
                  !lightSection._toggle.is_finalized
                ) {
                  lightSection._toggle.state = state;
                }

                // Update power button appearance
                if (
                  lightSection._powerButton &&
                  !lightSection._powerButton.is_finalized
                ) {
                  lightSection._powerButton.style_class = state
                    ? "keylightd-power-button keylightd-power-on"
                    : "keylightd-power-button keylightd-power-off";
                }

                // Sliders should always be visible, just dimmed by the UI builder's updateControlSection
                // No need to change visibility here

                // Add animation if enabled
                if (this._useAnimations) {
                  this._animateSection(lightSection, state);
                }
              }
            }
          }
        } catch (err) {
          log(
            "error",
            `Error updating UI for lights in group ${groupId}:`,
            err,
          );
        }

        // Update the backend - this also updates the state manager for the group AND its lights
        await this._groupsController.setGroupState(groupId, state);

        // Update global toggle state to ensure it reflects the current state
        await this._updateToggleState();
      } catch (error) {
        log("error", `Error toggling group ${groupId}:`, error);
      }
    }

    /**
     * Toggle all lights based on current state
     * If all lights are off, turn them all on
     * If any light is on, turn all lights off
     */
    async _toggleLights() {
      try {
        // First, determine the actual state of all lights (not just relying on UI state)
        const visibleGroups = await this._groupsController.getVisibleGroups();
        const visibleLights = await this._lightsController.getVisibleLights();
        const allLights = this._stateManager.getAllLights();
        const allGroups = this._stateManager.getAllGroups();
        const visibleLightIds = visibleLights.map((light) => light.id);
        const visibleGroupIds = visibleGroups.map((group) => group.id);
        const state = determineLightState(
          allLights,
          allGroups,
          visibleLightIds,
          visibleGroupIds,
        );
        const anyLightOn = state === LightState.ENABLED;

        // The new state will be the opposite
        const newState = !anyLightOn;

        // Update icon and state immediately for responsiveness
        this.checked = newState;
        this._updateIcon(newState ? LightStates.ENABLED : LightStates.DISABLED);

        // Log what we're doing
        log(
          "debug",
          `Toggle all lights: Current state=${anyLightOn}, new state=${newState}`,
        );

        // First toggle groups if any
        if (visibleGroups.length > 0) {
          // Update group UI immediately for all visible groups
          for (const group of visibleGroups) {
            const section = this._uiBuilder.getSectionById(
              this._lightsContainer,
              group.id,
              "group",
            );
            if (section && !section.is_finalized) {
              // Update toggle state
              if (section._toggle && !section._toggle.is_finalized) {
                section._toggle.state = newState;
              }

              // Update power button
              if (section._powerButton && !section._powerButton.is_finalized) {
                section._powerButton.style_class = newState
                  ? "keylightd-power-button keylightd-power-on"
                  : "keylightd-power-button keylightd-power-off";
              }

              // Sliders should always be visible, just dimmed by the UI builder's updateControlSection
              // No need to change visibility here

              // Add animation
              if (this._useAnimations) {
                this._animateSection(section, newState);
              }
            }
          }

          // Set all groups to the new state
          if (newState) {
            // Turn all groups on
            for (const group of visibleGroups) {
              await this._groupsController.setGroupState(group.id, true);
            }
          } else {
            // Turn all groups off
            for (const group of visibleGroups) {
              await this._groupsController.setGroupState(group.id, false);
            }
          }
        }

        // Then toggle individual lights
        if (visibleLights.length > 0) {
          // Update light UI immediately for all visible lights
          for (const light of visibleLights) {
            const section = this._uiBuilder.getSectionById(
              this._lightsContainer,
              light.id,
              "light",
            );
            if (section && !section.is_finalized) {
              // Update toggle state
              if (section._toggle && !section._toggle.is_finalized) {
                section._toggle.state = newState;
              }

              // Update power button
              if (section._powerButton && !section._powerButton.is_finalized) {
                section._powerButton.style_class = newState
                  ? "keylightd-power-button keylightd-power-on"
                  : "keylightd-power-button keylightd-power-off";
              }

              // Sliders should always be visible, just dimmed by the UI builder's updateControlSection
              // No need to change visibility here

              // Add animation
              if (this._useAnimations) {
                this._animateSection(section, newState);
              }
            }
          }

          // Set all individual lights to the new state
          if (newState) {
            // Turn all lights on
            for (const light of visibleLights) {
              await this._lightsController.setLightState(light.id, true);
            }
          } else {
            // Turn all lights off
            for (const light of visibleLights) {
              await this._lightsController.setLightState(light.id, false);
            }
          }
        }

        if (visibleGroups.length === 0 && visibleLights.length === 0) {
          log("info", "No visible lights or groups to toggle");
        }
      } catch (error) {
        log("error", "Error toggling lights:", error);
      }
    }

    /**
     * Animate a section when toggling state
     * @param {St.BoxLayout} section - The section to animate
     * @param {boolean} state - The new state
     */
    _animateSection(section, state) {
      // Safety check - ensure section is still valid
      if (!section || section.is_finalized) return;

      // Stop any existing transition
      section.remove_all_transitions();

      // Get animation duration from settings
      const animationDuration =
        this._settings.get_int("animation-duration") || 300;

      if (state) {
        // Turning on: start with slightly dimmed opacity and brighten
        section.opacity = 200; // Start at 78% opacity
        section.ease({
          opacity: 255,
          duration: animationDuration,
          mode: Clutter.AnimationMode.EASE_OUT_QUAD,
        });
      } else {
        // Turning off: fade out slightly
        section.opacity = 255;
        section.ease({
          opacity: 180, // Fade to 70% opacity
          duration: animationDuration,
          mode: Clutter.AnimationMode.EASE_OUT_QUAD,
          onComplete: () => {
            // Safety check - ensure section is still valid
            if (!section || section.is_finalized) return;

            // After fade out, check if slider rows are still valid before hiding
            if (
              section._brightnessRow &&
              !section._brightnessRow.is_finalized &&
              section._brightnessRow.visible !== true
            ) {
              section._brightnessRow.visible = true;
            }
            if (
              section._temperatureRow &&
              !section._temperatureRow.is_finalized &&
              section._temperatureRow.visible !== true
            ) {
              section._temperatureRow.visible = true;
            }

            // Fade back to normal opacity, but only if section is still valid
            if (!section.is_finalized) {
              section.ease({
                opacity: 255,
                duration: animationDuration / 2,
                mode: Clutter.AnimationMode.EASE_IN_QUAD,
              });
            }
          },
        });
      }
    }

    /**
     * Update the toggle state based on light states
     */
    async _updateToggleState() {
      try {
        // Get all visible groups and lights
        const visibleGroups = await this._groupsController.getVisibleGroups();
        const visibleLights = await this._lightsController.getVisibleLights();

        // Get all lights and groups for state checking
        const allLights = this._stateManager.getAllLights();
        const allGroups = this._stateManager.getAllGroups();

        // Get visible IDs for determineLightState
        const visibleLightIds = visibleLights.map((light) => light.id);
        const visibleGroupIds = visibleGroups.map((group) => group.id);

        // Determine the light state
        const state = determineLightState(
          allLights,
          allGroups,
          visibleLightIds,
          visibleGroupIds,
        );

        // Convert to LightStates enum for internal use
        let newState;
        switch (state) {
          case LightState.ENABLED:
            newState = LightStates.ENABLED;
            break;
          case LightState.DISABLED:
            newState = LightStates.DISABLED;
            break;
          default:
            newState = LightStates.UNKNOWN;
            break;
        }

        // Update icon and toggle state
        this._updateIcon(newState);
        this.checked = newState === LightStates.ENABLED;

        // Update menu header icon if menu exists and has been initialized
        if (this.menu && this.menu.setHeader) {
          try {
            this.menu.setHeader(this._getIcon(newState), _("Lights"));
          } catch (e) {
            log("debug", "Menu header not fully initialized yet: " + e);
          }
        }
      } catch (error) {
        log("error", "Error updating toggle state:", error);
        this._updateIcon(LightStates.UNKNOWN);
      }
    }

    /**
     * Update toggle state using existing data
     * @param {Array} visibleGroups - Array of visible groups
     * @param {Array} visibleLights - Array of visible lights
     */
    async _updateToggleStateWithData(visibleGroups, visibleLights) {
      try {
        // Get all lights and groups for state checking
        const allLights = this._stateManager.getAllLights();
        const allGroups = this._stateManager.getAllGroups();

        // Get visible IDs for determineLightState
        const visibleLightIds = visibleLights.map((light) => light.id);
        const visibleGroupIds = visibleGroups.map((group) => group.id);

        // Determine the light state
        const state = determineLightState(
          allLights,
          allGroups,
          visibleLightIds,
          visibleGroupIds,
        );

        // Convert to LightStates enum for internal use
        let newState;
        switch (state) {
          case LightState.ENABLED:
            newState = LightStates.ENABLED;
            break;
          case LightState.DISABLED:
            newState = LightStates.DISABLED;
            break;
          default:
            newState = LightStates.UNKNOWN;
            break;
        }

        // Update icon and toggle state
        this._updateIcon(newState);
        this.checked = newState === LightStates.ENABLED;

        // Update menu header icon if menu exists and has been initialized
        if (this.menu && this.menu.setHeader) {
          try {
            const subtitle =
              this.menu.header && this.menu.header.subtitle
                ? this.menu.header.subtitle
                : "";
            this.menu.setHeader(this._getIcon(newState), _("Lights"), subtitle);
          } catch (e) {
            log("debug", "Menu header not fully initialized yet: " + e);
          }
        }
      } catch (error) {
        log("error", "Error updating toggle state:", error);
        this._updateIcon(LightStates.UNKNOWN);
      }
    }

    /**
     * Destroy the toggle and clean up
     */
    destroy() {
      // Clear timer
      this._clearRefreshTimer();

      // Clear all timeout IDs
      if (this._toggleTimeoutId) {
        GLib.source_remove(this._toggleTimeoutId);
        this._toggleTimeoutId = null;
      }

      if (this._loadControlsDebounceId) {
        GLib.source_remove(this._loadControlsDebounceId);
        this._loadControlsDebounceId = null;
      }

      if (this._maxHeightTimeoutId) {
        GLib.source_remove(this._maxHeightTimeoutId);
        this._maxHeightTimeoutId = null;
      }

      // Unsubscribe from state events
      this._unsubscribeFromStateEvents();

      // Clear references
      this._extension = null;
      this._settings = null;
      this._stateManager = null;
      this._uiBuilder = null;
      this._lightsController = null;
      this._groupsController = null;

      // Call parent
      super.destroy();
    }
  },
);
