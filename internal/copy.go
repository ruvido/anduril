package internal

import (
    "crypto/sha256"
    "fmt"
    "io"
    "os"
    "path/filepath"
	"errors"
    "strings"
    "time"

    "github.com/rwcarlsen/goexif/exif"
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

func ProcessFile(src string, cfg *Config, user string, library string, dryRun bool) error {
    ext := strings.ToLower(filepath.Ext(src))

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

    var fileDate time.Time
    var err error

    if isImage {
        fileDate, err = getExifDateOriginal(src)
        if err != nil {
            fileDate = time.Time{}
        }
    }

    if fileDate.IsZero() {
        fileDate, err = getFileModTime(src)
        if err != nil {
            return fmt.Errorf("failed to get file date for %s: %w", src, err)
        }
    }

    var destDir string
    var destBase = filepath.Base(src)

    if isVideo {
        // put in video library root: /library_video/user/YYYY/MM/DD/
        destDir = filepath.Join(cfg.VideoLib, user,
            fmt.Sprintf("%04d", fileDate.Year()),
            fmt.Sprintf("%02d", fileDate.Month()),
            fmt.Sprintf("%02d", fileDate.Day()))
    } else if isImage && !fileDate.IsZero() {
        // image with EXIF date: /library/user/YYYY/MM/DD/
        destDir = filepath.Join(cfg.Library, user,
            fmt.Sprintf("%04d", fileDate.Year()),
            fmt.Sprintf("%02d", fileDate.Month()),
            fmt.Sprintf("%02d", fileDate.Day()))
    } else {
        // no exif or non-image: /library/user/noexif/YYYY-MM/
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


// func ProcessFile(src string, cfg *Config, user string, library string, dryRun bool) error {
//     ext := strings.ToLower(filepath.Ext(src))
//     isImage := false
//     for _, e := range cfg.ImageExt {
//         if ext == e {
//             isImage = true
//             break
//         }
//     }
//
//     var fileDate time.Time
//     var err error
//
//     if isImage {
//         // try EXIF date extraction
//         fileDate, err = getExifDate(src)
//         if err != nil {
//             // no exif date, fallback below
//             fileDate = time.Time{}
//         }
//     }
//
//     if fileDate.IsZero() {
//         // fallback to mod time
//         fileDate, err = getFileModTime(src)
//         if err != nil {
//             return fmt.Errorf("failed to get file date for %s: %w", src, err)
//         }
//     }
//
//     var destDir string
//     var destBase string = filepath.Base(src)
//
//     if isImage && !fileDate.IsZero() {
//         // e.g. /library/user/YYYY/MM/DD/
//         destDir = filepath.Join(library, user,
//             fmt.Sprintf("%04d", fileDate.Year()),
//             fmt.Sprintf("%02d", fileDate.Month()),
//             fmt.Sprintf("%02d", fileDate.Day()))
//     } else {
//         // no exif or non-image: /library/noexif/YYYY-MM/
//         destDir = filepath.Join(library, "noexif",
//             fmt.Sprintf("%04d-%02d", fileDate.Year(), fileDate.Month()))
//     }
//
//     if dryRun {
//         fmt.Printf("[dry-run] would copy %s → %s\n", src, filepath.Join(destDir, destBase))
//         return nil
//     }
//
//     // Ensure destDir exists
//     if err := os.MkdirAll(destDir, 0755); err != nil {
//         return fmt.Errorf("failed to create directory %s: %w", destDir, err)
//     }
//
//     destPath := filepath.Join(destDir, destBase)
//
//     // Check if destination file exists
//     if _, err := os.Stat(destPath); err == nil {
//         // file exists, compare hashes
//         srcHash, err := fileHash(src)
//         if err != nil {
//             return fmt.Errorf("failed to hash src file %s: %w", src, err)
//         }
//
//         destHash, err := fileHash(destPath)
//         if err != nil {
//             return fmt.Errorf("failed to hash dest file %s: %w", destPath, err)
//         }
//
//         if srcHash == destHash {
//             // duplicate file, skip copying
//             fmt.Printf("Skipping duplicate file: %s\n", src)
//             return nil
//         }
//
//         // hashes differ, check image quality if image
//         if isImage {
//             sameQuality, err := checkImageQualityEqual(src, destPath)
//             if err != nil {
//                 // log but continue with safe renaming
//                 fmt.Printf("Warning: quality check failed for %s and %s: %v\n", src, destPath, err)
//             } else if sameQuality {
//                 fmt.Printf("Skipping file (same quality different hash): %s\n", src)
//                 return nil
//             }
//         }
//
//         // safe rename to avoid overwriting
//         destPath = safeCopyPath(destPath)
//     } else if !errors.Is(err, os.ErrNotExist) {
//         // Stat failed with unknown error
//         return fmt.Errorf("failed to stat %s: %w", destPath, err)
//     }
//
//     // Finally, copy the file atomically
//     if err := copyFileAtomic(src, destPath); err != nil {
//         return fmt.Errorf("failed to copy file %s to %s: %w", src, destPath, err)
//     }
//
//     fmt.Printf("Copied %s → %s\n", src, destPath)
//     return nil
// }

// getExifDate extracts the DateTimeOriginal from EXIF metadata
func getExifDateOriginal(path string) (time.Time, error) {
    f, err := os.Open(path)
    if err != nil {
        return time.Time{}, err
    }
    defer f.Close()

    x, err := exif.Decode(f)
    if err != nil {
        return time.Time{}, err
    }

    tag, err := x.Get(exif.DateTimeOriginal)
    if err != nil {
        return time.Time{}, err
    }

    dateStr, err := tag.StringVal()
    if err != nil {
        return time.Time{}, err
    }

    return time.Parse("2006:01:02 15:04:05", dateStr)
}
// func getExifDate(path string) (time.Time, error) {
//     f, err := os.Open(path)
//     if err != nil {
//         return time.Time{}, err
//     }
//     defer f.Close()
//
//     x, err := exif.Decode(f)
//     if err != nil {
//         return time.Time{}, err
//     }
//
//     dt, err := x.DateTime()
//     if err != nil {
//         return time.Time{}, err
//     }
//     return dt, nil
// }
