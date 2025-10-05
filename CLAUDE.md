# Anduril - Smart Media File Organizer

## Project Overview

Anduril is a Go CLI tool for organizing large media collections by capture date with intelligent duplicate handling and quality-based file replacement. Features multi-level date detection, messaging app filename support, atomic file operations, and intelligent folder analysis.

## Current Status

✅ **Implemented:**
- **Import command** with advanced EXIF/metadata extraction
- **Analytics command** for intelligent folder analysis with browseable output
- Multi-level date detection (EXIF → filename patterns → file timestamps)
- Messaging app filename pattern recognition (Signal, WhatsApp, Telegram, Instagram)
- Smart duplicate detection with SHA256 hash verification
- Quality-based image/video comparison without artificial thresholds
- Date confidence scoring system (HIGH/MEDIUM/LOW/VERY_LOW)
- Atomic file operations with integrity verification
- Global ExifTool instance for performance optimization
- Real-time progress tracking for large folder scans
- Hardlink-based browse structures for file organization
- Comprehensive test coverage for quality and pattern matching
- Structured configuration management with TOML

🚧 **Planned:** Server mode with PocketBase integration (see prompt-server.md)

## Project Structure

```
anduril/
├── main.go                    # Entry point
├── cmd/                       # CLI commands
│   ├── root.go               # Root cobra command
│   ├── import.go             # Import subcommand implementation
│   ├── analytics.go          # Analytics subcommand implementation
│   └── server.go             # Server subcommand (basic structure)
├── internal/                  # Internal packages
│   ├── config.go             # Configuration management (Viper + TOML)
│   ├── copy.go               # File copying, organization, and quality logic
│   ├── copy_test.go          # Quality check tests
│   ├── analytics.go          # Folder analysis with progress tracking
│   ├── browse.go             # Hardlink browse structure creation
│   ├── watcher.go            # Filesystem watcher (for server mode)
│   ├── log.go                # Logging utilities  
│   └── media.go              # Media file processing (EXIF extraction)
└── go.mod                    # Go module definition
```

## Core Functionality

### Import Command
```bash
anduril import [--user USER] [--library LIBRARY] [--dry-run] [--exiftool] INPUT_DIR
```

**Features:**
- **Multi-level date detection**: EXIF metadata → filename patterns → file timestamps
- **Messaging app support**: Recognizes Signal, WhatsApp, Telegram, Instagram filename patterns  
- **Date confidence scoring**: HIGH/MEDIUM/LOW/VERY_LOW confidence levels
- **Smart organization**: High confidence files go to `YYYY/MM/DD/`, low confidence to `noexif/YYYY-MM/`
- **Quality-based deduplication**: Keeps highest quality version without artificial thresholds
- **Video support**: Full video metadata extraction and quality comparison
- **Atomic operations**: Safe copying with SHA256 verification
- **Performance optimized**: Global ExifTool instance, native Go libraries

### Analytics Command
```bash
anduril analytics [--browse] [--duplicates] [--media-only] [--max-depth N] FOLDER
```

**Features:**
- **Intelligent folder scanning**: Skips noise folders (node_modules, cache, Lightroom previews) for performance
- **File type categorization**: Images, Videos, Documents, Books, Code, Config, Archives, Audio
- **Project detection**: Identifies Git repos, Node.js, Python, Go, Rust projects
- **Real-time progress tracking**: Shows files/dirs scanned with scan rate
- **Large file detection**: Identifies files >100MB with categorization
- **Duplicate analysis**: Optional SHA256-based duplicate detection for media files
- **Browseable output**: Creates hardlink-organized `.browse/` folder structure
- **Media insights**: Quality distribution and date range analysis

### Advanced Date Detection

**Date Confidence Levels:**
- **HIGH**: EXIF DateTimeOriginal, CreateDate metadata
- **MEDIUM**: Filename pattern parsing (20240315_143022, IMG-20240315-WA0001)
- **LOW**: File creation time
- **VERY_LOW**: File modification time (fallback)

**Supported Filename Patterns:**
- Generic: `20240315_143022`, `IMG_20240315_143022`, `2024-03-15-14-30-22`
- WhatsApp: `IMG-20240315-WA0001`, `VID-20240315-WA0002`  
- Signal: `signal-2024-03-15-143022`
- Telegram: `telegram-2024-03-15-14-30-22`
- InShot: `inshot-2024-03-15-143022`

### Quality Comparison System

**Image Quality Logic:**
1. **Resolution priority**: Higher pixel count (width × height) wins
2. **Compression quality**: For same resolution, larger file size wins
3. **No artificial thresholds**: Direct comparison for accurate results

