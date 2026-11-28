"use strict";

import Adw from "gi://Adw";
import Gtk from "gi://Gtk";
import GObject from "gi://GObject";

import { gettext as _ } from "resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js";

// Import shared utilities
import { fetchAPI, getErroring, filteredLog } from "../utils.js";
import { SYSTEM_PREFS_LIGHTS_ICON } from "../icon-names.js";

// Main Lights Page
export var LightsPage = GObject.registerClass(
  {
    Signals: {
      "refresh-requested": {},
    },
  },
  class LightsPage extends Adw.PreferencesPage {
    _init(settings, settingsKey) {
      super._init({
        title: _("Lights"),
        icon_name: SYSTEM_PREFS_LIGHTS_ICON,
        name: "LightsPage",
      });

      this._settings = settings;
      this._settingsKey = settingsKey;

      // Create the lights list
      this._lightsGroup = new Adw.PreferencesGroup({
        title: _("Show in Quick Settings"),
        description: "",
      });

      // Add a header with a refresh button
      const headerBox = new Gtk.Box({
        orientation: Gtk.Orientation.HORIZONTAL,
        spacing: 6,
      });

      this._refreshButton = new Gtk.Button({
        icon_name: "view-refresh-symbolic",
        tooltip_text: _("Refresh Lights"),
        valign: Gtk.Align.CENTER,
      });
      this._refreshButton.connect("clicked", () => this._loadLights());

      headerBox.append(this._refreshButton);

      this._lightsGroup.set_header_suffix(headerBox);

      // Create toggle all button
      const toggleAllBox = new Gtk.Box({
        orientation: Gtk.Orientation.HORIZONTAL,
        margin_end: 8,
        spacing: 8,
      });

      const showAllButton = new Gtk.Button({
        label: _("Show All"),
        valign: Gtk.Align.CENTER,
      });

      const hideAllButton = new Gtk.Button({
        label: _("Hide All"),
        valign: Gtk.Align.CENTER,
      });

      showAllButton.connect("clicked", () =>
        this._toggleAllLightsVisibility(true),
      );
      hideAllButton.connect("clicked", () =>
        this._toggleAllLightsVisibility(false),
      );

      toggleAllBox.append(showAllButton);
      toggleAllBox.append(hideAllButton);

      this._visibilityToggleRow = toggleAllBox;
      toggleAllBox.halign = Gtk.Align.END;

      // Status information
      this._statusBox = new Gtk.Box({
        orientation: Gtk.Orientation.VERTICAL,
        spacing: 12,
        margin_top: 12,
        margin_bottom: 12,
        margin_start: 12,
        margin_end: 12,
        visible: false,
      });

      this._spinner = new Gtk.Spinner({
        halign: Gtk.Align.CENTER,
        visible: false,
      });

      this._statusLabel = new Gtk.Label({
        halign: Gtk.Align.CENTER,
      });

      this._statusBox.append(this._spinner);
      this._statusBox.append(this._statusLabel);

      // Container for light rows
      this._lightsList = new Gtk.Box({
        orientation: Gtk.Orientation.VERTICAL,
        spacing: 6,
      });

      // No lights message
      this._noLightsLabel = new Gtk.Label({
        label: _("No lights found"),
        halign: Gtk.Align.CENTER,
        margin_top: 12,
        margin_bottom: 12,
      });

      const listBox = new Gtk.Box({
        orientation: Gtk.Orientation.VERTICAL,
      });

      listBox.append(this._statusBox);
      listBox.append(this._lightsList);
      listBox.append(toggleAllBox);
      listBox.append(this._noLightsLabel);

      this._lightsGroup.add(listBox);
      this.add(this._lightsGroup);

      // Connect to settings changes
      for (const key of ["connection-method", "socket-path"]) {
        this._settings.connect("changed::" + key, () => this._loadLights());
      }
      this._settings.connect("changed::api-url", () => {
        this._loadLights();
      });

      this._settings.connect("changed::api-key", () => {
        this._loadLights();
      });

      // Load lights
      this._loadLights();
    }

    _updateUIState() {
      const isErroring = getErroring();

      // Disable all actions when in error state
      this._refreshButton.sensitive = true; // Always allow refresh

      this._lightsList.sensitive = !isErroring;
      this._visibilityToggleRow.sensitive = !isErroring;
    }

    _setStatus(message, isError = false) {
      this._statusLabel.label = message;
      this._statusBox.visible = true;

      if (isError) {
        this._statusLabel.add_css_class("error");
      } else {
        this._statusLabel.remove_css_class("error");
      }
    }

    _clearStatus() {
      this._statusBox.visible = false;
    }

    async _loadLights() {
      // Clear existing lights
      while (this._lightsList.get_first_child()) {
        this._lightsList.remove(this._lightsList.get_first_child());
      }

      this._spinner.visible = true;
      this._spinner.start();
      this._setStatus(_("Loading lights..."));
      this._lightsList.visible = false;
      this._noLightsLabel.visible = false;

      try {
        // Fetch lights from the API
        let lightsResponse = await fetchAPI("lights");

        this._spinner.stop();
        this._spinner.visible = false;
        this._clearStatus();

        // Convert response to array if it's an object
        let lightsArray = [];
        if (Array.isArray(lightsResponse)) {
          lightsArray = lightsResponse;
        } else if (lightsResponse && typeof lightsResponse === "object") {
          lightsArray = Object.entries(lightsResponse).map(
            ([id, lightData]) => {
              return {
                id: id,
                name: lightData.name || id,
                ...lightData,
              };
            },
          );
        }

        // Sort lights by name (alphanumeric, case-insensitive)
        lightsArray = lightsArray.sort((a, b) =>
          (a.name || "").localeCompare(b.name || "", undefined, {
            numeric: true,
            sensitivity: "base",
          }),
        );

        if (!lightsArray || lightsArray.length === 0) {
          this._noLightsLabel.visible = true;
        } else {
          this._lightsList.visible = true;

          // Get current visible lights
          let visibleLights = this._settings.get_strv("visible-lights") || [];

          // Add each light to the list
          for (const light of lightsArray) {
            const lightRow = this._createLightRow(light);
            this._lightsList.append(lightRow);
          }
        }
      } catch (error) {
        filteredLog("error", "Error loading lights:", error);
        this._spinner.stop();
        this._spinner.visible = false;
        this._setStatus(
          _("Error loading lights. Check connection settings."),
          true,
        );
        this._noLightsLabel.visible = false;
        this._lightsList.visible = false;
        this._updateUIState();
      }
    }

    _createLightRow(light) {
      const row = new Adw.ActionRow({
        title: light.name,
        subtitle: [light.productName || light.productname, light.ip].filter(Boolean).join(" · ") || light.id,
        use_markup: false,
      });

      // Get current visible lights
      const visibleLights = this._settings.get_strv("visible-lights") || [];
      const isVisible = visibleLights.includes(light.id);

      // Visibility toggle
      const visibilitySwitch = new Gtk.Switch({
        active: isVisible,
        valign: Gtk.Align.CENTER,
        margin_end: 8,
      });

      visibilitySwitch.connect("notify::active", () => {
        let visibleLights = this._settings.get_strv("visible-lights") || [];

        if (visibilitySwitch.active) {
          // Add to visible lights if not already there
          if (!visibleLights.includes(light.id)) {
            visibleLights.push(light.id);
          }
        } else {
          // Remove from visible lights
          visibleLights = visibleLights.filter((id) => id !== light.id);
        }

        this._settings.set_strv("visible-lights", visibleLights);
      });

      row.add_suffix(visibilitySwitch);
      row.activatable_widget = visibilitySwitch;

      return row;
    }

    // Toggle visibility for all lights
    async _toggleAllLightsVisibility(visible) {
      try {
        let lightsResponse = await fetchAPI("lights");
        let lightsArray = [];
        if (Array.isArray(lightsResponse)) {
          lightsArray = lightsResponse;
        } else if (lightsResponse && typeof lightsResponse === "object") {
          lightsArray = Object.entries(lightsResponse).map(
            ([id, lightData]) => {
              return {
                id: id,
                name: lightData.name || id,
                ...lightData,
              };
            },
          );
        }
        // Sort lights by name (alphanumeric, case-insensitive)
        lightsArray = lightsArray.sort((a, b) =>
          (a.name || "").localeCompare(b.name || "", undefined, {
            numeric: true,
            sensitivity: "base",
          }),
        );
        if (lightsArray && lightsArray.length > 0) {
          // First collect all light IDs
          const allLightIds = lightsArray.map((light) => light.id);

          // Then update the setting
          if (visible) {
            // Show all lights
            filteredLog(
              "info",
              `Setting all ${allLightIds.length} lights to visible`,
            );
            this._settings.set_strv("visible-lights", allLightIds);
          } else {
            // Hide all lights
            filteredLog("info", "Hiding all lights");
            this._settings.set_strv("visible-lights", []);
          }
          this._loadLights();
        } else {
          this._setStatus(_("No lights found"));
        }
      } catch (error) {
        filteredLog("error", "Error toggling all lights visibility:", error);
        this._setStatus(
          `${_("Failed to toggle visibility")}: ${error.message}`,
          true,
        );
      }
    }
  },
);
