"use strict";

import Adw from "gi://Adw";
import Gtk from "gi://Gtk";
import GObject from "gi://GObject";

import { gettext as _ } from "resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js";

// Import shared utilities
import { fetchAPI, getErroring, log } from "../utils.js";
import { SYSTEM_PREFS_GROUPS_ICON } from "../icon-names.js";

// A dialog for adding/editing groups
const GroupDialog = GObject.registerClass(
  {
    Signals: { "group-saved": {} },
  },
  class GroupDialog extends Adw.Window {
    _init(parent, settings, group = null) {
      super._init({
        title: group ? _("Edit Group") : _("Add Group"),
        transient_for: parent,
        modal: true,
        width_request: 400,
        height_request: 250,
        hide_on_close: true,
      });

      this._settings = settings;
      this._group = group;

      const headerBar = new Adw.HeaderBar();

      // Cancel button
      const cancelButton = new Gtk.Button({
        label: _("Cancel"),
        css_classes: ["flat"],
      });
      cancelButton.connect("clicked", () => this.close());
      headerBar.pack_start(cancelButton);

      // Save button
      const saveButton = new Gtk.Button({
        label: _("Save"),
        css_classes: ["suggested-action"],
      });
      saveButton.connect("clicked", () => this._saveGroup());
      headerBar.pack_end(saveButton);

      const content = new Gtk.Box({
        orientation: Gtk.Orientation.VERTICAL,
        margin_start: 12,
        margin_end: 12,
        margin_top: 12,
        margin_bottom: 12,
        spacing: 12,
      });

      // Group name
      const nameEntry = new Gtk.Entry({
        placeholder_text: _("Group Name"),
        text: group ? group.name : "",
        margin_bottom: 12,
      });
      this._nameEntry = nameEntry;

      content.append(nameEntry);

      // Lights list with checkboxes
      const scrollWindow = new Gtk.ScrolledWindow({
        hscrollbar_policy: Gtk.PolicyType.NEVER,
        vscrollbar_policy: Gtk.PolicyType.AUTOMATIC,
        min_content_height: 150,
        vexpand: true,
      });

      this._lightsStore = new Gtk.ListStore();
      this._lightsStore.set_column_types([
        GObject.TYPE_BOOLEAN, // selected
        GObject.TYPE_STRING, // light name
        GObject.TYPE_STRING, // light id
      ]);

      const lightsView = new Gtk.TreeView({
        model: this._lightsStore,
        headers_visible: false,
        enable_search: false,
      });

      // Toggle column
      const toggleRenderer = new Gtk.CellRendererToggle();
      toggleRenderer.connect("toggled", (renderer, path) => {
        const [success, iter] = this._lightsStore.get_iter_from_string(path);
        if (success) {
          const selected = this._lightsStore.get_value(iter, 0);
          this._lightsStore.set_value(iter, 0, !selected);
        }
      });
      const toggleColumn = new Gtk.TreeViewColumn();
      toggleColumn.pack_start(toggleRenderer, false);
      toggleColumn.add_attribute(toggleRenderer, "active", 0);
      lightsView.append_column(toggleColumn);

      // Name column
      const nameRenderer = new Gtk.CellRendererText();
      const nameColumn = new Gtk.TreeViewColumn({ title: _("Light") });
      nameColumn.pack_start(nameRenderer, true);
      nameColumn.add_attribute(nameRenderer, "text", 1);
      lightsView.append_column(nameColumn);

      scrollWindow.set_child(lightsView);
      content.append(scrollWindow);

      // Error message
      this._errorLabel = new Gtk.Label({
        label: "",
        visible: false,
        css_classes: ["error"],
      });
      content.append(this._errorLabel);

      // Loading indicator
      this._spinner = new Gtk.Spinner();
      content.append(this._spinner);

      const box = new Gtk.Box({
        orientation: Gtk.Orientation.VERTICAL,
      });
      box.append(headerBar);
      box.append(content);

      this.set_content(box);

      // Load lights
      this._loadLights();
    }

    async _loadLights() {
      this._spinner.start();

      try {
        // First, fetch the list of all lights from the API
        filteredLog("info", "Fetching all lights from /lights endpoint...");
        const lightsResponse = await fetchAPI("lights");

        // Clear existing data
        this._lightsStore.clear();

        // Get current group lights if editing
        let groupLightIds = [];
        if (this._group) {
          try {
            filteredLog("info", `Fetching details for group ${this._group.id}...`);
            const groupDetails = await fetchAPI(`groups/${this._group.id}`);
            groupLightIds = groupDetails.lights || [];
            filteredLog("info", `Group ${this._group.id} has lights:`, groupLightIds);
          } catch (error) {
            filteredLog("error", `Error loading group details: ${error}`);
            this._showError(
              `${_("Failed to load group details")}: ${error.message}`,
            );
          }
        }

        // Check if response is valid
        if (!lightsResponse) {
          throw new Error(_("Failed to get lights from API"));
        }

        // Convert the response to an array of lights
        let lightsArray = [];

        if (Array.isArray(lightsResponse)) {
          // Response is already an array
          lightsArray = lightsResponse;
          filteredLog("info", "Lights endpoint returned an array of lights");
        } else if (typeof lightsResponse === "object") {
          // Response is an object with light IDs as keys
          // Convert to array format
          lightsArray = Object.entries(lightsResponse).map(
            ([id, lightData]) => {
              return {
                id: id,
                name: lightData.name || id,
                ...lightData,
              };
            },
          );
          filteredLog("info", "Converted lights object to array:", lightsArray);
        } else {
          filteredLog(
            "error",
            "Unexpected response format from lights endpoint:",
            lightsResponse,
          );
          throw new Error(_("Invalid lights data format from API"));
        }

        filteredLog(
          "info",
          `Found ${lightsArray.length} lights:`,
          lightsArray.map((l) => l.name || "unnamed"),
        );

        // Process each light
        for (const light of lightsArray) {
          if (!light || !light.id) {
            filteredLog("warn", "Invalid light entry:", light);
            continue; // Skip invalid entries
          }

          const lightId = light.id;
          const name = light.name || `Light ${lightId}`;
          const isSelected = groupLightIds.includes(lightId);

          filteredLog(
            "info",
            `Adding light "${name}" (${lightId}) to list, selected: ${isSelected}`,
          );

          const iter = this._lightsStore.append();
          this._lightsStore.set(iter, [0, 1, 2], [isSelected, name, lightId]);
        }

        this._spinner.stop();

        // Show a message if no lights were found
        if (lightsArray.length === 0) {
          this._showError(
            _("No lights found. Make sure your lights are connected."),
          );
        }
      } catch (error) {
        filteredLog("error", `Error in _loadLights: ${error}`);
        this._showError(`${_("Failed to load lights")}: ${error.message}`);
        this._spinner.stop();
      }
    }

    _showError(message) {
      this._errorLabel.label = message;
      this._errorLabel.visible = true;
    }

    _hideError() {
      this._errorLabel.visible = false;
    }

    async _saveGroup() {
      this._hideError();

      const name = this._nameEntry.text.trim();
      if (!name) {
        this._showError(_("Group name is required"));
        return;
      }

      // Collect selected lights
      const selectedLights = [];
      let [valid, iter] = this._lightsStore.get_iter_first();

      while (valid) {
        const selected = this._lightsStore.get_value(iter, 0);
        if (selected) {
          const lightId = this._lightsStore.get_value(iter, 2);
          selectedLights.push(lightId);
        }
        valid = this._lightsStore.iter_next(iter);
      }

      filteredLog("info", `Saving group with lights:`, selectedLights);

      this._spinner.start();

      try {
        const groupData = {
          name: name,
          lights: selectedLights,
        };

        let groupId;

        if (this._group) {
          // Update existing group
          filteredLog("info", `Updating group ${this._group.id} with:`, groupData);
          const endpoint = `groups/${this._group.id}/lights`;
          const method = "PUT";
          const body = { light_ids: selectedLights };
          await fetchAPI(endpoint, method, body);
          groupId = this._group.id;
        } else {
          // Create new group
          filteredLog("info", `Creating new group with:`, groupData);
          const response = await fetchAPI("groups", "POST", groupData);

          // New groups are NOT visible by default (changed from previous behavior)
          if (response && response.id) {
            groupId = response.id;
            filteredLog("info", `New group created with ID: ${groupId}`);

            // Do NOT add new group to visible groups by default
            // The user must explicitly make it visible
          } else {
            filteredLog("error", "Failed to get ID for new group:", response);
          }
        }

        this._spinner.stop();
        this.emit("group-saved");
        this.close();
      } catch (error) {
        filteredLog("error", `Error saving group: ${error}`);
        this._spinner.stop();
        this._showError(`${_("Failed to save group")}: ${error.message}`);
      }
    }
  },
);

