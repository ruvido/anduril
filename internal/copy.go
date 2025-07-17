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

	"github.com/rwcarlsen/goexif/exif"
	"github.com/abema/go-mp4"
)

// Global errors
var (
	ErrNoExifDate = errors.New("no EXIF or MP4 date tags found")
)

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
	out.Close()

	return os.Rename(tmp, dest)
}

// placeholder for future image quality check
func checkImageQualityEqual(path1, path2 string) (bool, error) {
	// TODO: implement actual quality check (e.g. resolution, bitrate, etc)
	return false, nil
}

// getExifDateOriginal returns the first valid EXIF timestamp found
func getExifDateOriginal(path string) (*time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return nil, err
	}

	tags := []exif.FieldName{
		exif.DateTimeOriginal,
		exif.DateTimeDigitized,
		"CreateDate",
		"MediaCreateDate",
		"TrackCreateDate",
	}

	for _, tag := range tags {
		t, err := x.Get(tag)
		if err != nil {
			continue
		}
		timeStr := strings.Trim(t.String(), `"`)
		dt, err := time.Parse("2006:01:02 15:04:05", timeStr)
		if err != nil {
			continue
		}
		return &dt, nil
	}
	return nil, ErrNoExifDate
}

// getVideoCreateDateMP4 extracts creation time from MP4 metadata (mvhd)
func getVideoCreateDateMP4(path string) (*time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var creationTime uint64
	_, err = mp4.ReadBoxStructure(f, func(h *mp4.ReadHandle) (interface{}, error) {
	if h.BoxInfo.Type == mp4.BoxTypeMvhd() {
		box, _, err := h.ReadPayload()
		if err != nil {
			return nil, err
		}

		mvhd, ok := box.(*mp4.Mvhd)
		if !ok {
			return nil, errors.New("failed to parse mvhd box")
		}

		// Handle different MVHD versions
		if mvhd.GetVersion() == 0 {
			creationTime = uint64(mvhd.CreationTimeV0)
		} else {
			creationTime = mvhd.CreationTimeV1
		}
		return nil, io.EOF
	}
	
		// if h.BoxInfo.Type == mp4.BoxTypeMvhd() {
		// 	// Read the entire mvhd box
		// 	box, _, err := h.ReadPayload()
		// 	if err != nil {
		// 		return nil, err
		// 	}
		//
		// 	mvhd, ok := box.(*mp4.Mvhd)
		// 	if !ok {
		// 		return nil, errors.New("failed to parse mvhd box")
		// 	}
		//
		// 	creationTime = uint64(mvhd.CreationTime)
		// 	return nil, io.EOF // Stop parsing
		// }
		return nil, nil
	})

	if err != nil && err != io.EOF {
		return nil, err
	}

	if creationTime == 0 {
		return nil, ErrNoExifDate
	}

	// MP4 timestamps are based on 1904-01-01
	baseTime := time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	t := baseTime.Add(time.Duration(creationTime) * time.Second)
	return &t, nil
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
		if isImage {
			exifDate, err := getExifDateOriginal(src)
			if err == nil && exifDate != nil {
				fileDate = *exifDate
				gotExif = true
			}
		} else if isVideo {
			vidDate, err := getVideoCreateDateMP4(src)
			if err == nil && vidDate != nil {
				fileDate = *vidDate
				gotExif = true
			}
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
