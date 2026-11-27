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
    latest)
        ARCH_SNAPSHOT=
        ARCH_IMAGE=docker.io/library/archlinux:latest
        ;;
    *) echo 'Supported GNOME_VERSION values: 48, 49, 50, 51, latest' >&2; exit 2 ;;
esac
BUILD_FLAGS=()
if [[ $GNOME_VERSION == latest && ( ${1:-} == --build-base || -z ${GNOME_TEST_IMAGE:-} ) ]]; then
    "$RUNTIME" pull "$ARCH_IMAGE"
    BUILD_FLAGS+=(--no-cache)
fi
if [[ ${1:-} == --build-base ]]; then
    TAG=$GNOME_VERSION
    if [[ -n $ARCH_SNAPSHOT ]]; then
        TAG+="-${ARCH_SNAPSHOT//\//}"
    fi
    IMAGE="ghcr.io/jmylchreest/keylightd-gnome-test:$TAG"
    "$RUNTIME" build "${BUILD_FLAGS[@]}" --target test-base -t "$IMAGE" \
        --label org.opencontainers.image.source=https://github.com/jmylchreest/keylightd \
        --build-arg "GNOME_VERSION=$GNOME_VERSION" --build-arg "ARCH_SNAPSHOT=$ARCH_SNAPSHOT" \
        --build-arg "ARCH_IMAGE=$ARCH_IMAGE" -f "$ROOT/tests/gnome-shell/Containerfile" "$ROOT"
    "$RUNTIME" run --rm --network=none "$IMAGE" gnome-shell --version
    if [[ ${2:-} == --push ]]; then
        "$RUNTIME" push "$IMAGE"
    fi
    exit
fi
IMAGE=localhost/keylightd-gnome${GNOME_VERSION}-test
mkdir -p "$ROOT/../../dist"
ARTIFACTS=$(mktemp -d "$ROOT/../../dist/gnome${GNOME_VERSION}-XXXXXXXX")
CID_FILE=$(mktemp)
rm "$CID_FILE"
trap 'if [[ -s "$CID_FILE" ]]; then "$RUNTIME" rm -f "$(cat "$CID_FILE")" >/dev/null; fi; rm -f "$CID_FILE"' EXIT
# No host display, D-Bus, GPU, home directory or network is exposed to the test.
if [[ -n ${GNOME_TEST_IMAGE:-} ]]; then
    "$RUNTIME" pull "$GNOME_TEST_IMAGE"
    "$RUNTIME" create --cidfile "$CID_FILE" --network=none \
        --volume "$ROOT:/opt/keylightd-extension:ro" "$GNOME_TEST_IMAGE" \
        bash /opt/keylightd-extension/tests/gnome-shell/session.sh "$@" >/dev/null
else
    "$RUNTIME" build "${BUILD_FLAGS[@]}" -t "$IMAGE" --build-arg "GNOME_VERSION=$GNOME_VERSION" \
        --build-arg "ARCH_SNAPSHOT=$ARCH_SNAPSHOT" --build-arg "ARCH_IMAGE=$ARCH_IMAGE" \
        -f "$ROOT/tests/gnome-shell/Containerfile" "$ROOT"
    "$RUNTIME" create --cidfile "$CID_FILE" --network=none "$IMAGE" "$@" >/dev/null
fi
status=0
"$RUNTIME" start -a "$(cat "$CID_FILE")" || status=$?
"$RUNTIME" cp "$(cat "$CID_FILE"):/tmp/keylightd-test/artifacts/." "$ARTIFACTS/" || status=1
echo "Screenshots and logs: $ARTIFACTS"
exit "$status"
