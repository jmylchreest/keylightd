import Gio from 'gi://Gio';
import {filteredLog, setSettings} from '../../keylightd-control@jmylchreest.github.io/utils.js';

const settings = new Gio.Settings({schema_id: 'org.gnome.shell.extensions.keylightd-control'});

setSettings(settings);
filteredLog('error', 'logging disabled');
settings.set_boolean('enable-logging', true);
settings.set_string('log-level', 'error');
filteredLog('warn', 'below threshold');
filteredLog('error', 'refresh failed', 'daemon unavailable');
setSettings(null);
filteredLog('error', 'after disable');
