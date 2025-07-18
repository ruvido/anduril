package internal

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	exif "github.com/rwcarlsen/goexif/exif"
	exiftool "github.com/barasher/go-exiftool"
)

// Global errors
var (
	ErrNoExifDate = errors.New("no EXIF or media creation date found")
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

// placeholder for future image quality check
func checkImageQualityEqual(path1, path2 string) (bool, error) {
	// TODO: implement actual quality check (e.g. resolution, bitrate, etc)
	return false, nil
}

// getCaptureTimestampNative uses goexif to get date for supported image files
func getCaptureTimestampNative(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}, err
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
	// Initialize exiftool
	et, err := exiftool.NewExiftool()
	if err != nil {
		return time.Time{}, fmt.Errorf("exiftool not installed: %w", err)
	}
	defer et.Close()

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
	ext := strings.ToLower(filepath.Ext(src))
	
	// Determine file type
	isImage := false
	for _, e := range cfg.ImageExt {
		if ext == e {
			isImage = true
			break
		}
	}
	
	isVideo := false
	for _, e := range cfg.VideoExt {
		if ext == e {
			isVideo = true
			break
		}
	}
	
	isMedia := isImage || isVideo
	var fileDate time.Time
	var err error
	gotExif := false
	
	// Try to get metadata for ALL media files
	if isMedia {
		captureTime, err := GetCaptureTimestamp(src, cfg.UseExifTool)
		if err == nil {
			fileDate = captureTime
			gotExif = true
		} else if !errors.Is(err, ErrNoExifDate) {
			// Only log real errors (not missing metadata)
			fmt.Printf("Warning: metadata extraction failed for %s: %v\n", src, err)
		}
	}
	
	// Fallback to modification time if no metadata found
	if !gotExif {
		fileDate, err = getFileModTime(src)
		if err != nil {
			return fmt.Errorf("failed to get file date for %s: %w", src, err)
		}
	}
	
	// Determine destination based on file type and metadata status
	destBase := filepath.Base(src)
	var destDir string
	
	switch {
	case isVideo && gotExif:
		// Videos with metadata: /library_video/user/YYYY/MM/DD/
		destDir = filepath.Join(cfg.VideoLib, user,
			fmt.Sprintf("%04d", fileDate.Year()),
			fmt.Sprintf("%02d", fileDate.Month()),
			fmt.Sprintf("%02d", fileDate.Day()))
	
	case isVideo && !gotExif:
		// Videos without metadata: /library_video/user/noexif/YYYY-MM/
		destDir = filepath.Join(cfg.VideoLib, user, "noexif",
			fmt.Sprintf("%04d-%02d", fileDate.Year(), fileDate.Month()))
	
	case isImage && gotExif:
		// Images with EXIF: /library/user/YYYY/MM/DD/
		destDir = filepath.Join(cfg.Library, user,
			fmt.Sprintf("%04d", fileDate.Year()),
			fmt.Sprintf("%02d", fileDate.Month()),
			fmt.Sprintf("%02d", fileDate.Day()))
	
	case isImage && !gotExif:
		// Images without EXIF: /library/user/noexif/YYYY-MM/
		destDir = filepath.Join(cfg.Library, user, "noexif",
			fmt.Sprintf("%04d-%02d", fileDate.Year(), fileDate.Month()))
	
	default:
		// Non-media files: /library/user/noexif/YYYY-MM/
		destDir = filepath.Join(cfg.Library, user, "noexif",
			fmt.Sprintf("%04d-%02d", fileDate.Year(), fileDate.Month()))
	}
	
	if dryRun {
		fmt.Printf("[dry-run] would copy %s → %s\n", src, filepath.Join(destDir, destBase))
		return nil
	}
	
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}
	
	destPath := filepath.Join(destDir, destBase)
	
	if _, err := os.Stat(destPath); err == nil {
		srcHash, err := fileHash(src)
		if err != nil {
			return fmt.Errorf("failed to hash src file %s: %w", src, err)
		}
		destHash, err := fileHash(destPath)
		if err != nil {
			return fmt.Errorf("failed to hash dest file %s: %w", destPath, err)
		}
		if srcHash == destHash {
			fmt.Printf("Skipping duplicate file: %s\n", src)
			return nil
		}
		
		if isImage {
			sameQuality, err := checkImageQualityEqual(src, destPath)
			if err != nil {
				fmt.Printf("Warning: quality check failed for %s and %s: %v\n", src, destPath, err)
			} else if sameQuality {
				fmt.Printf("Skipping file (same quality different hash): %s\n", src)
				return nil
			}
		}
		
		destPath = safeCopyPath(destPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %w", destPath, err)
	}
	
	if err := copyFileAtomic(src, destPath); err != nil {
		return fmt.Errorf("failed to copy file %s to %s: %w", src, destPath, err)
	}
	
	fmt.Printf("Copied %s → %s\n", src, destPath)
	return nil
}
