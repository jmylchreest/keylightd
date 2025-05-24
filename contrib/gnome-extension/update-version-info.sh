#!/bin/bash

# Script to update version-info.json for the GNOME extension
# This script is meant to be run by the Makefile before building the extension

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION_FILE="$SCRIPT_DIR/keylightd-control@jmylchreest.github.io/version-info.json"

# Default values
PROJECT_NAME="keylightd gnome-extension"
ABOUT="GNOME Shell extension for controlling Elgato Key Light devices through keylightd daemon"
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
echo "  Version: $VERSION"
echo "  Commit: $COMMIT"
echo "  File: $VERSION_FILE"