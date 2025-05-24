#!/bin/bash

# Script to check for improper Gtk/Gdk imports in GNOME Shell extension
# GNOME Shell extensions should not import Gtk/Gdk except in preferences files

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXT_DIR="$SCRIPT_DIR/../keylightd-control@jmylchreest.github.io"

echo "Checking for improper Gtk/Gdk imports in GNOME Shell extension..."

# Files that should NOT import Gtk/Gdk (run in Shell process)
SHELL_FILES=(
    "extension.js"
    "utils.js"
    "icons.js"
    "icon-names.js"
    "controllers/*.js"
    "managers/*.js"
    "ui/*.js"
)

# Files that CAN import Gtk/Gdk (preferences process)
PREFS_FILES=(
    "prefs.js"
    "preferences/*.js"
)

ERRORS=0

# Check shell files for forbidden imports
echo "Checking shell process files (should not import Gtk/Gdk)..."
for pattern in "${SHELL_FILES[@]}"; do
    for file in $EXT_DIR/$pattern; do
        if [ -f "$file" ]; then
            relative_file=${file#$EXT_DIR/}
            
            # Check for Gtk imports
            if grep -q "import.*from.*gi://Gtk" "$file" 2>/dev/null; then
                echo "ERROR: $relative_file imports Gtk (forbidden in shell process)"
                ERRORS=$((ERRORS + 1))
            fi
            
            # Check for Gdk imports
            if grep -q "import.*from.*gi://Gdk" "$file" 2>/dev/null; then
                echo "ERROR: $relative_file imports Gdk (forbidden in shell process)"
                ERRORS=$((ERRORS + 1))
            fi
            
            # Success message for clean files
            if ! grep -q "import.*from.*gi://G[dt]k" "$file" 2>/dev/null; then
                echo "✓ $relative_file - clean"
            fi
        fi
    done
done

# Check preferences files (these are allowed to import Gtk/Gdk)
echo ""
echo "Checking preferences process files (Gtk/Gdk allowed)..."
for pattern in "${PREFS_FILES[@]}"; do
    for file in $EXT_DIR/$pattern; do
        if [ -f "$file" ]; then
            relative_file=${file#$EXT_DIR/}
            
            # Just report what they import
            gtk_count=0
            gdk_count=0
            
            if grep -q "import.*from.*gi://Gtk" "$file" 2>/dev/null; then
                gtk_count=$(grep -c "import.*from.*gi://Gtk" "$file" 2>/dev/null)
            fi
            
            if grep -q "import.*from.*gi://Gdk" "$file" 2>/dev/null; then
                gdk_count=$(grep -c "import.*from.*gi://Gdk" "$file" 2>/dev/null)
            fi
            
            if [ "$gtk_count" -gt 0 ] || [ "$gdk_count" -gt 0 ]; then
                echo "✓ $relative_file - imports Gtk:$gtk_count Gdk:$gdk_count (allowed)"
            else
                echo "✓ $relative_file - no Gtk/Gdk imports"
            fi
        fi
    done
done

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "✅ All imports are correct!"
    echo ""
    echo "Summary:"
    echo "- Shell process files correctly avoid Gtk/Gdk imports"
    echo "- Preferences files can use Gtk/Gdk as needed"
    echo "- Extension follows GNOME Shell best practices"
else
    echo "❌ Found $ERRORS import violations!"
    echo ""
    echo "Fix these issues:"
    echo "- Remove Gtk/Gdk imports from shell process files"
    echo "- Use St (Shell Toolkit) for UI in shell process instead"
    echo "- Only preferences files should import Gtk/Gdk"
    exit 1
fi