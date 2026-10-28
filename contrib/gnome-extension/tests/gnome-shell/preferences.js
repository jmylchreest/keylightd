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
