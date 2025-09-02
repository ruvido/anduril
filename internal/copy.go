package internal

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	exif "github.com/rwcarlsen/goexif/exif"
	exiftool "github.com/barasher/go-exiftool"
)

// Global errors
var (
	ErrNoExifDate = errors.New("no EXIF or media creation date found")
)

// DateConfidence represents how reliable a date detection is
type DateConfidence int

const (
	HIGH       DateConfidence = iota // EXIF metadata
	MEDIUM                           // Filename parsing
	LOW                              // File creation time
	VERY_LOW                         // File modification time
)

// QualityResult represents the result of quality comparison
type QualityResult int

const (
	HIGHER  QualityResult = iota // New file is higher quality
	LOWER                        // New file is lower quality  
	EQUAL                        // Files have equal quality
	UNKNOWN                      // Cannot determine quality
)

// Image extensions supported by goexif
var nativeImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".tiff": true,
	".tif":  true,
	".cr2":  true,
	".nef":  true,
}

// Common filename patterns ordered by frequency (most common first)
var filenamePatterns = []*regexp.Regexp{
	// Most common generic patterns first
	regexp.MustCompile(`(\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`), // 20240315_143022
	regexp.MustCompile(`IMG[_-](\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`), // IMG_20240315_143022
	regexp.MustCompile(`(\d{4})[_-](\d{2})[_-](\d{2})[_-](\d{2})[_-](\d{2})[_-](\d{2})`), // 2024-03-15-14-30-22
	regexp.MustCompile(`(\d{4})[_-](\d{2})[_-](\d{2})`), // 2024-03-15
	regexp.MustCompile(`(\d{8})`), // 20240315
	
	// App-specific patterns (case-insensitive)
	regexp.MustCompile(`(?i)(IMG|VID)[_-](\d{4})(\d{2})(\d{2})[_-]WA\d+`), // WhatsApp: IMG-20240315-WA0001
	regexp.MustCompile(`(?i)signal[_-](\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`), // Signal
	regexp.MustCompile(`(?i)inshot[_-](\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`), // InShot
	regexp.MustCompile(`(?i)telegram[_-](\d{4})[_-](\d{2})[_-](\d{2})[_-](\d{2})[_-](\d{2})[_-](\d{2})`), // Telegram datetime
	regexp.MustCompile(`(?i)telegram[_-](\d{4})[_-](\d{2})[_-](\d{2})`), // Telegram date only
}

// fileHash computes SHA256 hash of a file content
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// safeCopyPath generates a safe new path if dest exists by appending _2, _3...
func safeCopyPath(dest string) string {
	ext := filepath.Ext(dest)
	base := dest[:len(dest)-len(ext)]
	for i := 2; ; i++ {
		try := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(try); os.IsNotExist(err) {
			return try
		}
	}
}

// copyFileAtomic copies a file atomically (copy temp → rename)
func copyFileAtomic(src, dest string) error {
	tmp := dest + ".tmp"
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, dest)
}

// getFileModTime returns a file's modification time
func getFileModTime(path string) (time.Time, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return fileInfo.ModTime(), nil
}

