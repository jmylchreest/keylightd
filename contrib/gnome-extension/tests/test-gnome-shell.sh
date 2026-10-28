#!/bin/bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
RUNTIME=${CONTAINER_RUNTIME:-podman}
GNOME_VERSION=${GNOME_VERSION:-51}
ARCH_IMAGE=docker.io/library/archlinux@sha256:dfc5be7be44cf1acf3d9b3b902e3e9c8a29f97ed41e35631823cdc9c97e5d4f0
# The February 2026 base retains certifications for historical package signers.
# A current keyring disables one of them; keep signature checking enabled and
# let pacman downgrade the entire runtime to the selected archive snapshot.
case "$GNOME_VERSION" in
    48)
        ARCH_SNAPSHOT=2025/08/01
        ARCH_IMAGE=docker.io/library/archlinux@sha256:9b8e02df0f0a08fa8144d901932bd85d751257f83e00314053314a27ff87884f
        ;;
    49)
        ARCH_SNAPSHOT=2026/02/01
        ARCH_IMAGE=docker.io/library/archlinux@sha256:9b8e02df0f0a08fa8144d901932bd85d751257f83e00314053314a27ff87884f
        ;;
    50) ARCH_SNAPSHOT=2026/08/01 ;;
    51) ARCH_SNAPSHOT=2026/10/28 ;;
    *) echo 'Supported GNOME_VERSION values: 48, 49, 50, 51' >&2; exit 2 ;;
esac
IMAGE=localhost/keylightd-gnome${GNOME_VERSION}-test
mkdir -p "$ROOT/../../dist"
ARTIFACTS=$(mktemp -d "$ROOT/../../dist/gnome${GNOME_VERSION}-XXXXXXXX")
CID_FILE=$(mktemp)
rm "$CID_FILE"
trap 'if [[ -s "$CID_FILE" ]]; then "$RUNTIME" rm -f "$(cat "$CID_FILE")" >/dev/null; fi; rm -f "$CID_FILE"' EXIT
"$RUNTIME" build -t "$IMAGE" --build-arg "GNOME_VERSION=$GNOME_VERSION" \
    --build-arg "ARCH_SNAPSHOT=$ARCH_SNAPSHOT" --build-arg "ARCH_IMAGE=$ARCH_IMAGE" \
    -f "$ROOT/tests/gnome-shell/Containerfile" "$ROOT"
# No host display, D-Bus, GPU, home directory or network is exposed to the test.
"$RUNTIME" create --cidfile "$CID_FILE" --network=none "$IMAGE" "$@" >/dev/null
status=0
"$RUNTIME" start -a "$(cat "$CID_FILE")" || status=$?
"$RUNTIME" cp "$(cat "$CID_FILE"):/tmp/keylightd-test/artifacts/." "$ARTIFACTS/" || status=1
echo "Screenshots and logs: $ARTIFACTS"
exit "$status"
