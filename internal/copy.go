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
	"sync"
	"time"

	exiftool "github.com/barasher/go-exiftool"
	exif "github.com/rwcarlsen/goexif/exif"
)

// Global errors
var (
	ErrNoExifDate = errors.New("no EXIF or media creation date found")
)

// DateConfidence represents how reliable a date detection is
type DateConfidence int

const (
	HIGH     DateConfidence = iota // EXIF metadata
	MEDIUM                         // Filename parsing
	LOW                            // File creation time
	VERY_LOW                       // File modification time
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
	regexp.MustCompile(`(\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`),                 // 20240315_143022
	regexp.MustCompile(`IMG[_-](\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`),          // IMG_20240315_143022
	regexp.MustCompile(`(\d{4})[_-](\d{2})[_-](\d{2})[_-](\d{2})[_-](\d{2})[_-](\d{2})`), // 2024-03-15-14-30-22
	regexp.MustCompile(`(\d{4})[_-](\d{2})[_-](\d{2})`),                                  // 2024-03-15
	regexp.MustCompile(`(\d{8})`),                                                        // 20240315

	// App-specific patterns (case-insensitive)
	regexp.MustCompile(`(?i)(IMG|VID)[_-](\d{4})(\d{2})(\d{2})[_-]WA\d+`),                                // WhatsApp: IMG-20240315-WA0001
	regexp.MustCompile(`(?i)signal[_-](\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`),                   // Signal
	regexp.MustCompile(`(?i)inshot[_-](\d{4})(\d{2})(\d{2})[_-](\d{2})(\d{2})(\d{2})`),                   // InShot
	regexp.MustCompile(`(?i)telegram[_-](\d{4})[_-](\d{2})[_-](\d{2})[_-](\d{2})[_-](\d{2})[_-](\d{2})`), // Telegram datetime
	regexp.MustCompile(`(?i)telegram[_-](\d{4})[_-](\d{2})[_-](\d{2})`),                                  // Telegram date only
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

var timeNow = time.Now

// timestampSuffixCopyPath returns a path with filename suffixed by the Unix
// timestamp when the copy/link decision is made:
//
//	/path/to/img.jpg -> /path/to/img_1742032800.jpg
func timestampSuffixCopyPath(dest string) string {
	ext := filepath.Ext(dest)
	name := strings.TrimSuffix(filepath.Base(dest), ext)

	dir := filepath.Dir(dest)
	stamp := fmt.Sprintf("%d", timeNow().UTC().Unix())

	target := filepath.Join(dir, fmt.Sprintf("%s_%s%s", name, stamp, ext))
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return target
	}

	return safeCopyPath(target)
}

// TestHardlinkSupport tests if hardlinks can be created from srcDir to destDir.
// Creates a temporary file in srcDir, tries to hardlink it to destDir, then cleans up.
// Returns nil if hardlinks work, or an error explaining why they don't.
func TestHardlinkSupport(srcDir, destDir string) error {
	// Create temp file in source directory
	tmpSrc, err := os.CreateTemp(srcDir, ".hardlink-test-*")
	if err != nil {
		return fmt.Errorf("cannot create test file in source: %w", err)
	}
	tmpSrcPath := tmpSrc.Name()
	tmpSrc.Close()
	defer os.Remove(tmpSrcPath)

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}

	// Try to create hardlink in destination
	tmpDestPath := filepath.Join(destDir, ".hardlink-test-"+filepath.Base(tmpSrcPath))
	err = os.Link(tmpSrcPath, tmpDestPath)
	if err != nil {
		fmt.Printf("\nERROR: Cannot create hardlinks\n")
		fmt.Printf("  Source:      %s\n", srcDir)
		fmt.Printf("  Destination: %s\n", destDir)
		fmt.Printf("  Reason:      %v\n\n", err)
		fmt.Printf("This usually means different filesystems or NAS limitations.\n")
		fmt.Printf("Remove --link to use regular copy with SHA256 verification.\n\n")
		return fmt.Errorf("hardlink not supported")
	}
	os.Remove(tmpDestPath)

	return nil
}

