'use strict';

import {filteredLog} from "../utils.js";
import {action, subpage} from "./navigation.js";
import Adw from 'gi://Adw';
import Gtk from 'gi://Gtk';
import Gio from 'gi://Gio';
import { gettext as _ } from "resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js";
import GLib from 'gi://GLib';
import GObject from 'gi://GObject';

export const AboutPage = GObject.registerClass(
    class AboutPage extends Adw.PreferencesPage {
        _init(settings) {
            super._init({
                title: 'About',
                icon_name: 'help-about-symbolic',
                name: 'AboutPage'
            });

            this._settings = settings;
            this._buildUI();
        }

        _buildUI() {
            // Create main group
            const aboutGroup = new Adw.PreferencesGroup({
                title: 'About',
                description: 'Version and project information'
            });

            // Load version info
            const versionInfo = this._loadVersionInfo();

            // Project name
            const projectRow = new Adw.ActionRow({
                title: 'Project',
                subtitle: versionInfo.project_name
            });
            aboutGroup.add(projectRow);

            // Version
            const versionRow = new Adw.ActionRow({
                title: 'Version',
                subtitle: versionInfo.version
            });
            aboutGroup.add(versionRow);

            // Commit hash
            const commitRow = new Adw.ActionRow({
                title: 'Commit',
                subtitle: versionInfo.commit
            });
            aboutGroup.add(commitRow);

            // Description
            if (versionInfo.about) {
                const descriptionGroup = new Adw.PreferencesGroup({
                    title: 'Description'
                });

                const descriptionRow = new Adw.ActionRow({
                    title: versionInfo.about
                });
                descriptionGroup.add(descriptionRow);
                this.add(descriptionGroup);
            }

            this.add(aboutGroup);
            action(aboutGroup, _("Diagnostics"), _("Logging and log level"), "system-run-symbolic", () => {
                const {group} = subpage(this.get_root(), _("Diagnostics"));
                this._addDiagnostics(group);
            });
        }

        _addDiagnostics(loggingGroup) {
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
      loggingRow.bind_property("active", logLevelRow, "visible", GObject.BindingFlags.SYNC_CREATE);

        }

        _loadVersionInfo() {
            try {
                // Get the extension directory
                const extensionDir = import.meta.url.replace('file://', '').replace('/preferences/aboutPage.js', '');
                const versionFile = GLib.build_filenamev([extensionDir, 'version-info.json']);
                
                if (GLib.file_test(versionFile, GLib.FileTest.EXISTS)) {
                    const [success, contents] = GLib.file_get_contents(versionFile);
                    if (success) {
                        const decoder = new TextDecoder('utf-8');
                        const jsonString = decoder.decode(contents);
                        return JSON.parse(jsonString);
                    }
                }
            } catch (error) {
                filteredLog('error', 'Failed to load version info:', error);
            }

            // Fallback values
            return {
                project_name: 'keylightd gnome-extension',
                about: 'GNOME Shell extension for controlling Key Light devices through keylightd daemon',
                version: 'development',
                commit: 'unknown'
            };
        }
    }
);