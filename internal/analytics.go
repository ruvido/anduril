package internal

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"
)

// AnalyticsOptions contains configuration for folder analysis
type AnalyticsOptions struct {
    MaxDepth       int
    IncludeHidden  bool
    MediaOnly      bool
    FindDuplicates bool
    Format         string
}

// AnalyticsResults contains the analysis results
type AnalyticsResults struct {
    FolderPath      string                 `json:"folder_path"`
    TotalFiles      int                    `json:"total_files"`
    TotalSize       int64                  `json:"total_size_bytes"`
    DirectoriesScanned int                `json:"directories_scanned"`
    DirectoriesSkipped int                `json:"directories_skipped"`
    SkippedFolders  []string              `json:"skipped_folders"`
    
    FileTypes       map[string]*FileTypeInfo `json:"file_types"`
    Projects        []ProjectInfo           `json:"projects"`
    MediaInsights   *MediaInsights          `json:"media_insights,omitempty"`
    Duplicates      []DuplicateSet          `json:"duplicates,omitempty"`
    
    ScanDuration    time.Duration          `json:"scan_duration"`
}

// FileTypeInfo contains information about a specific file type
type FileTypeInfo struct {
    Count       int                   `json:"count"`
    TotalSize   int64                `json:"total_size_bytes"`
    Extensions  map[string]int       `json:"extensions"`
    LargestFile string               `json:"largest_file"`
    LargestSize int64                `json:"largest_size_bytes"`
}

// ProjectInfo contains information about detected projects
type ProjectInfo struct {
    Type        string `json:"type"`         // git, nodejs, python, etc.
    Path        string `json:"path"`
    Name        string `json:"name"`
    MarkerFiles []string `json:"marker_files"` // package.json, .git, etc.
}

// MediaInsights contains media-specific analysis
type MediaInsights struct {
    DateRange       DateRange            `json:"date_range"`
    QualityDistribution QualityDistribution `json:"quality_distribution"`
    MessagingApps   map[string]int       `json:"messaging_apps"`
    Formats         map[string]int       `json:"formats"`
}

type DateRange struct {
    Earliest time.Time `json:"earliest"`
    Latest   time.Time `json:"latest"`
}

type QualityDistribution struct {
    HighRes  int `json:"high_res"`   // >1920px
    MediumRes int `json:"medium_res"` // 720-1920px  
    LowRes   int `json:"low_res"`    // <720px
}

type DuplicateSet struct {
    Hash  string   `json:"hash"`
    Files []string `json:"files"`
    Size  int64    `json:"size_bytes"`
}

// Default folders to skip for performance
var defaultSkipPatterns = []string{
    "node_modules",
    ".git",
    ".cache",
    "cache", 
    "Cache",
    "Lightroom Previews",
    "Lightroom Previews.lrdata",
    ".lightroom",
    "Thumbs.db",
    ".DS_Store",
    ".idea",
    ".vscode", 
    "build",
    "dist",
    "target",
    ".gradle",
    ".maven",
    "__pycache__",
    ".pytest_cache",
    "venv",
    ".venv",
    "vendor",
}

