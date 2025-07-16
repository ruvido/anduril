package internal

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// ScanMediaFiles scans input directory recursively for media files based on extensions
func ScanMediaFiles(inputDir string, cfg *Config) ([]string, error) {
    var files []string
    err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() {
            return nil
        }

        ext := strings.ToLower(filepath.Ext(info.Name()))
        for _, e := range cfg.ImageExt {
            if ext == e {
                files = append(files, path)
                return nil
            }
        }
        for _, e := range cfg.VideoExt {
            if ext == e {
                files = append(files, path)
                return nil
            }
        }
        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("error scanning files: %w", err)
    }
    return files, nil
}
