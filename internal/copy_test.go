package internal

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// createTestImage creates a test image with specified dimensions and quality
func createTestImage(width, height int, quality int) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Fill with a simple pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.RGBA{
				R: uint8((x * 255) / width),
				G: uint8((y * 255) / height),
				B: uint8((x + y) % 255),
				A: 255,
			}
			img.Set(x, y, c)
		}
	}
	
	return img, nil
}

// saveTestImage saves an image to a temporary file with specified JPEG quality
func saveTestImage(img image.Image, path string, quality int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	options := &jpeg.Options{Quality: quality}
	return jpeg.Encode(file, img, options)
}

func TestGetImageResolution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anduril_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Test cases: [width, height]
	testCases := []struct {
		width, height int
	}{
		{100, 100},
		{200, 150},
		{1920, 1080},
		{50, 200},
	}
	
	for _, tc := range testCases {
		img, err := createTestImage(tc.width, tc.height, 90)
		if err != nil {
			t.Fatal(err)
		}
		
		path := filepath.Join(tempDir, "test.jpg")
		err = saveTestImage(img, path, 90)
		if err != nil {
			t.Fatal(err)
		}
		
		w, h, err := getImageResolution(path)
		if err != nil {
			t.Errorf("getImageResolution failed: %v", err)
			continue
		}
		
		if w != tc.width || h != tc.height {
			t.Errorf("Expected resolution %dx%d, got %dx%d", tc.width, tc.height, w, h)
		}
		
		os.Remove(path)
	}
}

func TestCompareImageQuality(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anduril_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create test images
	smallImg, _ := createTestImage(100, 100, 90)
	largeImg, _ := createTestImage(200, 200, 90)
	sameImg, _ := createTestImage(100, 100, 90)
	
	smallPath := filepath.Join(tempDir, "small.jpg")
	largePath := filepath.Join(tempDir, "large.jpg")
	samePath := filepath.Join(tempDir, "same.jpg")
	sameHQPath := filepath.Join(tempDir, "same_hq.jpg")
	sameLQPath := filepath.Join(tempDir, "same_lq.jpg")
	
	// Save images with different qualities
	saveTestImage(smallImg, smallPath, 90)
	saveTestImage(largeImg, largePath, 90)
	saveTestImage(sameImg, samePath, 90)
	saveTestImage(sameImg, sameHQPath, 95) // Higher quality (larger file)
	saveTestImage(sameImg, sameLQPath, 50) // Lower quality (smaller file)
	
	testCases := []struct {
		name     string
		newPath  string
		existingPath string
		expected QualityResult
	}{
		{"Large vs Small", largePath, smallPath, HIGHER},
		{"Small vs Large", smallPath, largePath, LOWER},
		{"Same resolution, HQ vs Normal", sameHQPath, samePath, HIGHER},
		{"Same resolution, Normal vs HQ", samePath, sameHQPath, LOWER},
		{"Same resolution, Normal vs LQ", samePath, sameLQPath, HIGHER},
		{"Same resolution, LQ vs Normal", sameLQPath, samePath, LOWER},
		{"Identical images", samePath, samePath, EQUAL},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := compareImageQuality(tc.newPath, tc.existingPath)
			
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestParseDateFromFilename(t *testing.T) {
	testCases := []struct {
		filename string
		expected string // Format: "2006-01-02 15:04:05"
		shouldFail bool
	}{
		// Generic patterns
		{"IMG_20240315_143022.jpg", "2024-03-15 14:30:22", false},
		{"2024-03-15-14-30-22.jpg", "2024-03-15 14:30:22", false},
		{"20240315_143022.jpg", "2024-03-15 14:30:22", false},
		{"2024-03-15.jpg", "2024-03-15 12:00:00", false},
		{"20240315.jpg", "2024-03-15 12:00:00", false},
		
		// App-specific patterns
		{"signal_20240315_143022.jpg", "2024-03-15 14:30:22", false},
		{"SIGNAL_20240315_143022.JPG", "2024-03-15 14:30:22", false}, // Case insensitive
		{"IMG-20240315-WA0001.jpg", "2024-03-15 12:00:00", false}, // WhatsApp
		{"VID-20240315-WA0001.mp4", "2024-03-15 12:00:00", false}, // WhatsApp video
		{"telegram_2024-03-15_14-30-22.mp4", "2024-03-15 14:30:22", false},
		{"telegram_2024-03-15.jpg", "2024-03-15 12:00:00", false},
		{"InShot_20240315_143022.mp4", "2024-03-15 14:30:22", false},
		{"instagram_20240315_143022.jpg", "2024-03-15 14:30:22", false},
		
		// Invalid cases
		{"random_filename.jpg", "", true},
		{"IMG_99999999_999999.jpg", "", true}, // Invalid date
		{"signal_2024_99_99.jpg", "", true}, // Invalid month/day
	}
	
	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result, err := parseDateFromFilename(tc.filename)
			
			if tc.shouldFail {
				if err == nil {
					t.Errorf("Expected parsing to fail for %s, but got: %s", tc.filename, result.Format("2006-01-02 15:04:05"))
				}
				return
			}
			
			if err != nil {
				t.Errorf("Parsing failed for %s: %v", tc.filename, err)
				return
			}
			
			actual := result.Format("2006-01-02 15:04:05")
			if actual != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, actual)
			}
		})
	}
}

func TestGetFileSize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anduril_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a test image
	img, _ := createTestImage(100, 100, 90)
	path := filepath.Join(tempDir, "test.jpg")
	saveTestImage(img, path, 90)
	
	size, err := getFileSize(path)
	if err != nil {
		t.Errorf("getFileSize failed: %v", err)
	}
	
	if size <= 0 {
		t.Errorf("Expected positive file size, got %d", size)
	}
	
	// Verify against os.Stat
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	
	if size != info.Size() {
		t.Errorf("getFileSize returned %d, os.Stat returned %d", size, info.Size())
	}
}

func TestCompareVideoQuality(t *testing.T) {
	// Test basic functionality - compareVideoQuality will return UNKNOWN
	// for non-video files, which is expected behavior
	tempDir, err := os.MkdirTemp("", "anduril_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create dummy files for basic testing
	img, _ := createTestImage(100, 100, 90)
	path1 := filepath.Join(tempDir, "test1.jpg")
	path2 := filepath.Join(tempDir, "test2.jpg")
	saveTestImage(img, path1, 90)
	saveTestImage(img, path2, 90)
	
	// Should return UNKNOWN for non-video files (images)
	result := compareVideoQuality(path1, path2)
	if result != UNKNOWN {
		t.Errorf("Expected UNKNOWN for non-video files, got %v", result)
	}
	
	// Test with non-existent files
	result = compareVideoQuality("nonexistent1.mp4", "nonexistent2.mp4")
	if result != UNKNOWN {
		t.Errorf("Expected UNKNOWN for non-existent files, got %v", result)
	}
}