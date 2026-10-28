#!/bin/bash
# Upload the release bundle using a known GNOME CLI with upload support.
set -euo pipefail

if [[ $# != 1 || ! -f "$1" ]]; then
    echo "Usage: EGO_USERNAME=... EGO_PASSWORD=... $0 extension.shell-extension.zip" >&2
    exit 2
fi
: "${EGO_USERNAME:?Set EGO_USERNAME to the extensions.gnome.org account name}"
: "${EGO_PASSWORD:?Set EGO_PASSWORD to the extensions.gnome.org password}"
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
BUNDLE_DIR=$(cd "$(dirname "$1")" && pwd)
BUNDLE_NAME=$(basename "$1")
RUNTIME=${CONTAINER_RUNTIME:-podman}
IMAGE=localhost/keylightd-gnome51-runtime
"$RUNTIME" build --target runtime -t "$IMAGE" -f "$ROOT/tests/gnome-shell/Containerfile" "$ROOT"
# stdin avoids placing the password in the container arguments or environment.
printf '%s\n' "$EGO_PASSWORD" | "$RUNTIME" run --rm -i \
    --volume "$BUNDLE_DIR:/bundles:ro" --entrypoint gnome-extensions "$IMAGE" \
    upload --accept-tos --user "$EGO_USERNAME" --password-file=- "/bundles/$BUNDLE_NAME"