// linkFile creates a hardlink from src to dest.
// Does NOT fall back to copy - caller should handle errors appropriately.
func linkFile(src, dest string) error {
	return os.Link(src, dest)
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

	// Ensure bytes hit disk before rename
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}

	// Sync parent directory to persist metadata
	dir, err := os.Open(filepath.Dir(dest))
	if err != nil {
		return err
	}
	defer dir.Close()

	return dir.Sync()
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
			if year, err = strconv.Atoi(matches[yearIdx]); err != nil {
				continue
			}
			if month, err = strconv.Atoi(matches[yearIdx+1]); err != nil {
				continue
			}
			if day, err = strconv.Atoi(matches[yearIdx+2]); err != nil {
				continue
			}

			// Time components (optional)
			if yearIdx+5 < len(matches) { // Full datetime available
				if hour, err = strconv.Atoi(matches[yearIdx+3]); err != nil {
					hour = 12
				}
				if minute, err = strconv.Atoi(matches[yearIdx+4]); err != nil {
					minute = 0
				}
				if second, err = strconv.Atoi(matches[yearIdx+5]); err != nil {
					second = 0
				}
			} else {
				hour, minute, second = 12, 0, 0 // Default to noon
			}
		} else if groups == 1 && len(matches[1]) == 8 {
			// YYYYMMDD format
			dateStr := matches[1]
			if year, err = strconv.Atoi(dateStr[0:4]); err != nil {
				continue
			}
			if month, err = strconv.Atoi(dateStr[4:6]); err != nil {
				continue
			}
			if day, err = strconv.Atoi(dateStr[6:8]); err != nil {
				continue
			}
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
var (
	globalExifTool *exiftool.Exiftool
	exifToolMu     sync.Mutex
)

// getOrCreateExifToolLocked expects exifToolMu to be held
func getOrCreateExifToolLocked() (*exiftool.Exiftool, error) {
	if globalExifTool != nil {
		return globalExifTool, nil
	}

	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, fmt.Errorf("exiftool not available: %w", err)
	}
	globalExifTool = et
	return globalExifTool, nil
}

// CloseExifTool closes the global ExifTool instance
func CloseExifTool() {
	exifToolMu.Lock()
	defer exifToolMu.Unlock()

	if globalExifTool == nil {
		return
	}

	globalExifTool.Close()
	globalExifTool = nil
}

// extractMetadata serializes access to ExifTool for thread safety
func extractMetadata(paths ...string) ([]exiftool.FileMetadata, error) {
	exifToolMu.Lock()
	defer exifToolMu.Unlock()

	et, err := getOrCreateExifToolLocked()
	if err != nil {
		return nil, err
	}

	return et.ExtractMetadata(paths...), nil
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

	fileInfos, err := extractMetadata(path)
	if err != nil {
		return 0, 0, 0, err
	}

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
		duration, err = parseDuration(durStr)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("duration parse error: %w", err)
		}
	}

	if width == 0 || height == 0 {
		return 0, 0, 0, fmt.Errorf("missing video dimensions for %s", path)
	}

	return width, height, duration, nil
}

// parseDuration converts common ExifTool duration formats to seconds
func parseDuration(raw string) (float64, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Formats like "0:01:23" or "01:23"
	if strings.Contains(clean, ":") {
		parts := strings.Split(clean, ":")
		if len(parts) == 2 {
			// mm:ss
			min, err1 := strconv.Atoi(parts[0])
			sec, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 != nil || err2 != nil {
				return 0, fmt.Errorf("invalid mm:ss duration %q", raw)
			}
			return float64(min)*60 + sec, nil
		}
		if len(parts) == 3 {
			// hh:mm:ss
			h, err1 := strconv.Atoi(parts[0])
			min, err2 := strconv.Atoi(parts[1])
			sec, err3 := strconv.ParseFloat(parts[2], 64)
			if err1 != nil || err2 != nil || err3 != nil {
				return 0, fmt.Errorf("invalid hh:mm:ss duration %q", raw)
			}
			return float64(h)*3600 + float64(min)*60 + sec, nil
		}
		return 0, fmt.Errorf("unsupported duration format %q", raw)
	}

	// Fallback: plain seconds (possibly with decimals)
	seconds, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric duration %q", raw)
	}
	return seconds, nil
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

// Public helper functions for server mode

// DetermineFileType is a public wrapper for determineFileType
func DetermineFileType(filePath string, cfg *Config) FileType {
	return determineFileType(filePath, cfg)
}

// GetBestFileDate is a public wrapper for getBestFileDate
func GetBestFileDate(filePath string, cfg *Config) (time.Time, DateConfidence, error) {
	return getBestFileDate(filePath, cfg)
}

// FileHash is a public wrapper for fileHash
func FileHash(path string) (string, error) {
	return fileHash(path)
}

// GetFileSize is a public wrapper for getFileSize
func GetFileSize(path string) (int64, error) {
	return getFileSize(path)
}

