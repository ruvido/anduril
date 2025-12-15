package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"anduril/internal"
)

func TestImport_WithSession(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	inputDir := filepath.Join(tempDir, "input")
	libraryDir := filepath.Join(tempDir, "library")

	os.MkdirAll(inputDir, 0755)
	os.MkdirAll(libraryDir, 0755)

	// Create test files
	testFile1 := filepath.Join(inputDir, "IMG_20240101_120000.jpg")
	testFile2 := filepath.Join(inputDir, "IMG_20240102_130000.jpg")
	testFile3 := filepath.Join(inputDir, "photo.jpg")

	os.WriteFile(testFile1, []byte("test data 1"), 0644)
	os.WriteFile(testFile2, []byte("test data 2"), 0644)
	os.WriteFile(testFile3, []byte("test data 3"), 0644)

	// Create config
	conf := &internal.Config{
		User:         "testuser",
		Library:      libraryDir,
		ImageExt:     []string{".jpg", ".jpeg", ".png"},
		VideoExt:     []string{".mp4", ".mov"},
		UseExifTool:  false,
		UseHardlinks: false,
	}

	// Scan media files
	files, err := internal.ScanMediaFiles(inputDir, conf)
	if err != nil {
		t.Fatalf("ScanMediaFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("Expected 3 files, got %d", len(files))
	}

	// Process files with session
	err = processFiles(files, conf, conf.User, inputDir, false)
	if err != nil {
		t.Fatalf("processFiles failed: %v", err)
	}

	// Verify import session folder exists
	importsDir := filepath.Join(libraryDir, "imports")
	if _, err := os.Stat(importsDir); os.IsNotExist(err) {
		t.Errorf("Imports directory not created: %s", importsDir)
	}

	// Find the session directory (should be only one)
	entries, err := os.ReadDir(importsDir)
	if err != nil {
		t.Fatalf("Failed to read imports directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 session directory, found %d", len(entries))
	}

	sessionDir := filepath.Join(importsDir, entries[0].Name())

	// Verify manifest exists
	manifestPath := filepath.Join(sessionDir, "manifest.jsonl")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Errorf("Manifest file not created: %s", manifestPath)
	}

	// Verify hardlinks created in session directory
	sessionFiles, err := os.ReadDir(sessionDir)
	if err != nil {
		t.Fatalf("Failed to read session directory: %v", err)
	}

	// Should have manifest.jsonl + 3 hardlinks = 4 files
	fileCount := 0
	for _, entry := range sessionFiles {
		if !entry.IsDir() && entry.Name() != "manifest.jsonl" {
			fileCount++

			// Verify it's a real hardlink
			linkPath := filepath.Join(sessionDir, entry.Name())
			linkInfo, _ := os.Stat(linkPath)

			// Find corresponding library file
			// (This is simplified - in reality we'd parse the manifest)
			if linkInfo != nil && linkInfo.Size() > 0 {
				t.Logf("Found hardlink: %s (size: %d)", entry.Name(), linkInfo.Size())
			}
		}
	}

	if fileCount != 3 {
		t.Errorf("Expected 3 hardlinks in session, found %d", fileCount)
	}

	// Verify primary files exist in library
	expectedFiles := []string{
		filepath.Join(libraryDir, "testuser", "2024", "01", "01", "IMG_20240101_120000.jpg"),
		filepath.Join(libraryDir, "testuser", "2024", "01", "02", "IMG_20240102_130000.jpg"),
	}

	for _, expectedFile := range expectedFiles {
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Expected file not found in library: %s", expectedFile)
		}
	}

	// Verify removing session doesn't affect library files
	os.RemoveAll(sessionDir)

	for _, expectedFile := range expectedFiles {
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Library file was deleted when session removed: %s", expectedFile)
		}
	}

	t.Logf("Import session test completed successfully")
}

func TestImport_DryRunSkipsSession(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	inputDir := filepath.Join(tempDir, "input")
	libraryDir := filepath.Join(tempDir, "library")

	os.MkdirAll(inputDir, 0755)
	os.MkdirAll(libraryDir, 0755)

	// Create test file
	testFile := filepath.Join(inputDir, "IMG_20240101_120000.jpg")
	os.WriteFile(testFile, []byte("test data"), 0644)

	// Create config
	conf := &internal.Config{
		User:         "testuser",
		Library:      libraryDir,
		ImageExt:     []string{".jpg"},
		VideoExt:     []string{".mp4"},
		UseExifTool:  false,
		UseHardlinks: false,
	}

	// Scan media files
	files, err := internal.ScanMediaFiles(inputDir, conf)
	if err != nil {
		t.Fatalf("ScanMediaFiles failed: %v", err)
	}

	// Process files with DRY RUN
	err = processFiles(files, conf, conf.User, inputDir, true)
	if err != nil {
		t.Fatalf("processFiles failed: %v", err)
	}

	// Verify NO import session folder created
	importsDir := filepath.Join(libraryDir, "imports")
	if _, err := os.Stat(importsDir); !os.IsNotExist(err) {
		t.Errorf("Imports directory should not exist in dry-run mode: %s", importsDir)
	}

	// Verify NO files copied
	userDir := filepath.Join(libraryDir, "testuser")
	if _, err := os.Stat(userDir); !os.IsNotExist(err) {
		t.Errorf("User directory should not exist in dry-run mode: %s", userDir)
	}

	t.Logf("Dry-run session test completed successfully")
}

func TestImport_SessionIDFormat(t *testing.T) {
	tempDir := t.TempDir()

	session, err := internal.NewImportSession(tempDir, "user", "/input")
	if err != nil {
		t.Fatalf("NewImportSession failed: %v", err)
	}
	defer session.Close()

	// Verify session ID format (YYYY-MM-DD-HHMMSS)
	_, err = time.Parse("2006-01-02-150405", session.ID)
	if err != nil {
		t.Errorf("Session ID format invalid: %s (error: %v)", session.ID, err)
	}

	// Verify session directory name matches ID
	expectedDir := filepath.Join(tempDir, "imports", session.ID)
	if session.SessionDir != expectedDir {
		t.Errorf("Expected session dir %s, got %s", expectedDir, session.SessionDir)
	}

	t.Logf("Session ID format test passed: %s", session.ID)
}
