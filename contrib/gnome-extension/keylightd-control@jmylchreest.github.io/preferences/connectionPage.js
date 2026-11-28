import Adw from "gi://Adw";
import Gdk from "gi://Gdk?version=4.0";
import GLib from "gi://GLib";
import Gtk from "gi://Gtk";
import {gettext as _} from "resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js";
import {profilePath, runClient, testManual} from "../transport.js";

import {action, subpage} from "./navigation.js";

function confirm(window, title, body, label) {
  const dialog = new Adw.AlertDialog({heading:title, body});
  dialog.add_response("cancel", _("Cancel"));
  dialog.add_response("confirm", label);
  dialog.close_response = "cancel";
  dialog.default_response = "cancel";
  return new Promise(resolve => {
    dialog.connect("response", (_, id) => resolve(id === "confirm"));
    dialog.present(window);
  });
}

export function connectionPage(window, settings) {
  const page = new Adw.PreferencesPage({title:_("Connection"), icon_name:"network-wired-symbolic"});
  const methods = new Adw.PreferencesGroup();
  page.add(methods);
  const socket = () => settings.get_string("socket-path");
  const admin = (op, ...args) => runClient(["--socket", socket(), "https", op, ...args]);
  const toast = text => window.add_toast(new Adw.Toast({title:String(text), use_markup:false}));
  const guard = fn => async row => {
    row.sensitive = false;
    try { await fn(); } catch (error) { toast(error.message); }
    finally { row.sensitive = true; }
  };
  const select = method => {
    settings.set_string("connection-method", method);
    updateMethods();
    toast(_("Connected"));
  };

  function local() {
    const {group} = subpage(window, _("This computer"));
    const path = new Adw.EntryRow({title:_("Socket path"), text:socket()});
    group.add(path);
    action(group, _("Test and use"), "", null, guard(async () => {
      const candidate = path.text.trim();
      if (!candidate) throw new Error(_("Enter a socket path."));
      await runClient(["--socket", candidate, "request", "GET", "lights"]);
      settings.set_string("socket-path", candidate);
      select("local");
      window.pop_subpage();
    }));
  }

  function manual() {
    const {group} = subpage(window, _("Manual address"));
    const url = new Adw.EntryRow({title:_("API address"), text:settings.get_string("api-url")});
    const key = new Adw.PasswordEntryRow({title:_("API key"), text:settings.get_string("api-key")});
    group.add(url);
    group.add(key);
    action(group, _("Test and use"), "", null, guard(async () => {
      const endpoint = url.text.trim().replace(/\/+$/, "");
      await testManual(endpoint, key.text);
      settings.set_string("api-url", endpoint);
      settings.set_string("api-key", key.text);
      select("manual");
      window.pop_subpage();
    }));
  }

  async function pair(token) {
    await runClient(["--profile", profilePath, "https", "pair"], token.trim());
    select("paired");
  }

  async function secure() {
    const {group, content} = subpage(window, _("Paired HTTPS"));
    action(group, _("Use an invitation"), _("Pair with another daemon"), "network-workgroup-symbolic", () => {
      const entryGroup = subpage(window, _("Pair with keylightd")).group;
      const entry = new Adw.PasswordEntryRow({title:_("Single-use invitation")});
      entryGroup.add(entry);
      action(entryGroup, _("Pair and test"), "", null, guard(async () => {
        await pair(entry.text);
        entry.text = "";
        window.pop_subpage();
      }));
    });
    if (GLib.file_test(profilePath, GLib.FileTest.EXISTS) && settings.get_string("connection-method") !== "paired") {
      action(group, _("Use saved pairing"), "", null, guard(async () => {
        await runClient(["--profile", profilePath, "request", "GET", "lights"]);
        select("paired");
        window.pop_subpage();
      }));
    }
    const management = new Adw.PreferencesGroup({title:_("On this computer")});
    let status;
    try { status = JSON.parse(await admin("status")); } catch { return; }
    content.add(management);
    let endpoint = status.urls?.[0] || "";
    const localPair = async () => pair(await admin("invite", "--url", endpoint));
    if (!status.enabled) {
      action(management, _("Enable and use HTTPS"), status.restore_http ? _("HTTP enabled") : _("Local socket only"), "changes-prevent-symbolic", guard(async () => {
        const warning = status.restore_http ? _("Existing HTTP clients will disconnect and need to reconnect using HTTPS.") : "";
        if (!await confirm(window, _("Use HTTPS?"), warning, _("Enable and use"))) return;
        await admin("enable");
        try { await localPair(); } finally { window.pop_subpage(); }
      }));
    } else {
      action(management, _("Use HTTPS on this computer"), endpoint, "changes-prevent-symbolic", guard(async () => {
        await localPair();
        window.pop_subpage();
      }));
      action(management, _("Create invitation"), _("Pair another client"), "network-server-symbolic", guard(async () => {
        const token = await admin("invite", "--url", endpoint);
        const reference = GLib.compute_checksum_for_string(GLib.ChecksumType.SHA256, token, -1).slice(0,8).toUpperCase();
        const dialog = new Adw.AlertDialog({heading:_("Pair another client"),
          body:_("Invitation %s · Single use · Expires in five minutes").replace("%s", reference)});
        dialog.add_response("close", _("Done"));
        dialog.add_response("copy", _("Copy invitation"));
        dialog.connect("response", (_, id) => {
          if (id === "copy") Gdk.Display.get_default().get_clipboard().set(token);
        });
        dialog.present(window);
      }));
      action(management, _("Disable HTTPS"), _("Paired connections will disconnect"), null, guard(async () => {
        if (!await confirm(window, _("Disable HTTPS?"), status.restore_http ? _("HTTP will resume.") : _("The local socket will remain available."), _("Disable HTTPS"))) return;
        let useLocal = false;
        if (settings.get_string("connection-method") === "paired") {
          const [, bytes] = GLib.file_get_contents(profilePath);
          useLocal = JSON.parse(new TextDecoder().decode(bytes)).fingerprint === status.fingerprint;
        }
        await admin("disable");
        if (useLocal) select("local");
        window.pop_subpage();
      }));
    }
    const address = action(management, _("Network address"), endpoint, "network-wired-symbolic", () => {
      const addresses = subpage(window, _("Network address")).group;
      for (const url of status.urls || []) action(addresses, url, url === endpoint ? _("Selected") : "", null, () => {
        endpoint = url;
        address.subtitle = url;
        window.pop_subpage();
      });
    });
    action(management, _("Daemon identity"), status.fingerprint ? _("Saved") : _("No saved identity"), "dialog-password-symbolic", () => {
      const identity = subpage(window, _("Daemon identity")).group;
      identity.add(new Adw.ActionRow({title:"SHA-256", subtitle:status.fingerprint || _("No saved identity")}));
      if (!status.enabled && status.fingerprint) action(identity, _("Forget identity…"), "", null, guard(async () => {
        if (!await confirm(window, _("Forget HTTPS identity?"), _("Clients will need to pair again after the next activation."), _("Forget identity"))) return;
        await admin("delete-identity");
        window.pop_subpage();
        window.pop_subpage();
      }));
    });
  }

  const rows = {
    local: action(methods, _("This computer"), socket(), "computer-symbolic", local),
    paired: action(methods, _("Paired HTTPS"), _("Pair using an invitation"), "changes-prevent-symbolic", guard(secure)),
    manual: action(methods, _("Manual address"), _("API address and key"), "network-server-symbolic", manual),
  };
  const checks = {};
  for (const [id, row] of Object.entries(rows)) {
    checks[id] = new Gtk.Image({icon_name:"object-select-symbolic", tooltip_text:_("Active connection")});
    row.add_suffix(checks[id]);
  }
  function updateMethods() {
    const selected = settings.get_string("connection-method");
    const configured = selected !== "manual" || Boolean(settings.get_string("api-url") && settings.get_string("api-key"));
    for (const [id, check] of Object.entries(checks)) check.visible = configured && id === selected;
    rows.local.subtitle = socket();
  }
  updateMethods();
  return page;
}
