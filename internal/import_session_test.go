package internal

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewImportSession(t *testing.T) {
	tempDir := t.TempDir()

	session, err := NewImportSession(tempDir, "testuser", "/input/test")
	if err != nil {
		t.Fatalf("NewImportSession failed: %v", err)
	}
	defer session.Close()

	// Verify session directory created
	if _, err := os.Stat(session.SessionDir); os.IsNotExist(err) {
		t.Errorf("Session directory not created: %s", session.SessionDir)
	}

	// Verify manifest file created
	manifestPath := filepath.Join(session.SessionDir, "manifest.jsonl")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Errorf("Manifest file not created: %s", manifestPath)
	}

	// Verify session fields
	if session.User != "testuser" {
		t.Errorf("Expected user 'testuser', got '%s'", session.User)
	}

	if session.InputDir != "/input/test" {
		t.Errorf("Expected inputDir '/input/test', got '%s'", session.InputDir)
	}
}

func TestImportSession_CreateHardlink_NoCollision(t *testing.T) {
	tempDir := t.TempDir()

	session, err := NewImportSession(tempDir, "testuser", "/input")
	if err != nil {
		t.Fatalf("NewImportSession failed: %v", err)
	}
	defer session.Close()

	// Create a test file to link
	testFile := filepath.Join(tempDir, "test.jpg")
	if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create hardlink
	browseName, err := session.CreateHardlink(testFile)
	if err != nil {
		t.Fatalf("CreateHardlink failed: %v", err)
	}

	// Verify basename is unchanged
	if browseName != "test.jpg" {
		t.Errorf("Expected basename 'test.jpg', got '%s'", browseName)
	}

	// Verify hardlink exists
	browsePath := filepath.Join(session.SessionDir, browseName)
	if _, err := os.Stat(browsePath); os.IsNotExist(err) {
		t.Errorf("Hardlink not created: %s", browsePath)
	}

	// Verify it's actually a hardlink (same inode)
	srcInfo, _ := os.Stat(testFile)
	destInfo, _ := os.Stat(browsePath)
	if !os.SameFile(srcInfo, destInfo) {
		t.Errorf("Not a hardlink - different inodes")
	}
}

func TestImportSession_CreateHardlink_WithCollision(t *testing.T) {
	tempDir := t.TempDir()

	session, err := NewImportSession(tempDir, "testuser", "/input")
	if err != nil {
		t.Fatalf("NewImportSession failed: %v", err)
	}
	defer session.Close()

	// Create multiple test files with same basename
	testFile1 := filepath.Join(tempDir, "lib1", "test.jpg")
	testFile2 := filepath.Join(tempDir, "lib2", "test.jpg")
	testFile3 := filepath.Join(tempDir, "lib3", "test.jpg")

	os.MkdirAll(filepath.Dir(testFile1), 0755)
	os.MkdirAll(filepath.Dir(testFile2), 0755)
	os.MkdirAll(filepath.Dir(testFile3), 0755)

	os.WriteFile(testFile1, []byte("data1"), 0644)
	os.WriteFile(testFile2, []byte("data2"), 0644)
	os.WriteFile(testFile3, []byte("data3"), 0644)

	// Create hardlinks - should handle collisions
	browse1, err := session.CreateHardlink(testFile1)
	if err != nil {
		t.Fatalf("CreateHardlink 1 failed: %v", err)
	}

	browse2, err := session.CreateHardlink(testFile2)
	if err != nil {
		t.Fatalf("CreateHardlink 2 failed: %v", err)
	}

	browse3, err := session.CreateHardlink(testFile3)
	if err != nil {
		t.Fatalf("CreateHardlink 3 failed: %v", err)
	}

	// Verify naming
	if browse1 != "test.jpg" {
		t.Errorf("Expected 'test.jpg', got '%s'", browse1)
	}
	if browse2 != "test_2.jpg" {
		t.Errorf("Expected 'test_2.jpg', got '%s'", browse2)
	}
	if browse3 != "test_3.jpg" {
		t.Errorf("Expected 'test_3.jpg', got '%s'", browse3)
	}

	// Verify all hardlinks exist
	for _, name := range []string{browse1, browse2, browse3} {
		path := filepath.Join(session.SessionDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Hardlink not created: %s", path)
		}
	}
}

