"use strict";

import GObject from "gi://GObject";
import Gtk from "gi://Gtk";
import Adw from "gi://Adw";
import { gettext as _ } from "resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js";
import { SYSTEM_PREFS_UI_ICON } from "../icon-names.js";

export const UIPage = GObject.registerClass(
  class UIPage extends Adw.PreferencesPage {
    _init(settings) {
      super._init({
        title: _("Display"),
        icon_name: SYSTEM_PREFS_UI_ICON,
      });

      this._settings = settings;

      // Refresh Settings
      // --------------
      let refreshSettingsGroup = new Adw.PreferencesGroup({
        title: _("Updates"),
        description: _(
          "Configure how often lights and groups status are updated",
        ),
      });

      // Create an Adw.SpinRow for refresh interval - much better than using a custom slider for this use case
      const refreshRow = new Adw.SpinRow({
        title: _("Refresh Interval"),
        subtitle: _("Time in seconds between automatic status updates"),
        adjustment: new Gtk.Adjustment({
          lower: 5, // Minimum 5 seconds
          upper: 300, // Maximum 5 minutes (300 seconds)
          step_increment: 5, // Step by 5 seconds
          page_increment: 30,
          value: this._settings.get_int("refresh-interval"),
        }),
        digits: 0,
        value: this._settings.get_int("refresh-interval"),
      });

      // Update the setting when the row value changes
      refreshRow.connect("notify::value", () => {
        this._settings.set_int("refresh-interval", refreshRow.get_value());
      });

      refreshSettingsGroup.add(refreshRow);

      // Create an Adw.SpinRow for debounce delay
      const debounceRow = new Adw.SpinRow({
        title: _("Control Update Delay"),
        subtitle: _("Delay in milliseconds before sending updates to lights"),
        adjustment: new Gtk.Adjustment({
          lower: 0, // Minimum 0 ms (immediate)
          upper: 2000, // Maximum 2 seconds
          step_increment: 50, // Step by 50 ms
          page_increment: 200,
          value: this._settings.get_int("debounce-delay"),
        }),
        digits: 0,
        value: this._settings.get_int("debounce-delay"),
      });

      // Update the setting when the row value changes
      debounceRow.connect("notify::value", () => {
        this._settings.set_int("debounce-delay", debounceRow.get_value());
      });

      // Add description
      debounceRow.set_tooltip_text(
        _(
          "Higher values reduce network traffic but make controls less responsive",
        ),
      );

      refreshSettingsGroup.add(debounceRow);

      // Add a suffix to indicate units
      const refreshSuffix = new Gtk.Label({
        label: _("seconds"),
        margin_start: 8,
        css_classes: ["dim-label"],
      });
      refreshRow.add_suffix(refreshSuffix);

      const debounceSuffix = new Gtk.Label({
        label: _("milliseconds"),
        margin_start: 8,
        css_classes: ["dim-label"],
      });
      debounceRow.add_suffix(debounceSuffix);

      this.add(refreshSettingsGroup);

      // UI Group
      const uiGroup = new Adw.PreferencesGroup({
        title: _("Quick Settings"),
      });
      this.add(uiGroup);

      // Animations toggle
      const animationsRow = new Adw.ActionRow({
        title: _("Use Animations"),
        subtitle: _("Enable smooth transitions when toggling lights"),
      });

      const animationsToggle = new Gtk.Switch({
        active: this._settings.get_boolean("use-animations"),
        valign: Gtk.Align.CENTER,
      });

      animationsToggle.connect("notify::active", (widget) => {
        this._settings.set_boolean("use-animations", widget.get_active());
      });

      animationsRow.add_suffix(animationsToggle);
      animationsRow.activatable_widget = animationsToggle;
      uiGroup.add(animationsRow);

      // Animation duration
      const animationSpeedRow = new Adw.ActionRow({
        title: _("Animation Speed"),
        subtitle: _("Speed of transitions when toggling lights"),
      });

      const animationSpeedAdjustment = new Gtk.Adjustment({
        lower: 100,
        upper: 1000,
        step_increment: 50,
        value: this._settings.get_int("animation-duration"),
      });

      const animationSpeedScale = new Gtk.Scale({
        adjustment: animationSpeedAdjustment,
        draw_value: true,
        value_pos: Gtk.PositionType.RIGHT,
        width_request: 200,
        hexpand: true,
        digits: 0,
      });

      animationSpeedScale.add_mark(100, Gtk.PositionType.BOTTOM, _("Fast"));
      animationSpeedScale.add_mark(500, Gtk.PositionType.BOTTOM, _("Medium"));
      animationSpeedScale.add_mark(1000, Gtk.PositionType.BOTTOM, _("Slow"));

      animationSpeedScale.connect("value-changed", (widget) => {
        this._settings.set_int("animation-duration", widget.get_value());
      });

      animationSpeedRow.add_suffix(animationSpeedScale);
      uiGroup.add(animationSpeedRow);

      // Update UI when animation toggle changes
      this._settings.connect("changed::use-animations", () => {
        const useAnimations = this._settings.get_boolean("use-animations");
        animationSpeedRow.visible = useAnimations;
      });

      // Initialize animation speed row sensitivity
      animationSpeedRow.visible =
        this._settings.get_boolean("use-animations");

      // Max Height Percent
      const maxHeightRow = new Adw.ActionRow({
        title: _("Max List Height"),
        subtitle: _("Maximum percent of screen height for group/lights list"),
      });
      const maxHeightAdjustment = new Gtk.Adjustment({
        lower: 0.2,
        upper: 0.6,
        step_increment: 0.01,
        value: this._settings.get_double("max-height-percent"),
      });
      const maxHeightScale = new Gtk.Scale({
        adjustment: maxHeightAdjustment,
        draw_value: true,
        value_pos: Gtk.PositionType.RIGHT,
        width_request: 200,
        hexpand: true,
        digits: 2,
      });
      maxHeightScale.connect("value-changed", (widget) => {
        this._settings.set_double("max-height-percent", widget.get_value());
      });
      // Show as percent
      maxHeightScale.set_format_value_func(
        (scale, value) => `${Math.round(value * 100)}%`,
      );
      maxHeightRow.add_suffix(maxHeightScale);
      uiGroup.add(maxHeightRow);
    }
  },
);