// parseDateFromFilename tries to extract date from filename using common patterns
func parseDateFromFilename(filename string) (time.Time, error) {
	base := filepath.Base(filename)
	
	for _, pattern := range filenamePatterns {
		matches := pattern.FindStringSubmatch(base)
		if matches == nil {
			continue
		}
		
		var year, month, day, hour, minute, second int
		var err error
		
		// Handle different capture group patterns
		groups := len(matches) - 1 // -1 because first match is full string
		
		// Find year position (skip app prefix groups like "signal", "IMG", "VID")
		yearIdx := 1
		for i := 1; i < len(matches); i++ {
			if len(matches[i]) == 4 && matches[i][0] == '2' { // Find 4-digit year starting with 2
				yearIdx = i
				break
			}
		}
		
		// Extract date components starting from year position
		if yearIdx+2 < len(matches) { // Need at least year, month, day
			if year, err = strconv.Atoi(matches[yearIdx]); err != nil { continue }
			if month, err = strconv.Atoi(matches[yearIdx+1]); err != nil { continue }
			if day, err = strconv.Atoi(matches[yearIdx+2]); err != nil { continue }
			
			// Time components (optional)
			if yearIdx+5 < len(matches) { // Full datetime available
				if hour, err = strconv.Atoi(matches[yearIdx+3]); err != nil { hour = 12 }
				if minute, err = strconv.Atoi(matches[yearIdx+4]); err != nil { minute = 0 }
				if second, err = strconv.Atoi(matches[yearIdx+5]); err != nil { second = 0 }
			} else {
				hour, minute, second = 12, 0, 0 // Default to noon
			}
		} else if groups == 1 && len(matches[1]) == 8 {
			// YYYYMMDD format
			dateStr := matches[1]
			if year, err = strconv.Atoi(dateStr[0:4]); err != nil { continue }
			if month, err = strconv.Atoi(dateStr[4:6]); err != nil { continue }
			if day, err = strconv.Atoi(dateStr[6:8]); err != nil { continue }
			hour, minute, second = 12, 0, 0
		} else {
			continue
		}
		
		// Validate date ranges
		if year < 1990 || year > 2050 || month < 1 || month > 12 || day < 1 || day > 31 {
			continue
		}
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
			continue
		}
		
		return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC), nil
	}
	
	return time.Time{}, fmt.Errorf("no date pattern found in filename")
}

// getBestFileDate tries multiple methods to get the most accurate file date
func getBestFileDate(filePath string, cfg *Config) (time.Time, DateConfidence, error) {
	fileType := determineFileType(filePath, cfg)
	
	// Method 1: Try EXIF/metadata (HIGH confidence)
	if fileType == TypeImage || fileType == TypeVideo {
		captureTime, err := GetCaptureTimestamp(filePath, cfg.UseExifTool)
		if err == nil {
			return captureTime, HIGH, nil
		}
	}
	
	// Method 2: Parse filename (MEDIUM confidence)
	if fileDate, err := parseDateFromFilename(filePath); err == nil {
		return fileDate, MEDIUM, nil
	}
	
	// Method 3: File modification time (LOW confidence)
	if modTime, err := getFileModTime(filePath); err == nil {
		return modTime, LOW, nil
	}
	
	return time.Time{}, VERY_LOW, fmt.Errorf("could not determine file date for %s", filePath)
}

// getImageResolution returns the width and height of an image file
func getImageResolution(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return config.Width, config.Height, nil
}

// getFileSize returns the size of a file in bytes
func getFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// compareImageQuality compares quality between two images
func compareImageQuality(newPath, existingPath string) QualityResult {
	w1, h1, err := getImageResolution(newPath)
	if err != nil {
		return UNKNOWN
	}

	w2, h2, err := getImageResolution(existingPath)
	if err != nil {
		return UNKNOWN
	}

	pixels1 := w1 * h1
	pixels2 := w2 * h2

	// Compare resolution first (most important factor)
	if pixels1 > pixels2 {
		return HIGHER
	}
	if pixels2 > pixels1 {
		return LOWER
	}

	// Same resolution, compare file sizes (compression quality)
	size1, err := getFileSize(newPath)
	if err != nil {
		return UNKNOWN
	}

	size2, err := getFileSize(existingPath)
	if err != nil {
		return UNKNOWN
	}

	// Direct comparison without artificial thresholds
	if size1 > size2 {
		return HIGHER
	}
	if size2 > size1 {
		return LOWER
	}
	
	return EQUAL
}

// Global ExifTool instance for reuse
var globalExifTool *exiftool.Exiftool

// getOrCreateExifTool returns a reusable ExifTool instance
func getOrCreateExifTool() (*exiftool.Exiftool, error) {
	if globalExifTool == nil {
		et, err := exiftool.NewExiftool()
		if err != nil {
			return nil, fmt.Errorf("exiftool not available: %w", err)
		}
		globalExifTool = et
	}
	return globalExifTool, nil
}

