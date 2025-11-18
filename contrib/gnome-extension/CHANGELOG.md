# Changelog

All notable changes to the Keylightd Control GNOME extension will be documented in this file.

The format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and versioning is aligned with extension releases (the `metadata.json` `version` field).  
Items under "Unreleased" will move to a tagged version upon release packaging.

---

## [Unreleased]

### Added
- Declared compatibility with GNOME Shell 49 (`"shell-version": ["48", "49"]` in `metadata.json`).

### Changed
- Updated development / debugging instructions in `README.md` to recommend:
  - `dbus-run-session -- gnome-shell --devkit` for GNOME Shell 49+ nested/devkit sessions.
  - Retained legacy `--nested --wayland` command reference for older shells.
- Minor internal cleanup: removed redundant `add_child` call when inserting the panel indicator (in `extension.js`). The indicator insertion is now handled solely by the logic in `KeylightdControl` (via its `_addToQuickSettings()` path), preventing a no-op duplicate add.

### Technical Notes
- No GNOME 49 API adjustments were required beyond metadata since the extension:
  - Does not use deprecated `Meta.Rectangle` / replaced geometry APIs.
  - Does not rely on removed `Clutter.ClickAction` / `TapAction`.
  - Does not depend on removed `AppMenuButton` or calendar / DND legacy APIs.
- Existing Quick Settings integration (`QuickSettings.SystemIndicator`, `QuickSettings.QuickMenuToggle`) remains compatible with 49.
- Background refresh logic and state synchronization unchanged; tested logic requires no adaptation for new `brightnessManager` or other Shell service changes.

### Developer Follow‑Ups (Optional)
- Consider adding automated CI matrix to validate packaging against multiple GNOME Shell versions (48, 49, future 50).
- Potential enhancement: add a small runtime self-test command (e.g. logging environment info when `enable()` runs with a debug flag).

---

## [1] - Initial Public Version
(Implicit baseline from repository history prior to formal CHANGELOG introduction)

### Features
- Quick Settings indicator and toggle.
- Individual & group light control (power, brightness, temperature).
- Preferences dialog with multiple pages (General, Groups, Lights, UI, About).
- Automatic version info embedding (`version-info.json`).
- Configurable polling interval, debounce delay, animations, and logging.
- Dynamic icon state based on visible lights/groups.

---

## Legend
- Added: New features.
- Changed: Adjustments to existing behavior.
- Fixed: Bug fixes.
- Removed: Deletions / deprecations.
- Technical Notes: Developer-facing context or migration notes.

---