"use strict";

import Adw from "gi://Adw";
import Gtk from "gi://Gtk";
import GObject from "gi://GObject";
import GLib from "gi://GLib";

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
        title: _("Keylight Lights"),
        description: _("Manage individual Key Lights"),
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

      const addNearby = new Gtk.Button({ label: _("Add light via Bluetooth (BLE)"), valign: Gtk.Align.CENTER });
      addNearby.connect("clicked", () => this._showSetupDialog(this, body => this._runNearbySetup(body), null, "address"));
      this._cancelSetupButton = new Gtk.Button({
        label: _("Cancel setup"), valign: Gtk.Align.CENTER, visible: false,
      });
      this._cancelSetupButton.connect("clicked", async () => {
        try {
          if (this._activeSetupPath) await fetchAPI(this._activeSetupPath, "DELETE");
        } catch (_error) {
          this._setStatus(_("Could not cancel setup."), true);
        }
      });
      headerBox.append(addNearby);
      headerBox.append(this._cancelSetupButton);
      headerBox.append(this._refreshButton);

      this._lightsGroup.set_header_suffix(headerBox);

      // Create a visibility toggle-all action row
      this._visibilityToggleRow = new Adw.ActionRow({
        title: _("Light Visibility"),
        subtitle: _("Show or hide all lights"),
      });

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

      this._visibilityToggleRow.add_suffix(toggleAllBox);
      this._lightsGroup.add(this._visibilityToggleRow);

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
      listBox.append(this._noLightsLabel);

      this._lightsGroup.add(listBox);
      this.add(this._lightsGroup);

      // Connect to settings changes
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

      // Disable all light rows when in error state
      let child = this._lightsList.get_first_child();
      while (child) {
        // Find all buttons within each row
        const popover = new Gtk.Popover();
        const menu = new Gtk.Box({
          orientation: Gtk.Orientation.VERTICAL,
          spacing: 6,
        });
        popover.set_child(menu);
        row.add_suffix(new Gtk.MenuButton({
          label: _("Setup…"),
          valign: Gtk.Align.CENTER,
          popover,
        }));
        const buttons = [];
        if (child instanceof Adw.ActionRow) {
          // Get all suffixes (which should include our buttons)
          for (let i = 0; i < child.get_suffix_count(); i++) {
            const widget = child.get_suffix_at_index(i);
            if (widget instanceof Gtk.Button || widget instanceof Gtk.Switch) {
              buttons.push(widget);
            }
          }
        }

        // Set sensitivity of all buttons
        buttons.forEach((button) => {
          button.sensitive = !isErroring;
        });

        child = child.get_next_sibling();
      }

      if (isErroring) {
        this._setStatus(
          _("API connection error. Please check settings."),
          true,
        );
      }
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

    _showSetupDialog(parent, submit, lightName = null, phase = "all") {
      const fields = new Adw.PreferencesGroup();
      const peripheral = new Adw.EntryRow({ title: _("Bluetooth address or UUID") });
      const ssid = new Adw.EntryRow({ title: _("Wi-Fi network name") });
      const security = new Adw.ComboRow({
        title: _("Wi-Fi security"),
        model: Gtk.StringList.new([_("WPA2 Personal (AES)"), _("Open network")]),
      });
      const password = new Adw.PasswordEntryRow({ title: _("Wi-Fi password") });
      for (const field of [peripheral, ssid, security, password])
        fields.add(field);
      peripheral.visible = phase !== "wifi";
      ssid.visible = security.visible = phase !== "address";

      const dialog = new Adw.AlertDialog({
        heading: phase === "wifi" ? _("Join Wi-Fi") : lightName ? _("Set up %s").format(lightName) : _("Configure light"),
        body: phase === "wifi" ? _("Enter the network settings for the confirmed light.") :
          _("Put the light in pairing mode and enter its Bluetooth address on the keylightd host."),
        extra_child: fields,
      });
      dialog.add_response("cancel", _("Cancel"));
      dialog.add_response("start", phase === "address" ? _("Next") : phase === "wifi" ? _("Join Wi-Fi") : _("Start setup"));
      dialog.set_response_appearance("start", Adw.ResponseAppearance.SUGGESTED);
      dialog.set_close_response("cancel");

      const encoder = new TextEncoder();
      const update = () => {
        const secured = security.selected === 0;
        password.visible = secured && phase !== "address";
        const address = peripheral.text.trim();
        const hasAddress = /^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(address) ||
          /^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i.test(address);
        const ssidLength = encoder.encode(ssid.text).length;
        const passwordLength = encoder.encode(password.text).length;
        dialog.set_response_enabled("start",
          (phase === "wifi" || hasAddress) &&
          (phase === "address" || (ssidLength > 0 && ssidLength <= 32 &&
            (!secured || (passwordLength > 0 && passwordLength <= 64)))));
      };
      for (const entry of [peripheral, ssid, password])
        entry.connect("changed", update);
      security.connect("notify::selected", update);
      update();

      dialog.connect("response", (_dialog, response) => {
        if (response !== "start") {
          password.text = "";
          if (phase === "wifi") submit(null);
          return;
        }
        if (phase === "address") {
          submit({ peripheral: peripheral.text.trim() });
          return;
        }
        const secured = security.selected === 0;
        const options = {
          peripheral: peripheral.text.trim(),
          ssid: ssid.text,
          password: secured ? password.text : "",
          security: secured ? 200 : 0,
        };
        password.text = "";
        if (phase === "wifi") delete options.peripheral;
        submit(options);
      });
      dialog.present(parent);
      return dialog;
    }

    async _runNearbySetup(body, lightID = "nearby") {
      if (this._nearbySetupRunning) return;
      this._nearbySetupRunning = true;
      const path = `lights/${encodeURIComponent(lightID)}/setup`;
      this._clearStatus();
      try {
        let status = await fetchAPI(path, "POST", body);
        this._activeSetupPath = path;
        this._cancelSetupButton.visible = true;
        while (this.get_root()) {
          if (status.stage === "awaiting_confirmation") {
            const confirmed = await this._confirmNearbyLight(path, status.confirmationId);
            if (!confirmed) {
              await fetchAPI(path, "DELETE");
              this._setStatus(_("Setup canceled"), false);
              return;
            }
            await fetchAPI(`${path}/confirm`, "POST", { confirmationId: status.confirmationId });
          } else if (status.stage === "awaiting_wifi") {
            const network = await new Promise(resolve => this._showSetupDialog(this, resolve, null, "wifi"));
            if (!network) {
              await fetchAPI(path, "DELETE");
              this._setStatus(_("Setup canceled"), false);
              return;
            }
            await fetchAPI(`${path}/wifi`, "POST", { wifiId: status.wifiId, network });
          } else if (status.stage === "ready") {
            await this._loadLights();
            this._setStatus(_("Setup complete"), false);
            return;
          } else if (status.stage !== "configuring" && status.stage !== "canceling") {
            const failures = new Map([
              ["identity_mismatch", _("The Bluetooth light did not match the selected light.")],
              ["bluetooth_failed", _("Could not communicate over Bluetooth. Check pairing mode and try again.")],
              ["certificate_install_failed", _("The light could not install its certificates.")],
              ["wifi_credentials_rejected", _("The light rejected the Wi-Fi credentials. Check the password and security setting.")],
              ["wifi_join_failed", _("The light could not join Wi-Fi. Check the network name and signal.")],
              ["discovery_failed", _("The light joined Wi-Fi but could not be found on the network.")],
              ["verification_failed", _("Could not verify the light over the network.")],
              ["save_failed", _("Keylightd could not save the light’s credentials.")],
            ]);
            const message = failures.get(status.failureReason) ??
              (status.stage === "timed_out" ? _("Setup timed out") :
                status.stage === "canceled" ? _("Setup canceled") : _("Setup failed"));
            this._setStatus(message, status.stage !== "canceled");
            return;
          }
          this._setStatus(_("Configuring light…"), false);
          await new Promise(resolve => {
            GLib.timeout_add(GLib.PRIORITY_DEFAULT, 1000, () => {
              resolve();
              return GLib.SOURCE_REMOVE;
            });
          });
          status = await fetchAPI(path);
        }
      } catch (error) {
        this._setStatus(_("Could not complete the setup request."), true);
        filteredLog("debug", "Nearby light setup request failed", error);
      } finally {
        this._nearbySetupRunning = false;
        this._activeSetupPath = null;
        this._cancelSetupButton.visible = false;
      }
    }

    _confirmNearbyLight(path, confirmationId) {
      return new Promise(resolve => {
        const dialog = new Adw.AlertDialog({
          heading: _("Did the intended light flash?"),
          body: _("Confirm only if the light you want to configure flashed. You’ll enter Wi-Fi settings after confirming."),
        });
        const blink = new Gtk.Button({ label: _("Blink again"), halign: Gtk.Align.CENTER });
        blink.connect("clicked", async () => {
          blink.sensitive = false;
          try {
            await fetchAPI(`${path}/blink`, "POST", { confirmationId });
            blink.label = _("Blink again");
          } catch (_error) {
            blink.label = _("Could not blink");
          } finally {
            blink.sensitive = true;
          }
        });
        dialog.set_extra_child(blink);
        dialog.add_response("cancel", _("Cancel"));
        dialog.add_response("confirm", _("Confirm light"));
        dialog.set_close_response("cancel");
        dialog.set_response_appearance("confirm", Adw.ResponseAppearance.SUGGESTED);
        dialog.connect("response", (_dialog, response) => resolve(response === "confirm"));
        dialog.present(this);
      });
    }

    _createLightRow(light) {
      const row = new Adw.ActionRow({
        title: light.name,
        subtitle: _("ID: %s").format(light.id),
      });

      if (light.needsConfiguration?.length) {
        row.subtitle = _("Setup required");
        const setup = new Gtk.Button({ label: _("Configure…"), valign: Gtk.Align.CENTER });
        setup.connect("clicked", () => this._showSetupDialog(row, body => this._runNearbySetup(body, light.id), light.name, "address"));
        row.add_suffix(setup);
      }

      if (light.managedCredentials === true) {
        const forget = new Gtk.Button({
          label: _("Forget setup…"),
          valign: Gtk.Align.CENTER,
        });
        forget.connect("clicked", () => {
          const dialog = new Adw.AlertDialog({
            heading: _("Forget this light’s setup?"),
            body: _("This removes the saved credentials from keylightd. You’ll need to pair the light again to control it."),
          });
          dialog.add_response("cancel", _("Cancel"));
          dialog.add_response("forget", _("Forget setup"));
          dialog.set_response_appearance("forget", Adw.ResponseAppearance.DESTRUCTIVE);
          dialog.set_default_response("cancel");
          dialog.set_close_response("cancel");
          dialog.connect("response", async (_dialog, response) => {
            if (response !== "forget") return;
            try {
              await fetchAPI(`lights/${encodeURIComponent(light.id)}/credentials`, "DELETE");
              await this._loadLights();
            } catch (error) {
              this._setStatus(_("Could not forget this light’s setup."), true);
              filteredLog("debug", "Forget setup request failed", error);
            }
          });
          dialog.present(row);
        });
        row.add_suffix(forget);
      }

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

      const visibilityBox = new Gtk.Box({
        orientation: Gtk.Orientation.HORIZONTAL,
        margin_end: 8,
      });

      const visibilityLabel = new Gtk.Label({
        label: _("Visible"),
        margin_end: 8,
      });

      visibilityBox.append(visibilityLabel);
      visibilityBox.append(visibilitySwitch);
      row.add_suffix(visibilityBox);

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