// Main Groups Page
export var GroupsPage = GObject.registerClass(
  {
    Signals: {
      "refresh-requested": {},
    },
  },
  class GroupsPage extends Adw.PreferencesPage {
    _init(settings, settingsKey) {
      super._init({
        title: _("Groups"),
        icon_name: SYSTEM_PREFS_GROUPS_ICON,
        name: "GroupsPage",
      });

      this._settings = settings;
      this._settingsKey = settingsKey;

      // Create the groups list
      this._groupsGroup = new Adw.PreferencesGroup({
        title: _("Keylight Groups"),
        description: _("Manage groups of Key Lights"),
      });

      // Add a header with a refresh button and add button
      const headerBox = new Gtk.Box({
        orientation: Gtk.Orientation.HORIZONTAL,
        spacing: 6,
      });

      this._refreshButton = new Gtk.Button({
        icon_name: "view-refresh-symbolic",
        tooltip_text: _("Refresh Groups"),
        valign: Gtk.Align.CENTER,
      });
      this._refreshButton.connect("clicked", () => {
        try {
          this._loadGroups().catch((error) => {
            filteredLog("error", "Error loading groups:", error);
            this._setStatus(
              _("Failed to load groups. Check connection settings."),
              true,
            );
          });
        } catch (error) {
          filteredLog("error", "Error loading groups:", error);
          this._setStatus(
            _("Failed to load groups. Check connection settings."),
            true,
          );
        }
      });

      this._addButton = new Gtk.Button({
        icon_name: "list-add-symbolic",
        tooltip_text: _("Add Group"),
        valign: Gtk.Align.CENTER,
      });
      this._addButton.connect("clicked", () => this._showGroupDialog());

      headerBox.append(this._refreshButton);
      headerBox.append(this._addButton);

      this._groupsGroup.set_header_suffix(headerBox);

      // Create a visibility toggle-all action row
      this._visibilityToggleRow = new Adw.ActionRow({
        title: _("Group Visibility"),
        subtitle: _("Show or hide all groups"),
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

      showAllButton.connect("clicked", () => {
        try {
          this._toggleAllGroupsVisibility(true).catch((error) => {
            filteredLog("error", "Error showing all groups:", error);
            this._setStatus(
              _("Failed to show all groups. Check connection settings."),
              true,
            );
          });
        } catch (error) {
          filteredLog("error", "Error showing all groups:", error);
          this._setStatus(
            _("Failed to show all groups. Check connection settings."),
            true,
          );
        }
      });

      hideAllButton.connect("clicked", () => {
        try {
          this._toggleAllGroupsVisibility(false).catch((error) => {
            filteredLog("error", "Error hiding all groups:", error);
            this._setStatus(
              _("Failed to hide all groups. Check connection settings."),
              true,
            );
          });
        } catch (error) {
          filteredLog("error", "Error hiding all groups:", error);
          this._setStatus(
            _("Failed to hide all groups. Check connection settings."),
            true,
          );
        }
      });

      toggleAllBox.append(showAllButton);
      toggleAllBox.append(hideAllButton);

      this._visibilityToggleRow.add_suffix(toggleAllBox);
      this._groupsGroup.add(this._visibilityToggleRow);

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

      // Container for group rows
      this._groupsList = new Gtk.Box({
        orientation: Gtk.Orientation.VERTICAL,
        spacing: 6,
      });

      // No groups message
      this._noGroupsLabel = new Gtk.Label({
        label: _("No groups configured"),
        halign: Gtk.Align.CENTER,
        margin_top: 12,
        margin_bottom: 12,
      });

      const listBox = new Gtk.Box({
        orientation: Gtk.Orientation.VERTICAL,
      });

      listBox.append(this._statusBox);
      listBox.append(this._groupsList);
      listBox.append(this._noGroupsLabel);

      this._groupsGroup.add(listBox);
      this.add(this._groupsGroup);

      // Connect to settings changes
      this._settings.connect("changed::api-url", () => {
        try {
          this._loadGroups().catch((error) => {
            filteredLog("error", "Error loading groups after API URL change:", error);
          });
        } catch (error) {
          filteredLog("error", "Error loading groups after API URL change:", error);
        }
      });

      this._settings.connect("changed::api-key", () => {
        try {
          this._loadGroups().catch((error) => {
            filteredLog("error", "Error loading groups after API key change:", error);
          });
        } catch (error) {
          filteredLog("error", "Error loading groups after API key change:", error);
        }
      });
      // Listen for visible-groups changes and reload
      this._settings.connect("changed::visible-groups", () => {
        this._loadGroups();
      });

      // Load groups with proper error handling
      try {
        this._loadGroups().catch((error) => {
          filteredLog("error", "Initial error loading groups:", error);
          this._setStatus(
            _("Failed to load groups. Check connection settings."),
            true,
          );
          this._spinner.stop();
          this._spinner.visible = false;
        });
      } catch (error) {
        filteredLog("error", "Error in initial groups load:", error);
        this._setStatus(
          _("Failed to load groups. Check connection settings."),
          true,
        );
        this._spinner.stop();
        this._spinner.visible = false;
      }
    }

    _updateUIState() {
      const isErroring = getErroring();

      // Disable the add button and all group actions when in error state
      this._addButton.sensitive = !isErroring;
      this._refreshButton.sensitive = true; // Always allow refresh

      // Disable all group rows when in error state
      let child = this._groupsList.get_first_child();
      while (child) {
        // Find all buttons within each row
        const buttons = [];
        if (child instanceof Adw.ActionRow) {
          // Different methods for getting suffixes based on GNOME version
          if (typeof child.get_suffix_count === "function") {
            // Newer GNOME versions use explicit suffix count/access methods
            try {
              for (let i = 0; i < child.get_suffix_count(); i++) {
                const widget = child.get_suffix_at_index(i);
                if (widget instanceof Gtk.Button) {
                  buttons.push(widget);
                }
              }
            } catch (e) {
              filteredLog("error", "Error accessing suffixes by index", e);
            }
          } else if (typeof child.get_suffixes === "function") {
            // Some GNOME versions provide a get_suffixes() method
            try {
              const suffixes = child.get_suffixes();
              for (const widget of suffixes) {
                if (widget instanceof Gtk.Button) {
                  buttons.push(widget);
                }
              }
            } catch (e) {
              filteredLog("error", "Error accessing suffixes array", e);
            }
          } else {
            // Fallback method: try to find buttons in child widgets
            let childWidget = child.get_first_child();
            while (childWidget) {
              if (childWidget instanceof Gtk.Button) {
                buttons.push(childWidget);
              } else if (childWidget instanceof Gtk.Box) {
                // Look inside boxes for buttons
                let boxChild = childWidget.get_first_child();
                while (boxChild) {
                  if (boxChild instanceof Gtk.Button) {
                    buttons.push(boxChild);
                  }
                  boxChild = boxChild.get_next_sibling();
                }
              }
              childWidget = childWidget.get_next_sibling();
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

    async _loadGroups() {
      if (this._loadingGroups) return;
      this._loadingGroups = true;
      try {
        // Clear existing groups
        while (this._groupsList.get_first_child()) {
          this._groupsList.remove(this._groupsList.get_first_child());
        }

        this._spinner.visible = true;
        this._spinner.start();
        this._setStatus(_("Loading groups..."));
        this._groupsList.visible = false;
        this._noGroupsLabel.visible = false;

        let groups = await fetchAPI("groups");
        // Sort groups by name (alphanumeric, case-insensitive)
        groups = groups.sort((a, b) =>
          (a.name || "").localeCompare(b.name || "", undefined, {
            numeric: true,
            sensitivity: "base",
          }),
        );

        this._spinner.stop();
        this._spinner.visible = false;
        this._clearStatus();

        if (groups.length === 0) {
          this._noGroupsLabel.visible = true;
        } else {
          this._groupsList.visible = true;

          // Get current visible groups
          let visibleGroups = this._settings.get_strv("visible-groups");

          // Add each group to the list
          for (const group of groups) {
            const groupRow = this._createGroupRow(group);
            this._groupsList.append(groupRow);
          }
        }
      } catch (error) {
        this._spinner.stop();
        this._spinner.visible = false;
        this._setStatus(
          `${_("Failed to load groups")}: ${error.message}`,
          true,
        );
      } finally {
        this._loadingGroups = false;
      }

      // Update UI state based on error status
      this._updateUIState();
    }

    _createGroupRow(group) {
      const row = new Adw.ActionRow({
        title: group.name,
        subtitle: _("ID: %s").format(group.id),
      });

      // Get current visible groups
      const visibleGroups = this._settings.get_strv("visible-groups");
      const isVisible = visibleGroups.includes(group.id);

      // Visibility toggle
      const visibilitySwitch = new Gtk.Switch({
        active: isVisible,
        valign: Gtk.Align.CENTER,
        margin_end: 8,
      });

      visibilitySwitch.connect("notify::active", () => {
        let visibleGroups = this._settings.get_strv("visible-groups");
        if (visibilitySwitch.active) {
          // Add to visible groups if not already there
          if (!visibleGroups.includes(group.id)) {
            visibleGroups.push(group.id);
          }
        } else {
          // Remove from visible groups
          visibleGroups = visibleGroups.filter((id) => id !== group.id);
        }
        this._settings.set_strv("visible-groups", visibleGroups);
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

      // Lights count
      const lightsCount = new Gtk.Label({
        label: group.lights ? group.lights.length.toString() : "0",
        css_classes: ["dim-label", "numeric"],
        margin_end: 8,
      });
      row.add_suffix(lightsCount);

      // Edit button
      const editButton = new Gtk.Button({
        icon_name: "document-edit-symbolic",
        tooltip_text: _("Edit"),
        valign: Gtk.Align.CENTER,
        margin_end: 8,
      });
      editButton.connect("clicked", () => this._showGroupDialog(group));
      row.add_suffix(editButton);

      // Delete button
      const deleteButton = new Gtk.Button({
        icon_name: "user-trash-symbolic",
        tooltip_text: _("Delete"),
        valign: Gtk.Align.CENTER,
      });
      deleteButton.connect("clicked", () => this._confirmDeleteGroup(group));
      row.add_suffix(deleteButton);

      return row;
    }

    _showGroupDialog(group = null) {
      const dialog = new GroupDialog(this.get_root(), this._settings, group);
      dialog.connect("group-saved", () => this._loadGroups());
      dialog.present();
    }

    _confirmDeleteGroup(group) {
      const dialog = new Adw.MessageDialog({
        heading: _("Delete Group"),
        body: _('Are you sure you want to delete the group "%s"?').format(
          group.name,
        ),
        transient_for: this.get_root(),
        modal: true,
      });

      dialog.add_response("cancel", _("Cancel"));
      dialog.add_response("delete", _("Delete"));
      dialog.set_response_appearance(
        "delete",
        Adw.ResponseAppearance.DESTRUCTIVE,
      );

      dialog.connect("response", async (dialog, response) => {
        if (response === "delete") {
          try {
            filteredLog("info", `Deleting group ${group.id}`);

            const result = await fetchAPI(`groups/${group.id}`, "DELETE");

            filteredLog("debug", `Delete result:`, result);

            // Update visible groups if needed
            const visibleGroups = this._settings.get_strv("visible-groups");
            if (visibleGroups.includes(group.id)) {
              const updatedVisibleGroups = visibleGroups.filter(
                (id) => id !== group.id,
              );
              this._settings.set_strv("visible-groups", updatedVisibleGroups);
            }

            // Reload the groups list
            this._loadGroups();
          } catch (error) {
            filteredLog("error", `Failed to delete group: ${error}`);
            this._setStatus(
              `${_("Failed to delete group")}: ${error.message}`,
              true,
            );
          }
        }
        dialog.destroy();
      });

      dialog.present();
    }

    // Toggle visibility for all groups
    async _toggleAllGroupsVisibility(visible) {
      try {
        let groups = await fetchAPI("groups");
        // Sort groups by name (alphanumeric, case-insensitive)
        groups = groups.sort((a, b) =>
          (a.name || "").localeCompare(b.name || "", undefined, {
            numeric: true,
            sensitivity: "base",
          }),
        );
        if (groups && groups.length > 0) {
          // First collect all group IDs
          const allGroupIds = groups.map((group) => group.id);

          // Then update the setting
          if (visible) {
            // Show all groups
            filteredLog("info", `Setting all ${allGroupIds.length} groups to visible`);
            this._settings.set_strv("visible-groups", allGroupIds);
          } else {
            // Hide all groups
            filteredLog("info", "Hiding all groups");
            this._settings.set_strv("visible-groups", []);
          }
          // No direct _loadGroups() call here; rely on settings signal
        } else {
          this._setStatus(_("No groups found"));
        }
      } catch (error) {
        filteredLog("error", "Error toggling all groups visibility:", error);
        this._setStatus(
          `${_("Failed to toggle visibility")}: ${error.message}`,
          true,
        );
      }
    }
  },
);
