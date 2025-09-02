#!/bin/bash

echo "=== Quality Comparison Real Test ==="

# Clean up
rm -rf test-library

# Step 1: Import low quality version first
echo "Step 1: Importing low quality version..."
mkdir -p test-quality-step1
cp test/quality/test_photo_low.jpg test-quality-step1/same_photo.jpg
./anduril import test-quality-step1 --user testuser --library test-library
echo "Imported: $(find test-library -name "same_photo.jpg" -exec ls -la {} \;)"
echo

# Step 2: Try to import high quality version with same name
echo "Step 2: Importing high quality version (should replace)..."
mkdir -p test-quality-step2  
cp test/quality/test_photo_high.jpg test-quality-step2/same_photo.jpg
./anduril import test-quality-step2 --user testuser --library test-library
echo "After replace: $(find test-library -name "same_photo*" -exec ls -la {} \;)"
echo

# Step 3: Try to import medium quality (should be skipped)
echo "Step 3: Importing medium quality (should be skipped)..."
mkdir -p test-quality-step3
cp test/quality/test_photo_medium.jpg test-quality-step3/same_photo.jpg  
./anduril import test-quality-step3 --user testuser --library test-library
echo "After medium: $(find test-library -name "same_photo*" -exec ls -la {} \;)"
echo

# Step 4: Try to import different resolution (should copy with suffix)
echo "Step 4: Importing different resolution (should copy with suffix)..."
mkdir -p test-quality-step4
cp test/quality/test_photo_320x240.jpg test-quality-step4/same_photo.jpg
./anduril import test-quality-step4 --user testuser --library test-library
echo "After different res: $(find test-library -name "same_photo*" -exec ls -la {} \;)"

echo
echo "=== Test Complete ==="
echo "Check file sizes to verify quality comparison worked correctly"

# Cleanup temp dirs
rm -rf test-quality-step1 test-quality-step2 test-quality-step3 test-quality-step4