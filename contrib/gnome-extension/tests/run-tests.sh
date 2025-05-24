#!/bin/bash

# Test runner script for GNOME Shell extension
# Runs all tests and reports results

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS_PASSED=0
TESTS_FAILED=0

echo "Running GNOME Shell Extension Tests..."
echo "=================================="

# Function to run a test script
run_test() {
    local test_name="$1"
    local test_script="$2"
    
    echo ""
    echo "Running: $test_name"
    echo "---"
    
    if [ ! -f "$SCRIPT_DIR/$test_script" ]; then
        echo "❌ FAIL: Test script $test_script not found"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
    
    if [ ! -x "$SCRIPT_DIR/$test_script" ]; then
        echo "❌ FAIL: Test script $test_script is not executable"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
    
    if "$SCRIPT_DIR/$test_script"; then
        echo "✅ PASS: $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo "❌ FAIL: $test_name"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Check command line arguments for test type
TEST_TYPE="${1:-unit}"

case "$TEST_TYPE" in
    "unit")
        echo "Running unit tests..."
        run_test "Import Checks" "check-imports.sh"
        run_test "Version Info Tests" "test-version-info.sh"
        ;;
    "integration")
        echo "Running integration tests..."
        run_test "Build Integration Tests" "test-build-integration.sh"
        ;;
    "all")
        echo "Running all tests..."
        run_test "Import Checks" "check-imports.sh"
        run_test "Version Info Tests" "test-version-info.sh"
        run_test "Build Integration Tests" "test-build-integration.sh"
        ;;
    *)
        echo "Usage: $0 [unit|integration|all]"
        echo "  unit        - Run fast validation tests (default)"
        echo "  integration - Run full build and packaging tests"
        echo "  all         - Run all tests"
        exit 1
        ;;
esac

# Summary
echo ""
echo "Test Results Summary"
echo "==================="
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"
echo "Total:  $((TESTS_PASSED + TESTS_FAILED))"

if [ $TESTS_FAILED -eq 0 ]; then
    echo ""
    echo "🎉 All tests passed!"
    exit 0
else
    echo ""
    echo "💥 Some tests failed!"
    exit 1
fi