#!/bin/bash
set -euo pipefail

ROOT=/opt/keylightd-extension
if [[ ! -d "$ROOT" || ( ! -e /run/.containerenv && ! -e /.dockerenv ) ]]; then
    echo "Run tests/test-gnome-shell.sh to launch this script in its container." >&2
    exit 2
fi
ulimit -c 0
# GNOME expects a system bus even when desktop services are absent. This bus
# lives entirely inside the container; it is never the host system bus.
mkdir -p /run/dbus
dbus-uuidgen --ensure
dbus-daemon --system --fork
# Package hooks create this directory, but no logind runs in this container.
# Let GNOME select its built-in fallback login manager.
rm -rf /run/systemd/seats
UUID=keylightd-control@jmylchreest.github.io
PROBE=keylightd-test@local
export HOME=/tmp/keylightd-test
export XDG_RUNTIME_DIR=$HOME/runtime
export XDG_CONFIG_HOME=$HOME/config
export XDG_DATA_HOME=$HOME/data
export XDG_CACHE_HOME=$HOME/cache
export KEYLIGHTD_TEST_ARTIFACTS=$HOME/artifacts
mkdir -p "$KEYLIGHTD_TEST_ARTIFACTS"
trap 'cp "$HOME/"*.log "$KEYLIGHTD_TEST_ARTIFACTS/" 2>/dev/null || true' EXIT
mkdir -p "$XDG_RUNTIME_DIR" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME/gnome-shell/extensions"
chmod 700 "$XDG_RUNTIME_DIR"
cp -r "$ROOT/$UUID" "$XDG_DATA_HOME/gnome-shell/extensions/$UUID"
glib-compile-schemas "$XDG_DATA_HOME/gnome-shell/extensions/$UUID/schemas"
export GSETTINGS_SCHEMA_DIR="$XDG_DATA_HOME/gnome-shell/extensions/$UUID/schemas"
gsettings set org.gnome.shell.extensions.keylightd-control api-url http://127.0.0.1:9999
gsettings set org.gnome.shell.extensions.keylightd-control api-key container-test-key
gsettings set org.gnome.shell.extensions.keylightd-control visible-lights "['key', 'fill']"
gsettings set org.gnome.shell.extensions.keylightd-control visible-groups "['studio']"
gsettings set org.gnome.desktop.interface enable-animations false
gsettings set org.gnome.desktop.notifications show-banners false
python "$ROOT/tests/gnome-shell/fixture-api.py" > "$HOME/fixture-api.log" 2>&1 &
mkdir "$XDG_DATA_HOME/gnome-shell/extensions/$PROBE"
cp "$ROOT/tests/gnome-shell/probe.js" "$XDG_DATA_HOME/gnome-shell/extensions/$PROBE/extension.js"
printf '{"uuid":"%s","name":"Keylightd test","description":"Container-only regression probe","shell-version":["%s"]}\n' "$PROBE" "$GNOME_VERSION" > "$XDG_DATA_HOME/gnome-shell/extensions/$PROBE/metadata.json"
gnome-shell --version
pacman -Q gnome-shell mutter gjs mesa | tee "$KEYLIGHTD_TEST_ARTIFACTS/versions.txt"
gsettings set org.gnome.shell enabled-extensions "['$PROBE', '$UUID']"
gsettings set org.gnome.shell disable-user-extensions false
# Reproduce the report on an older checkout without editing its metadata.
if [[ ${1:-} == --allow-unsupported ]]; then
    gsettings set org.gnome.shell disable-extension-version-validation true
elif [[ $# != 0 ]]; then
    echo "Usage: session.sh [--allow-unsupported]" >&2
    exit 2
fi

if dbus-run-session -- bash -c '
    gnome-shell --headless --wayland --no-x11 --virtual-monitor 1600x1200 > "$HOME/shell.log" 2>&1 &
    shell_pid=$!
    trap "kill $shell_pid 2>/dev/null || true; wait $shell_pid 2>/dev/null || true" EXIT
    for ((i=0; i<120; i++)); do
        if [[ -f "$HOME/result" ]]; then
            cat "$HOME/result"
            if grep -q "^PASS:" "$HOME/result"; then
                exit 0
            fi
            cat "$HOME/shell.log"
            exit 1
        fi
        if ! kill -0 "$shell_pid" 2>/dev/null; then
            break
        fi
        sleep 1
    done
    echo "FAIL: GNOME Shell exited or the probe timed out"
    cat "$HOME/shell.log"
    exit 1
' > "$HOME/session.log" 2>&1; then
    if grep -E 'JS ERROR|\[keylightd-control \(ERROR\)\]|Extension keylightd-control.*Error|clutter_actor_remove_child: assertion' "$HOME/shell.log"; then
        echo 'FAIL: extension or preferences errors were logged during the test'
        exit 1
    fi
    cat "$HOME/result"
else
    cat "$HOME/session.log"
    exit 1
fi