// CloseExifTool closes the global ExifTool instance
func CloseExifTool() {
	if globalExifTool != nil {
		globalExifTool.Close()
		globalExifTool = nil
	}
}

// getVideoMetadata extracts basic video metadata using exiftool
func getVideoMetadata(path string) (width, height int, duration float64, err error) {
	// Quick check if file is actually a video by extension
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := map[string]bool{
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
		".webm": true, ".flv": true, ".wmv": true, ".m4v": true,
	}
	if !videoExts[ext] {
		return 0, 0, 0, fmt.Errorf("not a video file: %s", path)
	}

	et, err := getOrCreateExifTool()
	if err != nil {
		return 0, 0, 0, err
	}

	fileInfos := et.ExtractMetadata(path)
	if len(fileInfos) != 1 {
		return 0, 0, 0, fmt.Errorf("unexpected file info count: %d", len(fileInfos))
	}

	fi := fileInfos[0]
	if fi.Err != nil {
		return 0, 0, 0, fmt.Errorf("metadata extraction error: %w", fi.Err)
	}

	// Extract width
	if widthStr, err := fi.GetString("ImageWidth"); err == nil && widthStr != "" {
		if w, err := strconv.Atoi(widthStr); err == nil {
			width = w
		}
	}

	// Extract height  
	if heightStr, err := fi.GetString("ImageHeight"); err == nil && heightStr != "" {
		if h, err := strconv.Atoi(heightStr); err == nil {
			height = h
		}
	}

	// Extract duration
	if durStr, err := fi.GetString("Duration"); err == nil && durStr != "" {
		// Duration might be in format "0:01:23" or "83.45"
		if d, err := strconv.ParseFloat(durStr, 64); err == nil {
			duration = d
		}
	}

	return width, height, duration, nil
}

// compareVideoQuality compares quality between two videos
func compareVideoQuality(newPath, existingPath string) QualityResult {
	w1, h1, dur1, err := getVideoMetadata(newPath)
	if err != nil {
		return UNKNOWN
	}

	w2, h2, dur2, err := getVideoMetadata(existingPath)
	if err != nil {
		return UNKNOWN
	}

	// Compare duration first - if significantly different, they're different videos
	durDiff := dur1 - dur2
	if durDiff < 0 {
		durDiff = -durDiff
	}
	if durDiff > 5.0 { // More than 5 seconds difference
		return UNKNOWN // Different videos, don't compare quality
	}

	pixels1 := w1 * h1
	pixels2 := w2 * h2

	// Compare resolution (most important factor)
	if pixels1 > pixels2 {
		return HIGHER
	}
	if pixels2 > pixels1 {
		return LOWER
	}

	// Same resolution, compare file sizes (bitrate/compression quality)
	size1, err := getFileSize(newPath)
	if err != nil {
		return UNKNOWN
	}

	size2, err := getFileSize(existingPath)
	if err != nil {
		return UNKNOWN
	}

	// Direct comparison without artificial thresholds
	if size1 > size2 {
		return HIGHER
	}
	if size2 > size1 {
		return LOWER
	}
	
	return EQUAL
}

// FileType represents the type of file being processed
type FileType int

const (
	TypeImage FileType = iota
	TypeVideo
	TypeOther
)

// determineFileType checks what type of file we're dealing with
func determineFileType(filePath string, cfg *Config) FileType {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	for _, e := range cfg.ImageExt {
		if ext == e {
			return TypeImage
		}
	}
	
	for _, e := range cfg.VideoExt {
		if ext == e {
			return TypeVideo
		}
	}
	
	return TypeOther
}

