package internal

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// CreateBrowseStructure creates a .browse folder with hardlinks organized by file type
func CreateBrowseStructure(results *AnalyticsResults) error {
    browseDir := filepath.Join(results.FolderPath, ".browse")
    
    // Create .browse directory
    if err := os.MkdirAll(browseDir, 0755); err != nil {
        return fmt.Errorf("failed to create browse directory: %w", err)
    }
    
    fmt.Printf("\n📂 Creating browse structure in %s\n", browseDir)
    
    // Track files processed for progress
    var totalFiles int
    for _, info := range results.FileTypes {
        totalFiles += info.Count
    }
    
    // Skip large files and process each category
    processed := 0
    for category, info := range results.FileTypes {
        if info.Count == 0 {
            continue
        }
        
        categoryDir := filepath.Join(browseDir, category)
        if err := os.MkdirAll(categoryDir, 0755); err != nil {
            fmt.Printf("Warning: failed to create %s directory: %v\n", category, err)
            continue
        }
        
        // Walk the source directory to find files of this category
        err := filepath.Walk(results.FolderPath, func(path string, info os.FileInfo, err error) error {
            if err != nil {
                return nil // Skip errors, continue processing
            }
            
            // Skip the .browse directory itself
            if strings.Contains(path, ".browse") {
                return nil
            }
            
            if info.IsDir() {
                return nil
            }
            
            // Check if file belongs to this category
            ext := strings.ToLower(filepath.Ext(path))
            if categorizeFile(ext) != category {
                return nil
            }
            
            // Skip large files (they're in summary only)
            const largeSizeThreshold = 100 * 1024 * 1024
            if info.Size() > largeSizeThreshold {
                return nil
            }
            
            // Create hardlink preserving directory structure
            relPath, err := filepath.Rel(results.FolderPath, path)
            if err != nil {
                return nil
            }
            
            linkPath := filepath.Join(categoryDir, relPath)
            linkDir := filepath.Dir(linkPath)
            
            // Create subdirectory structure
            if err := os.MkdirAll(linkDir, 0755); err != nil {
                return nil
            }
            
            // Create hardlink
            if err := os.Link(path, linkPath); err != nil {
                // If hardlink fails, skip (may be cross-filesystem)
                return nil
            }
            
            processed++
            if processed%100 == 0 {
                fmt.Printf("\r📋 Creating hardlinks: %d/%d", processed, totalFiles)
            }
            
            return nil
        })
        
        if err != nil {
            fmt.Printf("Warning: error processing %s category: %v\n", category, err)
        }
    }
    
    fmt.Printf("\r📋 Created hardlinks: %d files\n", processed)
    
    // Create a README in the browse directory
    readmePath := filepath.Join(browseDir, "README.txt")
    readmeContent := fmt.Sprintf(`Anduril Browse Structure
========================

This directory contains hardlinks to files organized by type.
Generated on: %s
Source folder: %s

Categories:
%s

Note: Large files (>100MB) are excluded from this browse structure.
They are listed in the analytics summary only.
`, results.ScanDuration, results.FolderPath, getCategoryList(results))
    
    if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
        fmt.Printf("Warning: failed to create README: %v\n", err)
    }
    
    return nil
}

func getCategoryList(results *AnalyticsResults) string {
    var list []string
    for category, info := range results.FileTypes {
        if info.Count > 0 {
            list = append(list, fmt.Sprintf("- %s: %d files", category, info.Count))
        }
    }
    return strings.Join(list, "\n")
}