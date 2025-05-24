'use strict';

import Adw from 'gi://Adw';
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
                console.error('Failed to load version info:', error);
            }

            // Fallback values
            return {
                project_name: 'keylightd gnome-extension',
                about: 'GNOME Shell extension for controlling Elgato Key Light devices through keylightd daemon',
                version: 'development',
                commit: 'unknown'
            };
        }
    }
);