#!/bin/bash

# Test script to verify version info functionality
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXT_DIR="$SCRIPT_DIR/../keylightd-control@jmylchreest.github.io"

echo "Testing version info functionality..."

# Test 1: Check if update script exists and is executable
echo "Test 1: Checking update script..."
if [ ! -f "$SCRIPT_DIR/../update-version-info.sh" ]; then
    echo "FAIL: update-version-info.sh not found"
    exit 1
fi

if [ ! -x "$SCRIPT_DIR/../update-version-info.sh" ]; then
    echo "FAIL: update-version-info.sh is not executable"
    exit 1
fi
echo "PASS: Update script exists and is executable"

# Test 2: Check if Makefile exists and has version-info target
echo "Test 2: Checking Makefile..."
if [ ! -f "$SCRIPT_DIR/../Makefile" ]; then
    echo "FAIL: Makefile not found"
    exit 1
fi

if ! grep -q "version-info:" "$SCRIPT_DIR/../Makefile"; then
    echo "FAIL: Makefile missing version-info target"
    exit 1
fi
echo "PASS: Makefile exists with version-info target"

# Test 3: Check if aboutPage.js exists and is properly structured
echo "Test 3: Checking aboutPage.js..."
ABOUT_PAGE="$EXT_DIR/preferences/aboutPage.js"
if [ ! -f "$ABOUT_PAGE" ]; then
    echo "FAIL: aboutPage.js not found"
    exit 1
fi

# Check for proper class structure
if ! grep -q "class AboutPage extends Adw.PreferencesPage" "$ABOUT_PAGE"; then
    echo "FAIL: aboutPage.js missing proper class structure"
    exit 1
fi

# Check for version info loading
if ! grep -q "_loadVersionInfo" "$ABOUT_PAGE"; then
    echo "FAIL: aboutPage.js missing version info loading"
    exit 1
fi
echo "PASS: aboutPage.js exists and is properly structured"

# Test 4: Check if prefs.js imports aboutPage
echo "Test 4: Checking prefs.js imports..."
PREFS_FILE="$EXT_DIR/prefs.js"
if ! grep -q "aboutPage.js" "$PREFS_FILE"; then
    echo "FAIL: prefs.js does not import aboutPage.js"
    exit 1
fi
if ! grep -q "AboutPage" "$PREFS_FILE"; then
    echo "FAIL: prefs.js does not use AboutPage"
    exit 1
fi
echo "PASS: prefs.js properly imports and uses AboutPage"

# Test 5: Validate Makefile dependencies
echo "Test 5: Checking Makefile dependencies..."
if ! grep -q "build:.*version-info" "$SCRIPT_DIR/../Makefile"; then
    echo "FAIL: build target missing version-info dependency"
    exit 1
fi

if ! grep -q "version-info.json" "$SCRIPT_DIR/../Makefile"; then
    echo "FAIL: version-info.json not included in build"
    exit 1
fi
echo "PASS: Makefile dependencies are correct"

# Test 6: Test basic extension file structure
echo "Test 6: Checking extension file structure..."

# Check extension files
if [ ! -f "$EXT_DIR/extension.js" ]; then
    echo "FAIL: extension.js not found"
    exit 1
fi

if [ ! -f "$EXT_DIR/metadata.json" ]; then
    echo "FAIL: metadata.json not found"
    exit 1
fi

if [ ! -f "$EXT_DIR/prefs.js" ]; then
    echo "FAIL: prefs.js not found"
    exit 1
fi

if [ ! -f "$EXT_DIR/preferences/aboutPage.js" ]; then
    echo "FAIL: preferences/aboutPage.js not found"
    exit 1
fi

if [ ! -f "$SCRIPT_DIR/../update-version-info.sh" ]; then
    echo "FAIL: update-version-info.sh not found"
    exit 1
fi

echo "PASS: All required files exist"

echo ""
echo "All tests passed! ✅"
echo ""
echo "Summary:"
echo "- Update script exists and is executable"
echo "- Makefile has proper version-info target and dependencies"
echo "- About page is properly implemented"
echo "- Preferences integration is correct"
echo "- All required files are present"
echo "- Extension structure is valid"