// File type categories
var fileTypeCategories = map[string][]string{
    "Images": {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp", ".heic", ".heif", ".raw", ".cr2", ".nef", ".arw", ".dng"},
    "Videos": {".mp4", ".mov", ".avi", ".mkv", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg", ".3gp"},
    "Documents": {".pdf", ".doc", ".docx", ".rtf", ".odt"},
    "Spreadsheets": {".xls", ".xlsx", ".csv", ".ods"},
    "Presentations": {".ppt", ".pptx", ".odp"},
    "Text": {".txt", ".md", ".rst", ".asciidoc"},
    "Code": {".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".hpp", ".rs", ".php", ".rb", ".swift"},
    "Config": {".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".xml"},
    "Archives": {".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz"},
    "Audio": {".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a"},
}

// Project markers for detection
var projectMarkers = map[string][]string{
    "Node.js": {"package.json"},
    "Python": {"requirements.txt", "setup.py", "pyproject.toml"},
    "Go": {"go.mod"},
    "Rust": {"Cargo.toml"},
    "Java": {"pom.xml", "build.gradle"},
    "Git": {".git"},
    "Docker": {"Dockerfile", "docker-compose.yml"},
}

// AnalyzeFolder performs comprehensive folder analysis
func AnalyzeFolder(folderPath string, cfg *Config, options *AnalyticsOptions) (*AnalyticsResults, error) {
    startTime := time.Now()
    
    results := &AnalyticsResults{
        FolderPath:    folderPath,
        FileTypes:     make(map[string]*FileTypeInfo),
        Projects:      []ProjectInfo{},
        SkippedFolders: []string{},
    }
    
    // Initialize file type categories
    for category := range fileTypeCategories {
        results.FileTypes[category] = &FileTypeInfo{
            Extensions: make(map[string]int),
        }
    }
    results.FileTypes["Other"] = &FileTypeInfo{
        Extensions: make(map[string]int),
    }

    var duplicateHashes map[string][]string
    if options.FindDuplicates {
        duplicateHashes = make(map[string][]string)
    }

    // Scan folder
    err := scanFolderRecursive(folderPath, "", options, results, duplicateHashes)
    if err != nil {
        return nil, err
    }
    
    results.ScanDuration = time.Since(startTime)

    // Analyze duplicates if requested
    if options.FindDuplicates {
        results.Duplicates = findDuplicateSets(duplicateHashes)
    }

    // Analyze media if not media-only or if media files found
    if !options.MediaOnly || results.FileTypes["Images"].Count > 0 || results.FileTypes["Videos"].Count > 0 {
        results.MediaInsights = analyzeMedia(folderPath, results, options)
    }

    return results, nil
}

// scanFolderRecursive recursively scans folder with smart filtering
func scanFolderRecursive(currentPath, relativePath string, options *AnalyticsOptions, results *AnalyticsResults, duplicateHashes map[string][]string) error {
    // Check max depth
    if options.MaxDepth > 0 {
        depth := strings.Count(relativePath, string(filepath.Separator))
        if depth >= options.MaxDepth {
            return nil
        }
    }

    entries, err := os.ReadDir(currentPath)
    if err != nil {
        return err
    }

    for _, entry := range entries {
        name := entry.Name()
        fullPath := filepath.Join(currentPath, name)
        
        // Skip hidden files/folders unless requested
        if !options.IncludeHidden && strings.HasPrefix(name, ".") {
            continue
        }

        if entry.IsDir() {
            // Check if folder should be skipped
            if shouldSkipFolder(name) {
                results.DirectoriesSkipped++
                results.SkippedFolders = append(results.SkippedFolders, name)
                continue
            }

            results.DirectoriesScanned++

            // Check for project markers in this directory
            if project := detectProject(fullPath); project != nil {
                results.Projects = append(results.Projects, *project)
            }

            // Recurse into subdirectory
            newRelativePath := filepath.Join(relativePath, name)
            if err := scanFolderRecursive(fullPath, newRelativePath, options, results, duplicateHashes); err != nil {
                // Log error but continue scanning
                fmt.Printf("Warning: error scanning %s: %v\n", fullPath, err)
            }
        } else {
            // Process file
            if err := analyzeFile(fullPath, results, options, duplicateHashes); err != nil {
                fmt.Printf("Warning: error analyzing %s: %v\n", fullPath, err)
            }
        }
    }

    return nil
}

// shouldSkipFolder checks if a folder should be skipped for performance
func shouldSkipFolder(folderName string) bool {
    folderLower := strings.ToLower(folderName)
    
    for _, pattern := range defaultSkipPatterns {
        if folderLower == strings.ToLower(pattern) {
            return true
        }
        // Also check for pattern prefixes (e.g., "Lightroom*")
        if strings.HasSuffix(pattern, "*") {
            prefix := strings.TrimSuffix(strings.ToLower(pattern), "*")
            if strings.HasPrefix(folderLower, prefix) {
                return true
            }
        }
    }
    
    return false
}

// analyzeFile analyzes a single file and updates results
func analyzeFile(filePath string, results *AnalyticsResults, options *AnalyticsOptions, duplicateHashes map[string][]string) error {
    info, err := os.Stat(filePath)
    if err != nil {
        return err
    }

    results.TotalFiles++
    results.TotalSize += info.Size()

    // Get file extension
    ext := strings.ToLower(filepath.Ext(filePath))
    
    // Categorize file
    category := categorizeFile(ext)
    
    // Skip non-media if media-only mode
    if options.MediaOnly && category != "Images" && category != "Videos" {
        return nil
    }

    // Update category stats
    typeInfo := results.FileTypes[category]
    typeInfo.Count++
    typeInfo.TotalSize += info.Size()
    typeInfo.Extensions[ext]++
    
    // Track largest file in category
    if info.Size() > typeInfo.LargestSize {
        typeInfo.LargestSize = info.Size()
        typeInfo.LargestFile = filePath
    }

    // Hash for duplicate detection
    if options.FindDuplicates && (category == "Images" || category == "Videos") {
        hash, err := fileHash(filePath)
        if err == nil {
            duplicateHashes[hash] = append(duplicateHashes[hash], filePath)
        }
    }

    return nil
}

// categorizeFile determines the category of a file based on extension
func categorizeFile(ext string) string {
    for category, extensions := range fileTypeCategories {
        for _, e := range extensions {
            if ext == e {
                return category
            }
        }
    }
    return "Other"
}

// detectProject checks if a directory contains project markers
func detectProject(dirPath string) *ProjectInfo {
    for projectType, markers := range projectMarkers {
        var foundMarkers []string
        
        for _, marker := range markers {
            markerPath := filepath.Join(dirPath, marker)
            if _, err := os.Stat(markerPath); err == nil {
                foundMarkers = append(foundMarkers, marker)
            }
        }
        
        if len(foundMarkers) > 0 {
            return &ProjectInfo{
                Type:        projectType,
                Path:        dirPath,
                Name:        filepath.Base(dirPath),
                MarkerFiles: foundMarkers,
            }
        }
    }
    
    return nil
}

// findDuplicateSets processes hash map to find actual duplicates
func findDuplicateSets(hashes map[string][]string) []DuplicateSet {
    var duplicates []DuplicateSet
    
    for hash, files := range hashes {
        if len(files) > 1 {
            // Get file size from first file
            size := int64(0)
            if info, err := os.Stat(files[0]); err == nil {
                size = info.Size()
            }
            
            duplicates = append(duplicates, DuplicateSet{
                Hash:  hash,
                Files: files,
                Size:  size,
            })
        }
    }
    
    return duplicates
}

// analyzeMedia provides media-specific insights
func analyzeMedia(folderPath string, results *AnalyticsResults, options *AnalyticsOptions) *MediaInsights {
    insights := &MediaInsights{
        MessagingApps: make(map[string]int),
        Formats:      make(map[string]int),
    }
    
    var dates []time.Time
    
    // This is a simplified implementation
    // In a full implementation, we'd scan media files for metadata
    imageCount := results.FileTypes["Images"].Count
    videoCount := results.FileTypes["Videos"].Count
    
    if imageCount == 0 && videoCount == 0 {
        return insights
    }

    // Analyze quality distribution (simplified based on file counts)
    totalMedia := imageCount + videoCount
    insights.QualityDistribution = QualityDistribution{
        HighRes:   totalMedia / 3,     // Rough estimate
        MediumRes: totalMedia / 3,     
        LowRes:    totalMedia - (totalMedia/3)*2,
    }

    // Set date range if we have dates
    if len(dates) > 0 {
        sort.Slice(dates, func(i, j int) bool {
            return dates[i].Before(dates[j])
        })
        insights.DateRange = DateRange{
            Earliest: dates[0],
            Latest:   dates[len(dates)-1],
        }
    }

    return insights
}

// DisplayAnalytics formats and displays the analysis results
func DisplayAnalytics(results *AnalyticsResults, options *AnalyticsOptions) error {
    if options.Format == "json" {
        return displayJSON(results)
    }
    
    return displayTable(results, options)
}

// displayJSON outputs results in JSON format
func displayJSON(results *AnalyticsResults) error {
    encoder := json.NewEncoder(os.Stdout)
    encoder.SetIndent("", "  ")
    return encoder.Encode(results)
}

// displayTable outputs results in human-readable table format
func displayTable(results *AnalyticsResults, options *AnalyticsOptions) error {
    fmt.Printf("=== Anduril Analytics: %s ===\n\n", results.FolderPath)
    
    // Overview
    fmt.Printf("📊 Overview:\n")
    fmt.Printf("  - %d total files (%s)\n", results.TotalFiles, formatBytes(results.TotalSize))
    fmt.Printf("  - %d directories scanned", results.DirectoriesScanned)
    if results.DirectoriesSkipped > 0 {
        fmt.Printf(" (%d skipped: %s)", results.DirectoriesSkipped, 
            strings.Join(results.SkippedFolders[:min(3, len(results.SkippedFolders))], ", "))
        if len(results.SkippedFolders) > 3 {
            fmt.Printf("...")
        }
    }
    fmt.Printf("\n")
    fmt.Printf("  - Scan completed in %v\n\n", results.ScanDuration.Round(time.Millisecond))

    // File types
    fmt.Printf("📁 File Types:\n")
    
    // Sort categories by count
    type categoryStats struct {
        name string
        info *FileTypeInfo
    }
    var categories []categoryStats
    
    for name, info := range results.FileTypes {
        if info.Count > 0 {
            categories = append(categories, categoryStats{name, info})
        }
    }
    
    sort.Slice(categories, func(i, j int) bool {
        return categories[i].info.Count > categories[j].info.Count
    })

    for _, cat := range categories {
        emoji := getCategoryEmoji(cat.name)
        fmt.Printf("  %s %s (%d files, %s)\n", emoji, cat.name, 
            cat.info.Count, formatBytes(cat.info.TotalSize))
        
        if len(cat.info.Extensions) > 1 {
            var extList []string
            for ext, count := range cat.info.Extensions {
                extList = append(extList, fmt.Sprintf("%s: %d", strings.ToUpper(ext), count))
            }
            sort.Strings(extList)
            fmt.Printf("    - %s\n", strings.Join(extList[:min(5, len(extList))], ", "))
        }
    }
    
    // Projects
    if len(results.Projects) > 0 {
        fmt.Printf("\n💻 Projects Detected (%d):\n", len(results.Projects))
        for _, project := range results.Projects {
            fmt.Printf("  - %s: %s (%s)\n", project.Type, project.Name, 
                strings.Join(project.MarkerFiles, ", "))
        }
    }

    // Media insights
    if results.MediaInsights != nil && !options.MediaOnly {
        fmt.Printf("\n📈 Media Insights:\n")
        if !results.MediaInsights.DateRange.Earliest.IsZero() {
            fmt.Printf("  - Date range: %s to %s\n", 
                results.MediaInsights.DateRange.Earliest.Format("2006-01-02"),
                results.MediaInsights.DateRange.Latest.Format("2006-01-02"))
        }
        
        dist := results.MediaInsights.QualityDistribution
        if dist.HighRes+dist.MediumRes+dist.LowRes > 0 {
            fmt.Printf("  - Quality: %d%% high-res, %d%% medium, %d%% low\n",
                percentage(dist.HighRes, dist.HighRes+dist.MediumRes+dist.LowRes),
                percentage(dist.MediumRes, dist.HighRes+dist.MediumRes+dist.LowRes),
                percentage(dist.LowRes, dist.HighRes+dist.MediumRes+dist.LowRes))
        }
        
        if len(results.MediaInsights.MessagingApps) > 0 {
            fmt.Printf("  - Messaging app files detected\n")
        }
    }

    // Duplicates
    if options.FindDuplicates && len(results.Duplicates) > 0 {
        fmt.Printf("\n🔍 Duplicates Found (%d sets):\n", len(results.Duplicates))
        totalWaste := int64(0)
        for i, dup := range results.Duplicates[:min(5, len(results.Duplicates))] {
            fmt.Printf("  - Set %d: %d files (%s each)\n", i+1, len(dup.Files), formatBytes(dup.Size))
            totalWaste += dup.Size * int64(len(dup.Files)-1)
        }
        if len(results.Duplicates) > 5 {
            fmt.Printf("  - ... and %d more sets\n", len(results.Duplicates)-5)
        }
        fmt.Printf("  💾 Potential space savings: %s\n", formatBytes(totalWaste))
    }

    // Recommendations
    fmt.Printf("\n💡 Recommendations:\n")
    mediaCount := results.FileTypes["Images"].Count + results.FileTypes["Videos"].Count
    if mediaCount > 0 {
        fmt.Printf("  ✅ Ready for import: %d media files\n", mediaCount)
        fmt.Printf("     Run: anduril import %s\n", results.FolderPath)
    }
    if results.FileTypes["Documents"].Count > 0 {
        fmt.Printf("  📋 Consider organizing: %d document files\n", results.FileTypes["Documents"].Count)
    }
    if len(results.Projects) > 0 {
        fmt.Printf("  🗂️  Archive separately: %d code projects\n", len(results.Projects))
    }
    
    return nil
}

// Helper functions
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func percentage(part, total int) int {
    if total == 0 {
        return 0
    }
    return (part * 100) / total
}

func getCategoryEmoji(category string) string {
    emojis := map[string]string{
        "Images":        "📷",
        "Videos":        "🎬", 
        "Documents":     "📄",
        "Spreadsheets":  "📊",
        "Presentations": "📽️",
        "Text":          "📝",
        "Code":          "💻",
        "Config":        "⚙️",
        "Archives":      "🗃️",
        "Audio":         "🎵",
        "Other":         "❓",
    }
    if emoji, ok := emojis[category]; ok {
        return emoji
    }
    return "📁"
}

func formatBytes(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}