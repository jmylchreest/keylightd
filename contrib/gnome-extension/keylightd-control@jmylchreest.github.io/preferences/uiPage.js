'use strict';

import GObject from 'gi://GObject';
import Gio from 'gi://Gio';
import Gtk from 'gi://Gtk';
import Adw from 'gi://Adw';
import {gettext as _} from 'resource:///org/gnome/Shell/Extensions/js/extensions/prefs.js';
import { SYSTEM_PREFS_UI_ICON } from '../icon-names.js';

export const UIPage = GObject.registerClass(
    class UIPage extends Adw.PreferencesPage {
        _init(settings) {
            super._init({
                title: _('UI'),
                icon_name: SYSTEM_PREFS_UI_ICON
            });

            this._settings = settings;
            
            // UI Group
            const uiGroup = new Adw.PreferencesGroup({
                title: _('User Interface')
            });
            this.add(uiGroup);

            // Animations toggle
            const animationsRow = new Adw.ActionRow({
                title: _('Use Animations'),
                subtitle: _('Enable smooth transitions when toggling lights')
            });
            
            const animationsToggle = new Gtk.Switch({
                active: this._settings.get_boolean('use-animations'),
                valign: Gtk.Align.CENTER,
            });
            
            animationsToggle.connect('notify::active', (widget) => {
                this._settings.set_boolean('use-animations', widget.get_active());
            });
            
            animationsRow.add_suffix(animationsToggle);
            animationsRow.activatable_widget = animationsToggle;
            uiGroup.add(animationsRow);
            
            // Animation duration
            const animationSpeedRow = new Adw.ActionRow({
                title: _('Animation Speed'),
                subtitle: _('Speed of transitions when toggling lights')
            });
            
            const animationSpeedAdjustment = new Gtk.Adjustment({
                lower: 100,
                upper: 1000,
                step_increment: 50,
                value: this._settings.get_int('animation-duration')
            });
            
            const animationSpeedScale = new Gtk.Scale({
                adjustment: animationSpeedAdjustment,
                draw_value: true,
                value_pos: Gtk.PositionType.RIGHT,
                width_request: 200,
                hexpand: true,
                digits: 0
            });
            
            animationSpeedScale.add_mark(100, Gtk.PositionType.BOTTOM, _('Fast'));
            animationSpeedScale.add_mark(500, Gtk.PositionType.BOTTOM, _('Medium'));
            animationSpeedScale.add_mark(1000, Gtk.PositionType.BOTTOM, _('Slow'));
            
            animationSpeedScale.connect('value-changed', (widget) => {
                this._settings.set_int('animation-duration', widget.get_value());
            });
            
            animationSpeedRow.add_suffix(animationSpeedScale);
            uiGroup.add(animationSpeedRow);
            
            // Update UI when animation toggle changes
            this._settings.connect('changed::use-animations', () => {
                const useAnimations = this._settings.get_boolean('use-animations');
                animationSpeedRow.sensitive = useAnimations;
            });
            
            // Initialize animation speed row sensitivity
            animationSpeedRow.sensitive = this._settings.get_boolean('use-animations');
            
            // Max Height Percent
            const maxHeightRow = new Adw.ActionRow({
                title: _('Max List Height'),
                subtitle: _('Maximum percent of screen height for group/lights list')
            });
            const maxHeightAdjustment = new Gtk.Adjustment({
                lower: 0.20,
                upper: 0.60,
                step_increment: 0.01,
                value: this._settings.get_double('max-height-percent'),
            });
            const maxHeightScale = new Gtk.Scale({
                adjustment: maxHeightAdjustment,
                draw_value: true,
                value_pos: Gtk.PositionType.RIGHT,
                width_request: 200,
                hexpand: true,
                digits: 2
            });
            maxHeightScale.connect('value-changed', (widget) => {
                this._settings.set_double('max-height-percent', widget.get_value());
            });
            // Show as percent
            maxHeightScale.set_format_value_func((scale, value) => `${Math.round(value * 100)}%`);
            maxHeightRow.add_suffix(maxHeightScale);
            uiGroup.add(maxHeightRow);
        }
    }
); 