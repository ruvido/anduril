#!/bin/bash

echo "=== Anduril Test Suite ==="
echo

# Build anduril first
echo "Building anduril..."
go build -o anduril
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi
echo "✅ Build successful"
echo

# Test 1: Quality comparison
echo "=== Test 1: Quality Comparison ==="
echo "Testing quality check with same image at different qualities..."
./anduril import test/quality --dry-run --user testuser --library test-output
echo

# Test 2: Messaging app patterns  
echo "=== Test 2: Messaging App Patterns ==="
echo "Testing filename pattern recognition for Signal, WhatsApp, Telegram..."
./anduril import test/messages --dry-run --user testuser --library test-output
echo

# Test 3: Mixed files (should skip non-media)
echo "=== Test 3: Mixed Files (Documents + Media) ==="
echo "Testing that documents are ignored and only media files processed..."
./anduril import test/mixed --dry-run --user testuser --library test-output
echo

# Test 4: Real import (non-dry run) to test actual behavior
echo "=== Test 4: Real Import Test ==="
echo "Creating temporary test library and importing quality test files..."
mkdir -p test-library
./anduril import test/quality --user testuser --library test-library
echo
echo "Checking what was imported:"
find test-library -type f | sort
echo

# Test 5: Import again to test duplicate/quality logic
echo "=== Test 5: Duplicate Import Test ==="
echo "Re-importing same files to test quality replacement logic..."
./anduril import test/quality --user testuser --library test-library
echo

echo "=== Test Suite Complete ==="
echo "Check test-library/ to see organized files"
echo "Use 'rm -rf test-library' to clean up"