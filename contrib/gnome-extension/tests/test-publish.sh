#!/bin/bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TEMP=$(mktemp -d)
trap 'rm -rf "$TEMP"' EXIT
touch "$TEMP/test.shell-extension.zip"

# Exercise the real wrapper without a network connection or EGO credentials.
cat > "$TEMP/runtime" <<'EOF'
#!/bin/bash
set -euo pipefail
if [[ $1 == build ]]; then
    [[ " $* " == *" --target runtime "* ]]
    exit
fi
[[ $1 == run ]]
[[ " $* " == *" --entrypoint gnome-extensions "* ]]
[[ " $* " == *" upload --accept-tos --user test-user --password-file=- "* ]]
[[ " $* " != *"test-password"* ]]
[[ ${!#} == /bundles/test.shell-extension.zip ]]
read -r password
[[ $password == test-password ]]
EOF
chmod +x "$TEMP/runtime"
EGO_USERNAME=test-user EGO_PASSWORD=test-password CONTAINER_RUNTIME="$TEMP/runtime" \
    bash "$ROOT/publish.sh" "$TEMP/test.shell-extension.zip"
if env -u EGO_PASSWORD EGO_USERNAME=test-user CONTAINER_RUNTIME="$TEMP/runtime" \
    bash "$ROOT/publish.sh" "$TEMP/test.shell-extension.zip" >/dev/null 2>&1; then
    echo 'FAIL: publishing accepted missing credentials' >&2
    exit 1
fi
if bash "$ROOT/publish.sh" "$TEMP/missing.zip" >/dev/null 2>&1; then
    echo 'FAIL: publishing accepted a missing bundle' >&2
    exit 1
fi
echo 'PASS: release bundle upload arguments, stdin credentials and input validation'
