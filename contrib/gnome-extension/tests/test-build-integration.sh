#!/bin/bash

# Integration test for full build and packaging workflow
# This test actually builds and packages the extension

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXT_DIR="$SCRIPT_DIR/../keylightd-control@jmylchreest.github.io"

echo "Running Build Integration Tests..."
echo "================================="

# Test 1: Clean build from scratch
echo "Test 1: Testing clean build..."
cd "$SCRIPT_DIR/.."
make clean >/dev/null 2>&1
echo "✓ Clean completed"

# Test 2: Generate version info
echo "Test 2: Testing version info generation..."
make version-info >/dev/null 2>&1

VERSION_FILE="$EXT_DIR/version-info.json"
if [ ! -f "$VERSION_FILE" ]; then
    echo "❌ FAIL: version-info.json not generated"
    exit 1
fi
echo "✓ Version info generated"

# Test 3: Validate JSON structure
echo "Test 3: Validating version info JSON..."
if command -v python3 >/dev/null 2>&1; then
    python3 -c "
import json
import sys

try:
    with open('$VERSION_FILE', 'r') as f:
        data = json.load(f)
    
    required_fields = ['project_name', 'about', 'version', 'commit']
    for field in required_fields:
        if field not in data:
            print(f'❌ FAIL: Missing field {field}')
            sys.exit(1)
        if not isinstance(data[field], str):
            print(f'❌ FAIL: Field {field} is not a string')
            sys.exit(1)
    
    print('✓ JSON structure is valid')
except Exception as e:
    print(f'❌ FAIL: JSON validation error: {e}')
    sys.exit(1)
"
else
    echo "⚠ SKIP: python3 not available for JSON validation"
fi

# Test 4: Test schema compilation
echo "Test 4: Testing schema compilation..."
make build >/dev/null 2>&1

if [ ! -f "$EXT_DIR/schemas/gschemas.compiled" ]; then
    echo "❌ FAIL: schemas not compiled"
    exit 1
fi
echo "✓ Schemas compiled successfully"

# Test 5: Test packaging
echo "Test 5: Testing extension packaging..."
make zip >/dev/null 2>&1

ZIP_FILE="$SCRIPT_DIR/../../../dist/gnome-extension/keylightd-control@jmylchreest.github.io.shell-extension.zip"
if [ ! -f "$ZIP_FILE" ]; then
    echo "❌ FAIL: Extension zip not created"
    exit 1
fi
echo "✓ Extension packaged successfully"

# Test 6: Validate zip contents
echo "Test 6: Validating zip contents..."
REQUIRED_IN_ZIP=(
    "version-info.json"
    "preferences/aboutPage.js"
    "extension.js"
    "metadata.json"
    "prefs.js"
    "schemas/gschemas.compiled"
)

for file in "${REQUIRED_IN_ZIP[@]}"; do
    if ! unzip -l "$ZIP_FILE" | grep -q "$file"; then
        echo "❌ FAIL: $file not found in extension zip"
        exit 1
    fi
done
echo "✓ All required files present in zip"

# Test 7: Test with environment variables (simulating CI)
echo "Test 7: Testing with CI environment variables..."
cd "$SCRIPT_DIR/.."
make clean >/dev/null 2>&1

GITHUB_REF="refs/tags/v1.0.0-test" GITHUB_SHA="abc123def456" make version-info >/dev/null 2>&1

if ! grep -q "1.0.0-test" "$VERSION_FILE"; then
    echo "❌ FAIL: Environment variables not picked up"
    exit 1
fi
echo "✓ Environment variables processed correctly"

echo ""
echo "🎉 All integration tests passed!"
echo ""
echo "Build Integration Summary:"
echo "- Clean build works correctly"
echo "- Version info generation functions properly"
echo "- JSON structure is valid"
echo "- Schema compilation succeeds"
echo "- Extension packaging works"
echo "- All required files included in package"
echo "- CI environment variable handling works"