// GetImageResolution is a public wrapper for getImageResolution
func GetImageResolution(path string) (int, int, error) {
	return getImageResolution(path)
}

// GetVideoMetadata is a public wrapper for getVideoMetadata
func GetVideoMetadata(path string) (int, int, float64, error) {
	return getVideoMetadata(path)
}

// ExtractUserFromPath extracts username from organized file path
func ExtractUserFromPath(filePath string, cfg *Config) string {
	// Try to find user directory in the path
	rel, err := filepath.Rel(cfg.Library, filePath)
	if err != nil {
		// Try video library
		rel, err = filepath.Rel(cfg.VideoLib, filePath)
		if err != nil {
			return "unknown"
		}
	}

	// Extract first directory component as user
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) > 0 && parts[0] != "" && parts[0] != "." {
		return parts[0]
	}

	return "unknown"
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
	highConfidenceDate := confidence <= MEDIUM

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

// handleDuplicateFile manages duplicate file resolution using strict hash comparison
func handleDuplicateFile(src, destPath string, fileType FileType, isSilent bool) (finalPath string, shouldSkip bool, err error) {
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
		if !isSilent {
			fmt.Printf("Skipping duplicate file (identical content): %s\n", src)
		}
		return "", true, nil
	}

	// Different content: if a timestamp-suffixed copy with the same hash already exists, skip.
	dir := filepath.Dir(destPath)
	ext := filepath.Ext(destPath)
	base := strings.TrimSuffix(filepath.Base(destPath), ext)

	pattern := filepath.Join(dir, fmt.Sprintf("%s_*%s", base, ext))
	matches, _ := filepath.Glob(pattern)
	for _, candidate := range matches {
		candidateHash, err := fileHash(candidate)
		if err != nil {
			continue
		}
		if candidateHash == srcHash {
			if !isSilent {
				fmt.Printf("Skipping duplicate file (matching timestamp copy exists): %s\n", src)
			}
			return "", true, nil
		}
	}

	// Keep both by placing the incoming file under a timestamp-suffixed name
	finalPath = timestampSuffixCopyPath(destPath)
	if !isSilent {
		fmt.Printf("Existing file has different content, saving with timestamp suffix: %s → %s\n", src, finalPath)
	}
	return finalPath, false, nil
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
	// Extract file metadata
	fileInfos, err := extractMetadata(filePath)
	if err != nil {
		return time.Time{}, err
	}
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
				"2006:01:02 15:04:05",       // Most common format
				"2006:01:02 15:04:05-07:00", // With timezone
				"2006:01:02 15:04:05.999",   // With milliseconds
				"2006-01-02 15:04:05",       // Hyphen format
				"2006-01-02 15:04:05-07:00", // Hyphen with timezone
				"2006:01:02",                // Date only
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

