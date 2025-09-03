# Anduril - Smart Media File Organizer

Anduril is a Go CLI tool for organizing large media collections by capture date with intelligent duplicate handling and quality-based file replacement.

## TL;DR

**Purpose:** Smart media organizer that **COPIES** files into date-organized folders with intelligent duplicate handling

**Key Commands:**
- `anduril import /photos` - Import and organize media files  
- `anduril import /photos --workers 8 --batch-size 100` - Performance optimized
- `anduril import /photos --dry-run` - Preview changes without copying
- `anduril server` - Web interface foundation with PocketBase

**Smart Features:** 4-level date detection (EXIF → filename patterns → timestamps), quality-based deduplication, messaging app support (Signal/WhatsApp/Telegram)

**File Handling:** Atomic copying with SHA256 verification (originals preserved in source location)

**Performance:** Parallel processing, batch ExifTool calls, real-time progress reporting

## Features

- **Smart Date Detection**: Multi-level date extraction from EXIF metadata, filename patterns, and file timestamps
- **Quality-Based Deduplication**: Automatically keeps the highest quality version of duplicate images/videos
- **Messaging App Support**: Recognizes filename patterns from Signal, WhatsApp, Telegram, and Instagram
- **Atomic File Operations**: Safe copying with integrity verification
- **Comprehensive Format Support**: Images (JPEG, PNG, HEIC, TIFF, RAW) and videos (MP4, MOV, AVI, MKV)
- **Flexible Configuration**: TOML-based config with command-line overrides

## Quick Start

```bash
# Build the tool
go build -o anduril

# Import media files with automatic organization
./anduril import /path/to/your/photos --user john --library /home/photos

# Dry run to preview changes
./anduril import /path/to/photos --dry-run

# Force use of ExifTool for all files
./anduril import /path/to/photos --exiftool
```

## Installation

```bash
git clone <repository>
cd anduril
go build -o anduril
```

### Dependencies

- Go 1.24.5+
- ExifTool (optional, for advanced metadata extraction)

## Configuration

Create `~/.config/anduril/anduril.toml`:

```toml
user = "username"
library = "/home/user/photos"
videolibrary = "/home/user/videos"
image_extensions = [".jpg", ".jpeg", ".png", ".gif", ".heic", ".tiff", ".cr2", ".nef"]
video_extensions = [".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv", ".wmv", ".m4v"]
```

## Usage

### Import Command

```bash
anduril import [OPTIONS] INPUT_DIR
```

**Options:**
- `--user USER`: Override user folder name
- `--library LIBRARY`: Override image library path
- `--dry-run`: Preview changes without copying files
- `--exiftool`: Force use of ExifTool for all metadata extraction

### File Organization

Files are organized using a hierarchical date-based structure:

```
LIBRARY/
└── user/
    ├── 2024/
    │   ├── 03/
    │   │   ├── 15/
    │   │   │   ├── IMG_20240315_143022.jpg
    │   │   │   └── VID_20240315_150000.mp4
    │   │   └── 16/
    │   └── 04/
    └── noexif/
        └── 2024-03/
            └── unknown_date_file.jpg
```

**Date Confidence Levels:**
- **HIGH**: EXIF metadata with precise timestamp
- **MEDIUM**: Filename pattern parsing (Signal, WhatsApp, etc.)
- **LOW**: File creation time
- **VERY_LOW**: File modification time fallback

## Smart Duplicate Handling

### Image Quality Comparison

1. **Resolution Priority**: Higher pixel count (width × height) wins
2. **Compression Quality**: For same resolution, larger file size indicates better quality
3. **No Artificial Thresholds**: Direct comparison without arbitrary percentage limits

### Video Quality Comparison

1. **Duration Check**: Videos with >5 second duration difference are treated as different files
2. **Resolution Priority**: Higher pixel count wins
3. **Bitrate Quality**: For same resolution, larger file size indicates better compression

### Duplicate Resolution Logic

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
        else:
            copy_with_suffix (_2, _3, etc.)
    else:
        copy_with_suffix
else:
    copy_file
```

## Supported Filename Patterns

Anduril recognizes common filename patterns from various sources:

- **Generic**: `20240315_143022`, `IMG_20240315_143022`, `2024-03-15-14-30-22`
- **WhatsApp**: `IMG-20240315-WA0001.jpg`, `VID-20240315-WA0002.mp4`
- **Signal**: `signal-2024-03-15-143022.jpg`
- **Telegram**: `telegram-2024-03-15-14-30-22.jpg`
- **InShot**: `inshot-2024-03-15-143022.mp4`

## Architecture

### Core Components

- **`cmd/`**: CLI command definitions using Cobra
  - `root.go`: Base command setup
  - `import.go`: Import command implementation
- **`internal/`**: Core business logic
  - `config.go`: Configuration management with Viper
  - `copy.go`: File processing, quality comparison, and organization
  - `media.go`: EXIF/metadata extraction
  - `log.go`: Logging utilities

### Key Functions

**Date Detection** (`internal/copy.go:203`):
```go
func getBestFileDate(filePath string, cfg *Config) (time.Time, DateConfidence, error)
```

**Quality Comparison** (`internal/copy.go:254`):
```go
func compareImageQuality(newPath, existingPath string) QualityResult
func compareVideoQuality(newPath, existingPath string) QualityResult
```

**File Processing** (`internal/copy.go:669`):
```go
func ProcessFile(src string, cfg *Config, user string, dryRun bool) error
```

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal -v

# Test with real files (requires test data)
./test_anduril.sh
./test_quality_real.sh
```

## Performance Optimizations

- **Global ExifTool Instance**: Reuses single ExifTool process across all files
- **Native Go Libraries**: Uses standard library for common image formats
- **Lazy Metadata Extraction**: Only extracts metadata when needed for quality comparison
- **Optimized Regex Patterns**: Common patterns checked first for filename parsing

## Error Handling

- **Atomic Operations**: All file copies use temporary files with atomic rename
- **Hash Verification**: SHA256 verification prevents data corruption
- **Graceful Degradation**: Falls back through multiple date detection methods
- **Resource Cleanup**: Proper cleanup of ExifTool processes and file handles

## Future Roadmap

### Server Mode (Planned)
- Embedded PocketBase for user management and API
- Real-time filesystem watching with `fsnotify`
- Web interface for photo browsing and album management
- Static file serving from organized library
- RESTful API for external integrations

See `prompt-server.md` for detailed server mode specifications.

## Development

### Code Style
- Package structure: `internal/` for private packages, `cmd/` for CLI commands
- Error handling: Explicit error handling with wrapped context
- File operations: Atomic operations for data integrity
- Testing: Unit tests for all core logic, table-driven tests preferred

### Building

```bash
# Development build
go build -o anduril

# Production builds
GOOS=linux GOARCH=amd64 go build -o anduril-linux
GOOS=windows GOARCH=amd64 go build -o anduril-windows.exe
GOOS=darwin GOARCH=amd64 go build -o anduril-macos
```

### Dependencies

- **CLI**: `github.com/spf13/cobra` + `github.com/spf13/viper`
- **EXIF**: `github.com/barasher/go-exiftool` + `github.com/rwcarlsen/goexif`
- **Image**: Go standard library `image/*` packages

## License

[Add your license here]