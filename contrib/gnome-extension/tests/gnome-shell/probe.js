import Clutter from 'gi://Clutter';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';
import Shell from 'gi://Shell';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import {ExtensionState} from 'resource:///org/gnome/shell/misc/extensionUtils.js';

const UUID = 'keylightd-control@jmylchreest.github.io';
const HOME = GLib.get_home_dir();

function delay(ms) {
    return new Promise(resolve => GLib.timeout_add(GLib.PRIORITY_DEFAULT, ms, () => {
        resolve();
        return GLib.SOURCE_REMOVE;
    }));
}

async function capture(name) {
    await delay(700);
    const file = Gio.File.new_for_path(`${GLib.getenv('KEYLIGHTD_TEST_ARTIFACTS')}/${name}.png`);
    const stream = file.replace(null, false, Gio.FileCreateFlags.NONE, null);
    try {
        await new Shell.Screenshot().screenshot(false, stream);
    } finally {
        stream.close(null);
    }
}

function assert(condition, message) {
    if (!condition)
        throw new Error(message);
}

async function waitForState(extension, state) {
    for (let attempt = 0; attempt < 100; attempt++) {
        assert(extension.state !== ExtensionState.ERROR,
            `Extension error: ${extension.error}`);
        if (extension.state === state)
            return;
        await new Promise(resolve => GLib.timeout_add(GLib.PRIORITY_DEFAULT, 100, () => {
            resolve();
            return GLib.SOURCE_REMOVE;
        }));
    }
    throw new Error(`Expected state ${state}, got ${extension.state}`);
}

function checkControls(extension) {
    const toggle = extension._keylightdIndicator._keylightdToggle;
    assert(toggle._mainBox.orientation === Clutter.Orientation.VERTICAL,
        'Menu content must be vertical');
    assert(toggle._lightsContainer.orientation === Clutter.Orientation.VERTICAL,
        'Light list must be vertical');
    const actionRow = toggle._mainBox.get_children().find(
        child => child.has_style_class_name('keylightd-action-row'));
    assert(actionRow?.orientation === Clutter.Orientation.HORIZONTAL,
        'Action buttons must be horizontal');

    // These controls are built lazily after fetching lights. Exercise them even
    // without a daemon or physical lights, so enabling alone cannot hide a bug.
    for (const type of ['light', 'group']) {
        const section = extension._uiBuilder.createControlSection({
            id: `test-${type}`, name: `Test ${type}`, type, isOn: true,
            brightness: 50, temperature: 4000,
        });
        try {
            assert(section.orientation === Clutter.Orientation.VERTICAL,
                `${type} control section must be vertical`);
            assert(section._brightnessRow && section._temperatureRow,
                `${type} must have brightness and temperature controls`);
            toggle._lightsContainer.add_child(section);
        } finally {
            section.destroy();
        }
    }
}

export default class RegressionProbe extends Extension {
    enable() {
        this._timeout = GLib.timeout_add_seconds(GLib.PRIORITY_DEFAULT, 5, () => {
            this._timeout = 0;
            this._run().catch(error => this._finish(`FAIL: ${error}\n${error.stack}`));
            return GLib.SOURCE_REMOVE;
        });
    }

    async _run() {
        const manager = Main.extensionManager;
        const extension = manager.lookup(UUID);
        assert(extension, 'Extension was not discovered');
        await waitForState(extension, ExtensionState.ACTIVE);
        checkControls(extension.stateObj);
        Main.overview.hide();
        const quickSettings = Main.panel.statusArea.quickSettings;
        quickSettings.menu.open();
        await capture('01-quick-settings');
        const toggle = extension.stateObj._keylightdIndicator._keylightdToggle;
        toggle.menu.open();
        await delay(1500);
        assert(toggle._lightsContainer.get_n_children() > 0, 'Fixture controls did not load');
        await capture('02-light-group-dropdown');
        toggle.menu.close();
        quickSettings.menu.close();
        await this._capturePreferences();
        manager.disableExtension(UUID);
        await waitForState(extension, ExtensionState.INACTIVE);
        manager.enableExtension(UUID);
        await waitForState(extension, ExtensionState.ACTIVE);
        checkControls(extension.stateObj);
        manager.disableExtension(UUID);
        await waitForState(extension, ExtensionState.INACTIVE);
        assert(!extension.error, extension.error);
        this._finish('PASS: enable, menu layouts, light/group controls, preferences screenshots, disable and re-enable');
    }

    async _capturePreferences() {
        const launcher = new Gio.SubprocessLauncher({flags: Gio.SubprocessFlags.NONE});
        launcher.setenv('WAYLAND_DISPLAY', GLib.getenv('WAYLAND_DISPLAY') || 'wayland-0', true);
        launcher.setenv('GI_TYPELIB_PATH', '/usr/lib/gnome-shell/girepository-1.0', true);
        launcher.setenv('LD_LIBRARY_PATH', '/usr/lib/gnome-shell', true);
        const process = launcher.spawnv(['gjs', '-m', '/opt/keylightd-extension/tests/gnome-shell/preferences.js']);
        let exited = false;
        process.wait_async(null, (source, result) => {
            source.wait_finish(result);
            exited = true;
        });
        try {
            for (let attempt = 0; attempt < 600; attempt++) {
                if (GLib.file_test(`${HOME}/prefs-error`, GLib.FileTest.EXISTS)) {
                    const [, bytes] = GLib.file_get_contents(`${HOME}/prefs-error`);
                    throw new Error(new TextDecoder().decode(bytes));
                }
                if (GLib.file_test(`${HOME}/prefs-done`, GLib.FileTest.EXISTS))
                    return;
                assert(!exited, 'Preferences process exited before captures completed; see shell.log');
                if (GLib.file_test(`${HOME}/capture-request`, GLib.FileTest.EXISTS)) {
                    const [, bytes] = GLib.file_get_contents(`${HOME}/capture-request`);
                    const name = new TextDecoder().decode(bytes);
                    await capture(name);
                    GLib.unlink(`${HOME}/capture-request`);
                }
                await delay(100);
            }
            throw new Error('Preferences screenshot capture timed out');
        } finally {
            process.force_exit();
        }
    }

    _finish(result) {
        GLib.file_set_contents(`${GLib.get_home_dir()}/result`, `${result}\n`);
    }

    disable() {
        if (this._timeout)
            GLib.source_remove(this._timeout);
        this._timeout = 0;
    }
}
