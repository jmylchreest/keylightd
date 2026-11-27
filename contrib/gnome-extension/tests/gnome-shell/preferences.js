import Adw from 'gi://Adw';
import Gtk from 'gi://Gtk?version=4.0';
import Gio from 'gi://Gio';
import GLib from 'gi://GLib';

// The normal GNOME Extensions entry point installs this GJS helper.
imports.package.initFormat();

Gio.resources_register(Gio.Resource.load('/usr/share/gnome-shell/org.gnome.Shell.Extensions.src.gresource'));
const {ExtensionPreferences} = await import('resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js');
const home = GLib.get_home_dir();
const path = `${GLib.get_user_data_dir()}/gnome-shell/extensions/keylightd-control@jmylchreest.github.io`;
const {default: Preferences} = await import(`file://${path}/prefs.js`);
const [, bytes] = GLib.file_get_contents(`${path}/metadata.json`);
const metadata = JSON.parse(new TextDecoder().decode(bytes));
const preferences = new Preferences({...metadata, path, dir: Gio.File.new_for_path(path)});
// Resolve translation calls just as the normal Extensions service does.
ExtensionPreferences.lookupByUUID = uuid => uuid === metadata.uuid ? preferences : null;

function delay(ms) {
    return new Promise(resolve => GLib.timeout_add(GLib.PRIORITY_DEFAULT, ms, () => {
        resolve();
        return GLib.SOURCE_REMOVE;
    }));
}

async function capture(name) {
    await delay(1000);
    GLib.file_set_contents(`${home}/capture-request`, name);
    for (let i = 0; i < 100; i++) {
        await delay(100);
        if (!GLib.file_test(`${home}/capture-request`, GLib.FileTest.EXISTS))
            return;
    }
    throw new Error(`Screenshot not acknowledged: ${name}`);
}

function descendants(widget) {
    const children = [];
    for (let child = widget.get_first_child(); child; child = child.get_next_sibling())
        children.push(child, ...descendants(child));
    return children;
}

const app = new Adw.Application({application_id: 'io.github.keylightd.ScreenshotTest'});
app.connect('activate', () => {
    (async () => {
        const window = new Adw.PreferencesWindow({application: app, title: 'Keylightd Preferences'});
        const pages = [];
        const add = window.add.bind(window);
        window.add = page => { pages.push(page); add(page); };
        preferences.fillPreferencesWindow(window);
        window.set_default_size(900, 1000);
        window.present();
        if (pages.length !== 5)
            throw new Error(`Expected five preferences pages, got ${pages.length}`);
        for (const [index, page] of pages.entries()) {
            window.set_visible_page(page);
            await capture(`0${index + 3}-preferences-${page.title.toLowerCase()}`);
        }

        const groups = pages.find(page => page.title === 'Groups');
        window.set_visible_page(groups);
        for (const [name, group] of [
            ['08-add-group', null],
            ['09-edit-group', {id: 'studio', name: 'Studio', lights: ['key', 'fill']}],
        ]) {
            groups._showGroupDialog(group);
            await capture(name);
            for (const dialog of Gtk.Window.get_toplevels()) {
                if (dialog !== window)
                    dialog.close();
            }
        }

        groups._confirmDeleteGroup({id: 'studio', name: 'Studio'});
        await capture('10-delete-group-confirmation');
        for (const dialog of Gtk.Window.get_toplevels()) {
            if (dialog instanceof Adw.MessageDialog)
                dialog.response('cancel');
        }

        const lights = pages.find(page => page.title === 'Lights');
        window.set_visible_page(lights);
        const pending = {id: 'air', name: 'Air MK.2', port: 9123, needsConfiguration: ['credentials_missing']};
        const row = lights._createLightRow(pending);
        const menu = descendants(row).find(widget => widget instanceof Gtk.Button && widget.label === 'Configure…');
        if (!menu)
            throw new Error('Setup menu not found');
        const dialog = lights._showSetupDialog(lights, options => {
            if (JSON.stringify(options) !== JSON.stringify({
                peripheral: '00:11:22:33:44:55', ssid: 'Test Wi-Fi',
                password: 'fixture-password', security: 200,
            }))
                throw new Error('Unexpected pairing settings');
            submitted = true;
        });
        let submitted = false;
        if (dialog.get_response_enabled('start'))
            throw new Error('An empty setup form can be submitted');
        const fields = descendants(dialog.get_extra_child());
        const entries = fields.filter(widget => widget instanceof Adw.EntryRow);
        const password = fields.find(widget => widget instanceof Adw.PasswordEntryRow);
        entries[0].text = '00:11:22:33:44:55';
        entries[1].text = 'Test Wi-Fi';
        password.text = 'fixture-password';
        if (!dialog.get_response_enabled('start'))
            throw new Error('A complete setup form cannot be submitted');
        await capture('12-air-mk2-setup');
        const start = descendants(dialog).find(widget =>
            widget instanceof Gtk.Button && widget.label === 'Start setup');
        if (!start)
            throw new Error('Setup start button not found');
        start.emit('clicked');
        if (!submitted || password.text)
            throw new Error('Setup must submit the form and clear its password');

        window.set_visible_page(pages[0]);
        const combo = descendants(pages[0]).find(widget => widget instanceof Adw.ComboRow);
        if (!combo)
            throw new Error('Log level dropdown not found');
        combo.activate();
        await capture('11-log-level-dropdown');
        GLib.file_set_contents(`${home}/prefs-done`, 'done');
    })().catch(error => {
        GLib.file_set_contents(`${home}/prefs-error`, `${error}\n${error.stack}`);
    });
});
await app.runAsync([]);