// BatchExtractMetadata extracts metadata for multiple files in one ExifTool call
func BatchExtractMetadata(filePaths []string) (map[string]time.Time, error) {
	if len(filePaths) == 0 {
		return make(map[string]time.Time), nil
	}

	// Extract metadata for all files at once (serialized)
	fileInfos, err := extractMetadata(filePaths...)
	if err != nil {
		return nil, err
	}
	results := make(map[string]time.Time)

	tags := []string{
		"DateTimeOriginal",
		"CreateDate",
		"CreationDate",
		"TrackCreateDate",
		"MediaCreateDate",
	}

	formats := []string{
		"2006:01:02 15:04:05",
		"2006:01:02 15:04:05-07:00",
		"2006:01:02 15:04:05.999",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05-07:00",
		"2006:01:02",
	}

	for _, fi := range fileInfos {
		if fi.Err != nil {
			continue // Skip files with extraction errors
		}

		// Find first valid timestamp
		for _, tag := range tags {
			val, err := fi.GetString(tag)
			if err == nil && val != "" {
				cleanVal := strings.Trim(val, "\"")

				for _, format := range formats {
					if t, err := time.Parse(format, cleanVal); err == nil {
						results[fi.File] = t
						goto nextFile
					}
				}
			}
		}
	nextFile:
	}

	return results, nil
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
// session parameter is optional - pass nil to skip session tracking
func ProcessFile(src string, cfg *Config, user string, dryRun bool, session *ImportSession, silent ...bool) error {
	isSilent := len(silent) > 0 && silent[0]
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
	if !isSilent && confidence >= LOW {
		fmt.Printf("Warning: low confidence date for %s (using %s)\n", src, fileDate.Format("2006-01-02"))
	}

	// Generate destination path
	destPath, err := generateDestinationPath(src, fileDate, confidence, fileType, cfg, user)
	if err != nil {
		return err
	}
	origDestPath := destPath

	if dryRun {
		if !isSilent {
			fmt.Printf("[dry-run] %s → %s (confidence: %v)\n", src, destPath, confidence)
		}
		return nil
	}

	// Create destination directory
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	// Handle duplicates if file exists
	destExists := false
	if _, err := os.Stat(destPath); err == nil {
		destExists = true
		finalPath, shouldSkip, err := handleDuplicateFile(src, destPath, fileType, isSilent)
		if err != nil {
			return err
		}
		if shouldSkip {
			// Log skip to session if tracking
			if session != nil {
				hash, _ := fileHash(src)
				session.LogSkippedDuplicate(src, destPath, hash)
			}
			return nil
		}
		destPath = finalPath
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %w", destPath, err)
	}

	// Replacement means we want the new file at the original destination name
	isUpgradeReplace := destExists && destPath == origDestPath

	// Perform file operation (hardlink or atomic copy)
	if cfg.UseHardlinks {
		if isUpgradeReplace {
			// Hardlinks cannot overwrite; fall back to atomic copy with verification
			if err := copyFileAtomic(src, destPath); err != nil {
				return fmt.Errorf("failed to replace file %s with upgraded copy: %w", destPath, err)
			}

			// Verify integrity with SHA256 comparison
			srcHash, err := fileHash(src)
			if err != nil {
				return fmt.Errorf("failed to hash source %s: %w", src, err)
			}

			destHash, err := fileHash(destPath)
			if err != nil {
				return fmt.Errorf("failed to hash destination %s: %w", destPath, err)
			}

			if srcHash != destHash {
				_ = os.Remove(destPath)
				return fmt.Errorf("hash verification failed after replacement %s -> %s", src, destPath)
			}

			if !isSilent {
				fmt.Printf("Replaced %s → %s (higher quality, hardlink fallback to copy)\n", src, destPath)
			}
			return nil
		}

		if err := linkFile(src, destPath); err != nil {
			return fmt.Errorf("failed to link file %s to %s: %w", src, destPath, err)
		}
		// Hardlinks share the same inode - no verification needed
		if !isSilent {
			fmt.Printf("Linked %s → %s (shared inode)\n", src, destPath)
		}

		// Log to session and create browse hardlink
		if session != nil {
			hash, _ := fileHash(src)
			size, _ := getFileSize(destPath)
			browsePath, err := session.CreateHardlink(destPath)
			if err != nil {
				fmt.Printf("Warning: failed to create import browser link: %v\n", err)
			} else {
				session.LogCopied(src, destPath, hash, size, browsePath)
			}
		}

		return nil
	}

	// Atomic copy with integrity verification
	copyAttempts := 0
	for {
		copyAttempts++
		if err := copyFileAtomic(src, destPath); err != nil {
			if errors.Is(err, os.ErrExist) && copyAttempts == 1 {
				destPath = timestampSuffixCopyPath(origDestPath)
				if !isSilent {
					fmt.Printf("Destination exists, retrying with %s\n", destPath)
				}
				continue
			}
			return fmt.Errorf("failed to copy file %s to %s: %w", src, destPath, err)
		}
		break
	}

	// Verify integrity with SHA256 comparison
	srcHash, err := fileHash(src)
	if err != nil {
		return fmt.Errorf("failed to hash source %s: %w", src, err)
	}

	destHash, err := fileHash(destPath)
	if err != nil {
		return fmt.Errorf("failed to hash destination %s: %w", destPath, err)
	}

	if srcHash != destHash {
		// Remove bad copy so it is not trusted later
		_ = os.Remove(destPath)
		return fmt.Errorf("hash verification failed after copy %s -> %s", src, destPath)
	}

	if !isSilent {
		fmt.Printf("Copied %s → %s\n", src, destPath)
	}

	// Log to session and create browse hardlink
	if session != nil {
		size, _ := getFileSize(destPath)
		browsePath, err := session.CreateHardlink(destPath)
		if err != nil {
			fmt.Printf("Warning: failed to create import browser link: %v\n", err)
		} else {
			// Check if this was a timestamped copy (collision resolution)
			if destPath != origDestPath {
				session.LogCopiedTimestamped(src, destPath, srcHash, size, browsePath)
			} else {
				session.LogCopied(src, destPath, srcHash, size, browsePath)
			}
		}
	}

	return nil
}