func TestImportSession_ManifestJSONL(t *testing.T) {
	tempDir := t.TempDir()

	session, err := NewImportSession(tempDir, "testuser", "/input/photos")
	if err != nil {
		t.Fatalf("NewImportSession failed: %v", err)
	}
	defer session.Close()

	// Log various events
	if err := session.LogSessionStart(100); err != nil {
		t.Fatalf("LogSessionStart failed: %v", err)
	}

	if err := session.LogCopied("/input/img1.jpg", "user/2024/01/01/img1.jpg", "hash123", 1024, "img1.jpg"); err != nil {
		t.Fatalf("LogCopied failed: %v", err)
	}

	if err := session.LogSkippedDuplicate("/input/img2.jpg", "user/2024/01/02/img2.jpg", "hash456"); err != nil {
		t.Fatalf("LogSkippedDuplicate failed: %v", err)
	}

	stats := ImportStats{
		TotalScanned:     100,
		Copied:           80,
		SkippedDuplicate: 15,
		Errors:           5,
	}
	if err := session.LogSessionEnd(stats); err != nil {
		t.Fatalf("LogSessionEnd failed: %v", err)
	}

	// Close to flush
	session.Close()

	// Read and verify manifest
	manifestPath := filepath.Join(session.SessionDir, "manifest.jsonl")
	file, err := os.Open(manifestPath)
	if err != nil {
		t.Fatalf("Failed to open manifest: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	eventTypes := []string{}

	for scanner.Scan() {
		lineCount++
		var event ManifestEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Errorf("Failed to parse JSON line %d: %v", lineCount, err)
			continue
		}
		eventTypes = append(eventTypes, event.Event)
	}

	// Verify we got all events
	expectedEvents := []string{"session_start", "copied", "skipped_duplicate", "session_end"}
	if lineCount != len(expectedEvents) {
		t.Errorf("Expected %d events, got %d", len(expectedEvents), lineCount)
	}

	for i, expected := range expectedEvents {
		if i >= len(eventTypes) || eventTypes[i] != expected {
			t.Errorf("Event %d: expected '%s', got '%s'", i, expected, eventTypes[i])
		}
	}
}

func TestImportSession_GetStats(t *testing.T) {
	tempDir := t.TempDir()

	session, err := NewImportSession(tempDir, "testuser", "/input")
	if err != nil {
		t.Fatalf("NewImportSession failed: %v", err)
	}
	defer session.Close()

	// Log some events
	session.LogCopied("/a", "b", "hash1", 100, "a.jpg")
	session.LogCopied("/c", "d", "hash2", 200, "c.jpg")
	session.LogSkippedDuplicate("/e", "f", "hash3")
	session.LogError("/g", os.ErrNotExist)

	stats := session.GetStats()

	if stats.Copied != 2 {
		t.Errorf("Expected 2 copied, got %d", stats.Copied)
	}
	if stats.SkippedDuplicate != 1 {
		t.Errorf("Expected 1 skipped, got %d", stats.SkippedDuplicate)
	}
	if stats.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", stats.Errors)
	}
}

func TestImportSession_CollisionWithExtensions(t *testing.T) {
	tempDir := t.TempDir()

	session, err := NewImportSession(tempDir, "testuser", "/input")
	if err != nil {
		t.Fatalf("NewImportSession failed: %v", err)
	}
	defer session.Close()

	// Create files with different extensions
	testJPG := filepath.Join(tempDir, "photo.jpg")
	testPNG := filepath.Join(tempDir, "photo.png")
	testJPG2 := filepath.Join(tempDir, "another", "photo.jpg")

	os.WriteFile(testJPG, []byte("jpg"), 0644)
	os.WriteFile(testPNG, []byte("png"), 0644)
	os.MkdirAll(filepath.Dir(testJPG2), 0755)
	os.WriteFile(testJPG2, []byte("jpg2"), 0644)

	// Create hardlinks
	b1, _ := session.CreateHardlink(testJPG)
	b2, _ := session.CreateHardlink(testPNG)
	b3, _ := session.CreateHardlink(testJPG2)

	// Different extensions = different basenames = no collision
	if b1 != "photo.jpg" {
		t.Errorf("Expected 'photo.jpg', got '%s'", b1)
	}
	if b2 != "photo.png" {
		t.Errorf("Expected 'photo.png', got '%s'", b2)
	}

	// Same extension = collision
	if b3 != "photo_2.jpg" {
		t.Errorf("Expected 'photo_2.jpg', got '%s'", b3)
	}
}