**Video Quality Logic:**
1. **Duration validation**: >5 second difference = different videos (no comparison)
2. **Resolution priority**: Higher pixel count wins  
3. **Bitrate quality**: For same resolution, larger file size wins

### Smart Duplicate Resolution

```
if file_exists_at_destination:
    if identical_content (SHA256 hash match):
        skip_file
    elif is_media_file:
        quality_result = compare_quality(new, existing)
        if quality_result == HIGHER:
            replace_existing_file
        elif quality_result == EQUAL:
            skip_file
        elif quality_result == LOWER:
            copy_with_suffix (_2, _3, etc.)
        else: // UNKNOWN
            copy_with_suffix
    else:
        copy_with_suffix
else:
    copy_file
```

## Dependencies

- **CLI Framework**: `github.com/spf13/cobra` + `github.com/spf13/viper`
- **EXIF Processing**: 
  - `github.com/barasher/go-exiftool` (primary, handles all formats)
  - `github.com/rwcarlsen/goexif` (fallback for common images)
- **Image Processing**: Standard library `image/*` packages

## Configuration

Default config: `~/.config/anduril/anduril.toml`

```toml
user = "username"
library = "/home/user/anduril/images"
videolibrary = "/home/user/anduril/videos"
image_extensions = [".jpg", ".jpeg", ".png", ".gif", ".heic"]
video_extensions = [".mp4", ".mov", ".avi", ".mkv"]
```

## Build and Test Commands

```bash
# Build the application
go build -o anduril

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal -v

# Format code
go fmt ./...

# Lint code  
go vet ./...

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o anduril-linux
GOOS=windows GOARCH=amd64 go build -o anduril-windows.exe
```

## Code Conventions

1. **Package Structure**: Use `internal/` for private packages, `cmd/` for CLI commands
2. **Error Handling**: Always handle errors explicitly, use wrapped errors with context
3. **File Operations**: Atomic operations (temp file + rename) for data integrity
4. **Testing**: Unit tests for all core logic, table-driven tests preferred
5. **Configuration**: Centralized config with sensible defaults
6. **Logging**: Structured output with clear operation descriptions

## Testing

**Quality Check Tests** (`internal/copy_test.go`):
- `TestGetImageResolution`: Verifies image dimension detection
- `TestIsHigherQuality`: Tests quality comparison logic
- `TestCheckImageQualityEqual`: Tests similar quality detection
- `TestGetFileSize`: Validates file size retrieval

**Test Coverage:**
- Image resolution detection ✅
- Quality comparison algorithms ✅  
- File size analysis ✅
- Filename pattern parsing ✅
- Video metadata extraction ✅
- Date confidence scoring ✅
- Edge cases and error handling ✅

## Key Quality Features

**File Integrity:**
- SHA256 hash verification for duplicate detection
- Atomic file operations prevent corruption
- Safe path generation for naming conflicts

**Smart Organization:**
- Multi-level date detection with confidence scoring
- EXIF/metadata extraction for precise timestamps
- Filename pattern parsing for messaging apps
- Intelligent folder structure based on date confidence
- Video and image support with format-specific handling

**Performance:**
- Global ExifTool instance reuse across files
- Native Go libraries for common image formats
- Optimized regex patterns (most common first)
- Lazy metadata extraction (only when needed for quality comparison)

## Future Architecture (Server Mode)

Planned server mode will add:
- Embedded PocketBase for user management and API
- Real-time filesystem watching with fsnotify  
- Web interface for photo browsing and album management
- Static file serving from organized library

## Target Users

Home users, photographers, and power users who need to:
- Import photos from phones, SD cards, or legacy drives
- Organize large media collections automatically  
- Maintain a clean, date-based file structure
- Handle duplicates with quality preservation
- Avoid data loss during organization
## Latest Architecture Improvements

**Code Streamlining (Less is More):**
- Refactored `ProcessFile` from 148 lines to 47 lines
- Extracted helper functions: `determineFileType`, `generateDestinationPath`, `handleDuplicateFile`
- Removed artificial quality thresholds for more accurate comparison
- Fixed critical --exiftool flag bug in cmd/import.go:38
- Optimized ExifTool usage with global instance reuse

**Key Bug Fixes:**
- ✅ Fixed --exiftool flag not being passed to config (cmd/import.go:38-40)
- ✅ Removed 10% file size threshold causing incorrect quality comparisons
- ✅ Added proper ExifTool cleanup with defer statement
- ✅ Fixed regex pattern ordering for better filename matching performance