// generateDestinationPath creates the target path based on file type and date confidence
func generateDestinationPath(src string, fileDate time.Time, confidence DateConfidence, fileType FileType, cfg *Config, user string) (string, error) {
	destBase := filepath.Base(src)
	highConfidenceDate := confidence >= MEDIUM
	
	var destDir string
	switch {
	case fileType == TypeVideo && highConfidenceDate:
		destDir = filepath.Join(cfg.VideoLib, user,
			fmt.Sprintf("%04d", fileDate.Year()),
			fmt.Sprintf("%02d", fileDate.Month()),
			fmt.Sprintf("%02d", fileDate.Day()))
	
	case fileType == TypeVideo && !highConfidenceDate:
		destDir = filepath.Join(cfg.VideoLib, user, "noexif",
			fmt.Sprintf("%04d-%02d", fileDate.Year(), fileDate.Month()))
	
	case fileType == TypeImage && highConfidenceDate:
		destDir = filepath.Join(cfg.Library, user,
			fmt.Sprintf("%04d", fileDate.Year()),
			fmt.Sprintf("%02d", fileDate.Month()),
			fmt.Sprintf("%02d", fileDate.Day()))
	
	case fileType == TypeImage && !highConfidenceDate:
		destDir = filepath.Join(cfg.Library, user, "noexif",
			fmt.Sprintf("%04d-%02d", fileDate.Year(), fileDate.Month()))
	
	default:
		return "", fmt.Errorf("non-media file passed to generateDestinationPath: %s", src)
	}
	
	return filepath.Join(destDir, destBase), nil
}

// handleDuplicateFile manages duplicate file resolution with quality comparison
func handleDuplicateFile(src, destPath string, fileType FileType) (finalPath string, shouldSkip bool, err error) {
	// Check if files are identical
	srcHash, err := fileHash(src)
	if err != nil {
		return "", false, fmt.Errorf("failed to hash src file %s: %w", src, err)
	}
	
	destHash, err := fileHash(destPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to hash dest file %s: %w", destPath, err)
	}
	
	// If content is identical, skip
	if srcHash == destHash {
		fmt.Printf("Skipping duplicate file (identical content): %s\n", src)
		return "", true, nil
	}
	
	// Different content, apply quality logic
	switch fileType {
	case TypeImage:
		switch compareImageQuality(src, destPath) {
		case HIGHER:
			fmt.Printf("Replacing with higher quality image: %s → %s\n", src, destPath)
			return destPath, false, nil
		case EQUAL:
			fmt.Printf("Skipping file (existing has equal quality): %s\n", src)
			return "", true, nil
		case LOWER:
			finalPath = safeCopyPath(destPath)
			fmt.Printf("Copying with new name (lower quality): %s → %s\n", src, finalPath)
			return finalPath, false, nil
		case UNKNOWN:
			finalPath = safeCopyPath(destPath)
			fmt.Printf("Copying with new name (quality unknown): %s → %s\n", src, finalPath)
			return finalPath, false, nil
		}
	
	case TypeVideo:
		switch compareVideoQuality(src, destPath) {
		case HIGHER:
			fmt.Printf("Replacing with higher quality video: %s → %s\n", src, destPath)
			return destPath, false, nil
		case EQUAL:
			fmt.Printf("Skipping video (existing has equal quality): %s\n", src)
			return "", true, nil
		case LOWER:
			finalPath = safeCopyPath(destPath)
			fmt.Printf("Copying with new name (lower quality video): %s → %s\n", src, finalPath)
			return finalPath, false, nil
		case UNKNOWN:
			finalPath = safeCopyPath(destPath)
			fmt.Printf("Copying with new name (different videos or quality unknown): %s → %s\n", src, finalPath)
			return finalPath, false, nil
		}
	
	default:
		return "", false, fmt.Errorf("unexpected file type in duplicate check: %s", src)
	}
	
	return destPath, false, nil
}

// getCaptureTimestampNative uses goexif to get date for supported image files
func getCaptureTimestampNative(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return time.Time{}, fmt.Errorf("opening file %s: %w", filePath, err)
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}, fmt.Errorf("decoding EXIF from %s: %w", filePath, err)
	}

	// Try multiple EXIF date fields
	for _, field := range []exif.FieldName{
		exif.DateTimeOriginal,
		exif.DateTimeDigitized,
		exif.DateTime,
	} {
		tag, err := x.Get(field)
		if err != nil {
			continue
		}

		timeStr, err := tag.StringVal()
		if err != nil {
			continue
		}

		// Clean and parse the timestamp
		timeStr = strings.Trim(timeStr, "\"")
		t, err := time.Parse("2006:01:02 15:04:05", timeStr)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, ErrNoExifDate
}

