# Anduril - Media File Organizer

## Project Overview

Anduril is a Go CLI tool for organizing large sets of media files (images/videos) by their capture date. Currently implements the **import command** with smart duplicate handling and quality-based file replacement.

## Current Status

✅ **Implemented:**
- Import command with EXIF/metadata extraction
- Smart duplicate detection (hash-based)
- Image quality comparison (resolution + file size)
- Safe file organization with atomic operations
- Comprehensive test coverage for quality checks
- Structured configuration management

🚧 **Planned:** Server mode with PocketBase integration (see prompt-server.md)

## Project Structure

```
anduril/
├── main.go                    # Entry point
├── cmd/                       # CLI commands
│   ├── root.go               # Root cobra command
│   └── import.go             # Import subcommand implementation
├── internal/                  # Internal packages
│   ├── config.go             # Configuration management (Viper + TOML)
│   ├── copy.go               # File copying, organization, and quality logic
│   ├── copy_test.go          # Quality check tests
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
- Extracts date from EXIF metadata (images) or file creation time (videos/fallback)
- Organizes files into `LIBRARY/user/YYYY/MM/DD/filename` structure  
- Files without metadata go to `LIBRARY/user/noexif/YYYY-MM/`
- Smart duplicate handling with quality comparison
- Atomic file operations with integrity verification
- Comprehensive logging

### Duplicate Resolution Logic

```
if file_exists_at_destination:
    if identical_content (same hash):
        skip_file
    elif is_image:
        if new_image_higher_quality:
            replace_existing_file
        elif same_quality:
            skip_file  
        else:
            copy_with_suffix (_2, _3, etc.)
    else:
        copy_with_suffix
else:
    copy_file
```

### Quality Comparison

**Image Quality Check:**
1. **Resolution priority**: Higher pixel count wins
2. **File size comparison**: For same resolution, larger file indicates better compression quality  
3. **Equal quality threshold**: Files within 10% size difference considered equal quality

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
- Edge cases and error handling ✅

## Key Quality Features

**File Integrity:**
- SHA256 hash verification for duplicate detection
- Atomic file operations prevent corruption
- Safe path generation for naming conflicts

**Smart Organization:**
- EXIF-based dating for images
- Metadata extraction for videos  
- Fallback to file modification time
- Separate handling for files without metadata

**Performance:**
- Lazy metadata extraction (only when needed)
- Native Go libraries for common formats
- ExifTool fallback for comprehensive format support

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