"use strict";

import Adw from "gi://Adw";
import Gtk from "gi://Gtk";
import GObject from "gi://GObject";
import Gio from "gi://Gio";

import { gettext as _ } from "resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js";
import { SYSTEM_PREFS_GENERAL_ICON } from "../icon-names.js";

export var GeneralPage = GObject.registerClass(
  class KeylightdGeneralPage extends Adw.PreferencesPage {
    _init(settings, settingsKey) {
      super._init({
        title: _("General"),
        icon_name: SYSTEM_PREFS_GENERAL_ICON,
        name: "GeneralPage",
      });
      this._settings = settings;
      this._settingsKey = settingsKey;

      // Keylight Client Configuration
      // --------------
      let keylightdClientGroup = new Adw.PreferencesGroup({
        title: _("Keylight Client Configuration"),
        description: _("Configuration specific to the keylightd client"),
      });

      // API URL
      const httpURL = new Adw.EntryRow({
        title: _("URL"),
      });
      // Add description below the entry row
      httpURL.set_tooltip_text(_("The HTTP URL of the keylightd server"));

      this._settings.bind(
        "api-url",
        httpURL,
        "text",
        Gio.SettingsBindFlags.DEFAULT,
      );

      // API Key
      const httpAPIKey = new Adw.PasswordEntryRow({
        title: _("API Key"),
      });
      // Add description below the entry row
      httpAPIKey.set_tooltip_text(
        _("The API Key provided by the keylightd server"),
      );

      this._settings.bind(
        "api-key",
        httpAPIKey,
        "text",
        Gio.SettingsBindFlags.DEFAULT,
      );

      keylightdClientGroup.add(httpURL);
      keylightdClientGroup.add(httpAPIKey);
      this.add(keylightdClientGroup);

      // Refresh Settings
      // --------------
      let refreshSettingsGroup = new Adw.PreferencesGroup({
        title: _("Refresh Settings"),
        description: _(
          "Configure how often lights and groups status are updated",
        ),
      });

      // Create an Adw.SpinRow for refresh interval - much better than using a custom slider for this use case
      const refreshRow = new Adw.SpinRow({
        title: _("Refresh Interval"),
        subtitle: _("Time in seconds between automatic status updates"),
        adjustment: new Gtk.Adjustment({
          lower: 5, // Minimum 5 seconds
          upper: 300, // Maximum 5 minutes (300 seconds)
          step_increment: 5, // Step by 5 seconds
          page_increment: 30,
          value: this._settings.get_int("refresh-interval"),
        }),
        digits: 0,
        value: this._settings.get_int("refresh-interval"),
      });

      // Update the setting when the row value changes
      refreshRow.connect("notify::value", () => {
        this._settings.set_int("refresh-interval", refreshRow.get_value());
      });

      refreshSettingsGroup.add(refreshRow);

      // Create an Adw.SpinRow for debounce delay
      const debounceRow = new Adw.SpinRow({
        title: _("Control Update Delay"),
        subtitle: _("Delay in milliseconds before sending updates to lights"),
        adjustment: new Gtk.Adjustment({
          lower: 0, // Minimum 0 ms (immediate)
          upper: 2000, // Maximum 2 seconds
          step_increment: 50, // Step by 50 ms
          page_increment: 200,
          value: this._settings.get_int("debounce-delay"),
        }),
        digits: 0,
        value: this._settings.get_int("debounce-delay"),
      });

      // Update the setting when the row value changes
      debounceRow.connect("notify::value", () => {
        this._settings.set_int("debounce-delay", debounceRow.get_value());
      });

      // Add description
      debounceRow.set_tooltip_text(
        _(
          "Higher values reduce network traffic but make controls less responsive",
        ),
      );

      refreshSettingsGroup.add(debounceRow);

      // Add a suffix to indicate units
      const refreshSuffix = new Gtk.Label({
        label: _("seconds"),
        margin_start: 8,
        css_classes: ["dim-label"],
      });
      refreshRow.add_suffix(refreshSuffix);

      const debounceSuffix = new Gtk.Label({
        label: _("milliseconds"),
        margin_start: 8,
        css_classes: ["dim-label"],
      });
      debounceRow.add_suffix(debounceSuffix);

      this.add(refreshSettingsGroup);

      // Logging Settings
      // --------------
      let loggingGroup = new Adw.PreferencesGroup({
        title: _("Logging Settings"),
        description: _("Configure logging behavior"),
      });

      // Enable logging toggle
      const loggingRow = new Adw.SwitchRow({
        title: _("Enable Logging"),
        subtitle: _("Enable detailed logging for debugging"),
      });

      this._settings.bind(
        "enable-logging",
        loggingRow,
        "active",
        Gio.SettingsBindFlags.DEFAULT,
      );
      loggingGroup.add(loggingRow);

      // Log level dropdown
      const logLevelRow = new Adw.ComboRow({
        title: _("Log Level"),
        subtitle: _("Minimum level of messages to log"),
      });

      // Set up the log level dropdown options
      const logLevelModel = new Gtk.StringList();
      logLevelModel.append(_("Debug")); // 0
      logLevelModel.append(_("Info")); // 1
      logLevelModel.append(_("Warning")); // 2
      logLevelModel.append(_("Error")); // 3

      logLevelRow.set_model(logLevelModel);

      // Map between dropdown index and actual log level value
      const logLevelMap = ["debug", "info", "warning", "error"];

      // Set the initial selected value based on settings
      const currentLogLevel = this._settings.get_string("log-level");
      const logLevelIndex = logLevelMap.indexOf(currentLogLevel);
      logLevelRow.set_selected(logLevelIndex >= 0 ? logLevelIndex : 2); // Default to warning if not found

      // Update setting when selection changes
      logLevelRow.connect("notify::selected", () => {
        const selected = logLevelRow.get_selected();
        if (selected >= 0 && selected < logLevelMap.length) {
          this._settings.set_string("log-level", logLevelMap[selected]);
        }
      });

      loggingGroup.add(logLevelRow);
      this.add(loggingGroup);
    }
  },
);