// getCaptureTimestampExifTool uses exiftool to get date for any media file
func getCaptureTimestampExifTool(filePath string) (time.Time, error) {
	et, err := getOrCreateExifTool()
	if err != nil {
		return time.Time{}, err
	}

	// Extract file metadata
	fileInfos := et.ExtractMetadata(filePath)
	if len(fileInfos) != 1 {
		return time.Time{}, fmt.Errorf("unexpected file info count: %d", len(fileInfos))
	}

	fi := fileInfos[0]
	if fi.Err != nil {
		return time.Time{}, fmt.Errorf("exif extraction error: %w", fi.Err)
	}

	// Tags to check in priority order
	tags := []string{
		"DateTimeOriginal",
		"CreateDate",
		"CreationDate",
		"TrackCreateDate",
		"MediaCreateDate",
	}

	// Find first valid timestamp
	for _, tag := range tags {
		val, err := fi.GetString(tag)
		if err == nil && val != "" {
			// Clean and parse the timestamp
			cleanVal := strings.Trim(val, "\"")
			
			// Try various date formats
			formats := []string{
				"2006:01:02 15:04:05",         // Most common format
				"2006:01:02 15:04:05-07:00",    // With timezone
				"2006:01:02 15:04:05.999",      // With milliseconds
				"2006-01-02 15:04:05",          // Hyphen format
				"2006-01-02 15:04:05-07:00",    // Hyphen with timezone
				"2006:01:02",                   // Date only
			}
			
			for _, format := range formats {
				t, err := time.Parse(format, cleanVal)
				if err == nil {
					return t, nil
				}
			}
		}
	}

	return time.Time{}, ErrNoExifDate
}

// GetCaptureTimestamp returns the media creation timestamp from a file
func GetCaptureTimestamp(filePath string, useExifTool bool) (time.Time, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// For videos or when exiftool is requested, use exiftool directly
	if useExifTool || !nativeImageExts[ext] {
		return getCaptureTimestampExifTool(filePath)
	}
	
	// First try native for supported images
	t, err := getCaptureTimestampNative(filePath)
	if err == nil {
		return t, nil
	}
	
	// Fallback to exiftool if native fails
	return getCaptureTimestampExifTool(filePath)
}

// ProcessFile processes media files and organizes them in the library  
func ProcessFile(src string, cfg *Config, user string, dryRun bool) error {
	// Determine file type
	fileType := determineFileType(src, cfg)
	if fileType == TypeOther {
		return nil // Skip non-media files
	}
	
	// Get best available date with confidence level
	fileDate, confidence, err := getBestFileDate(src, cfg)
	if err != nil {
		return fmt.Errorf("failed to get file date for %s: %w", src, err)
	}
	
	// Log confidence level for debugging
	if confidence <= LOW {
		fmt.Printf("Warning: low confidence date for %s (using %s)\n", src, fileDate.Format("2006-01-02"))
	}
	
	// Generate destination path
	destPath, err := generateDestinationPath(src, fileDate, confidence, fileType, cfg, user)
	if err != nil {
		return err
	}
	
	if dryRun {
		fmt.Printf("[dry-run] %s → %s (confidence: %v)\n", src, destPath, confidence)
		return nil
	}
	
	// Create destination directory
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}
	
	// Handle duplicates if file exists
	if _, err := os.Stat(destPath); err == nil {
		finalPath, shouldSkip, err := handleDuplicateFile(src, destPath, fileType)
		if err != nil {
			return err
		}
		if shouldSkip {
			return nil
		}
		destPath = finalPath
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %w", destPath, err)
	}
	
	// Perform atomic copy
	if err := copyFileAtomic(src, destPath); err != nil {
		return fmt.Errorf("failed to copy file %s to %s: %w", src, destPath, err)
	}
	
	fmt.Printf("Copied %s → %s\n", src, destPath)
	return nil
}
