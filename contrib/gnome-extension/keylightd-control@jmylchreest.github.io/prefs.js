"use strict";
import * as GeneralPrefs from "./preferences/generalPage.js";
import * as GroupsPrefs from "./preferences/groupsPage.js";
import * as LightsPrefs from "./preferences/lightsPage.js";
import * as UIPrefs from "./preferences/uiPage.js";
import * as AboutPrefs from "./preferences/aboutPage.js";

import { ExtensionPreferences } from "resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js";
import { setSettings } from "./utils.js";

const SettingsKey = {
  DEFAULT_WIDTH: "prefs-default-width",
  DEFAULT_HEIGHT: "prefs-default-height",
  API_URL: "api-url",
  API_KEY: "api-key",
};

export default class KeylightdControlPreferences extends ExtensionPreferences {
  fillPreferencesWindow(window) {
    window._settings = this.getSettings();

    // Set the global settings instance
    setSettings(window._settings);

    const generalPage = new GeneralPrefs.GeneralPage(
      window._settings,
      SettingsKey,
    );
    const groupsPage = new GroupsPrefs.GroupsPage(
      window._settings,
      SettingsKey,
    );
    const lightsPage = new LightsPrefs.LightsPage(
      window._settings,
      SettingsKey,
    );
    const uiPage = new UIPrefs.UIPage(window._settings);
    const aboutPage = new AboutPrefs.AboutPage(window._settings);

    let prefsWidth = window._settings.get_int(SettingsKey.DEFAULT_WIDTH);
    let prefsHeight = window._settings.get_int(SettingsKey.DEFAULT_HEIGHT);

    window.set_default_size(prefsWidth, prefsHeight);
    window.set_search_enabled(true);

    window.add(generalPage);
    window.add(groupsPage);
    window.add(lightsPage);
    window.add(uiPage);
    window.add(aboutPage);

    window.connect("close-request", () => {
      let currentWidth = window.default_width;
      let currentHeight = window.default_height;

      // Remember user window size adjustments.
      if (currentWidth !== prefsWidth || currentHeight !== prefsHeight) {
        window._settings.set_int(SettingsKey.DEFAULT_WIDTH, currentWidth);
        window._settings.set_int(SettingsKey.DEFAULT_HEIGHT, currentHeight);
      }

      // Clear the global settings instance when the preferences window is closed
      setSettings(null);

      window.destroy();
    });
  }
}
