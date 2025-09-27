#!/bin/bash

# Script to update version-info.json for the GNOME extension
# This script is meant to be run by the Makefile before building the extension

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION_FILE="$SCRIPT_DIR/keylightd-control@jmylchreest.github.io/version-info.json"
METADATA_FILE="$SCRIPT_DIR/keylightd-control@jmylchreest.github.io/metadata.json"

# Default values
PROJECT_NAME="keylightd gnome-extension"
ABOUT="GNOME Shell extension for controlling Key Light devices through keylightd daemon"
VERSION="development"
COMMIT="unknown"

# Try to get version from git tag or environment
if [ -n "$GITHUB_REF" ] && [[ "$GITHUB_REF" == refs/tags/* ]]; then
    # Extract version from git tag (remove 'refs/tags/' prefix and any 'v' prefix)
    VERSION="${GITHUB_REF#refs/tags/}"
    VERSION="${VERSION#v}"
elif [ -n "$GITHUB_REF_NAME" ]; then
    # Use the ref name (branch or tag name)
    VERSION="$GITHUB_REF_NAME"
elif command -v git >/dev/null 2>&1; then
    # Try to get version from git describe
    if git describe --tags --exact-match >/dev/null 2>&1; then
        VERSION=$(git describe --tags --exact-match | sed 's/^v//')
    elif git describe --tags >/dev/null 2>&1; then
        VERSION=$(git describe --tags | sed 's/^v//')
    else
        # Use branch name if no tags
        VERSION=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "development")
    fi
fi

# Try to get commit hash
if [ -n "$GITHUB_SHA" ]; then
    COMMIT="$GITHUB_SHA"
elif command -v git >/dev/null 2>&1; then
    COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
fi

# Truncate commit hash to first 7 characters if it's a full hash
if [[ ${#COMMIT} -eq 40 ]]; then
    COMMIT="${COMMIT:0:7}"
fi

# Normalize VERSION (remove any leading 'v' again defensively)
VERSION="${VERSION#v}"

# Derive integer extension version from semantic version (MAJOR.MINOR.PATCH)
# Encoding scheme: MAJOR * 10000 + MINOR * 100 + PATCH
if [[ "$VERSION" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
    MAJOR="${BASH_REMATCH[1]}"
    MINOR="${BASH_REMATCH[2]}"
    PATCH="${BASH_REMATCH[3]}"
else
    # Fallback if version is not strict semver
    MAJOR=0
    MINOR=0
    PATCH=0
fi
# Overflow guard: ensure MINOR and PATCH stay within 0-99 to avoid encoding collisions.
if [ "$MINOR" -ge 100 ] || [ "$PATCH" -ge 100 ]; then
    echo "Version component overflow: minor=$MINOR patch=$PATCH exceed encoding limits (0-99). Update encoding scheme before releasing." >&2
    exit 1
fi
EXT_INT_VERSION=$(( MAJOR * 10000 + MINOR * 100 + PATCH ))

# Create the version-info.json file
cat > "$VERSION_FILE" << EOF
{
  "project_name": "$PROJECT_NAME",
  "about": "$ABOUT",
  "version": "$VERSION",
  "commit": "$COMMIT"
}
EOF

echo "Updated version info:"
echo "  Version (semver): $VERSION"
echo "  Commit: $COMMIT"
echo "  File: $VERSION_FILE"
echo "  Computed extension integer version: $EXT_INT_VERSION"

# Update metadata.json version field (integer for EGO)
if [ -f "$METADATA_FILE" ]; then
    if command -v jq >/dev/null 2>&1; then
        TMP_META="$(mktemp)"
        jq --argjson v "$EXT_INT_VERSION" '.version = $v' "$METADATA_FILE" > "$TMP_META"
        mv "$TMP_META" "$METADATA_FILE"
    else
        # Sed fallback (simple pattern replacement)
        sed -i -E "s/\"version\": *[0-9]+/\"version\": ${EXT_INT_VERSION}/" "$METADATA_FILE"
    fi
    echo "metadata.json updated with version: $EXT_INT_VERSION"
else
    echo "WARNING: metadata.json not found at $METADATA_FILE"
fi
