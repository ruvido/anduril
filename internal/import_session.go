package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImportSession manages an import session with manifest logging and hardlink browser
type ImportSession struct {
	ID            string              // Session ID (timestamp: 2025-01-15-103045)
	LibraryPath   string              // Library root path
	SessionDir    string              // Full path to session directory
	ManifestFile  *os.File            // Open file handle for manifest.jsonl
	InputDir      string              // Original input directory
	User          string              // User name
	usedFilenames map[string]int      // Track filename usage for collision detection
	stats         ImportStats         // Session statistics
}

// ImportStats tracks statistics for an import session
type ImportStats struct {
	TotalScanned      int
	Copied            int
	SkippedDuplicate  int
	CopiedTimestamped int
	Errors            int
}

// ManifestEvent represents a single event in the manifest log
type ManifestEvent struct {
	Event    string `json:"event"`
	Ts       string `json:"ts"`
	Src      string `json:"src,omitempty"`
	Dest     string `json:"dest,omitempty"`
	Hash     string `json:"hash,omitempty"`
	Browse   string `json:"browse,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Existing string `json:"existing,omitempty"`
	Error    string `json:"error,omitempty"`

	// Error details (for categorized errors)
	ErrorCategory  string `json:"error_category,omitempty"`
	ErrorSeverity  string `json:"error_severity,omitempty"`
	ErrorSuggestion string `json:"error_suggestion,omitempty"`

	// Session start/end fields
	User              string `json:"user,omitempty"`
	InputDir          string `json:"input_dir,omitempty"`
	TotalFiles        int    `json:"total_files,omitempty"`
	TotalScanned      int    `json:"total_scanned,omitempty"`
	Copied            int    `json:"copied,omitempty"`
	SkippedDuplicate  int    `json:"skipped_duplicate,omitempty"`
	CopiedTimestamped int    `json:"copied_timestamped,omitempty"`
	ErrorCount        int    `json:"errors,omitempty"`
}

// NewImportSession creates a new import session
func NewImportSession(libraryPath, user, inputDir string) (*ImportSession, error) {
	// Generate session ID from current timestamp
	sessionID := time.Now().Format("2006-01-02-150405")

	// Create session directory
	importsDir := filepath.Join(libraryPath, "imports")
	sessionDir := filepath.Join(importsDir, sessionID)

	// Create imports directory if it doesn't exist
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Open manifest file for append-only writes
	manifestPath := filepath.Join(sessionDir, "manifest.jsonl")
	manifestFile, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest file: %w", err)
	}

	session := &ImportSession{
		ID:            sessionID,
		LibraryPath:   libraryPath,
		SessionDir:    sessionDir,
		ManifestFile:  manifestFile,
		InputDir:      inputDir,
		User:          user,
		usedFilenames: make(map[string]int),
		stats:         ImportStats{},
	}

	return session, nil
}

// LogSessionStart writes the session start event to manifest
func (s *ImportSession) LogSessionStart(totalFiles int) error {
	event := ManifestEvent{
		Event:      "session_start",
		Ts:         time.Now().UTC().Format(time.RFC3339),
		User:       s.User,
		InputDir:   s.InputDir,
		TotalFiles: totalFiles,
	}

	return s.writeEvent(event)
}

// LogCopied logs a successful file copy
func (s *ImportSession) LogCopied(src, dest, hash string, size int64, browsePath string) error {
	s.stats.Copied++

	event := ManifestEvent{
		Event:  "copied",
		Ts:     time.Now().UTC().Format(time.RFC3339),
		Src:    src,
		Dest:   dest,
		Hash:   hash,
		Browse: browsePath,
		Size:   size,
	}

	return s.writeEvent(event)
}

// LogCopiedTimestamped logs a file copied with timestamp suffix
func (s *ImportSession) LogCopiedTimestamped(src, dest, hash string, size int64, browsePath string) error {
	s.stats.CopiedTimestamped++

	event := ManifestEvent{
		Event:  "copied_timestamped",
		Ts:     time.Now().UTC().Format(time.RFC3339),
		Src:    src,
		Dest:   dest,
		Hash:   hash,
		Browse: browsePath,
		Size:   size,
	}

	return s.writeEvent(event)
}

// LogSkippedDuplicate logs a skipped duplicate file
func (s *ImportSession) LogSkippedDuplicate(src, existing, hash string) error {
	s.stats.SkippedDuplicate++

	event := ManifestEvent{
		Event:    "skipped_duplicate",
		Ts:       time.Now().UTC().Format(time.RFC3339),
		Src:      src,
		Existing: existing,
		Hash:     hash,
	}

	return s.writeEvent(event)
}

// LogError logs an error during file processing (legacy - use LogDetailedError for categorized errors)
func (s *ImportSession) LogError(src string, err error) error {
	s.stats.Errors++

	event := ManifestEvent{
		Event: "error",
		Ts:    time.Now().UTC().Format(time.RFC3339),
		Src:   src,
		Error: err.Error(),
	}

	return s.writeEvent(event)
}

// LogDetailedError logs a categorized error with full details
func (s *ImportSession) LogDetailedError(src string, procErr *ProcessError) error {
	s.stats.Errors++

	event := ManifestEvent{
		Event:           "error",
		Ts:              time.Now().UTC().Format(time.RFC3339),
		Src:             src,
		Error:           procErr.OriginalErr.Error(),
		ErrorCategory:   string(procErr.Category),
		ErrorSeverity:   string(procErr.Severity),
		ErrorSuggestion: procErr.Suggestion,
	}

	// Add context if available
	if dest, ok := procErr.Context["dest"]; ok {
		event.Dest = dest
	}
	if hash, ok := procErr.Context["hash"]; ok {
		event.Hash = hash
	}

	return s.writeEvent(event)
}

// LogSessionEnd writes the session end event to manifest
func (s *ImportSession) LogSessionEnd(stats ImportStats) error {
	event := ManifestEvent{
		Event:             "session_end",
		Ts:                time.Now().UTC().Format(time.RFC3339),
		TotalScanned:      stats.TotalScanned,
		Copied:            stats.Copied,
		SkippedDuplicate:  stats.SkippedDuplicate,
		CopiedTimestamped: stats.CopiedTimestamped,
		ErrorCount:        stats.Errors,
	}

	return s.writeEvent(event)
}

// CreateHardlink creates a hardlink in the session directory for browsing
// Returns the basename used in the session directory (with collision suffix if needed)
func (s *ImportSession) CreateHardlink(libraryFilePath string) (string, error) {
	basename := filepath.Base(libraryFilePath)

	// Check for collision
	count, exists := s.usedFilenames[basename]
	finalBasename := basename

	if exists {
		// Collision! Use suffix
		ext := filepath.Ext(basename)
		nameNoExt := strings.TrimSuffix(basename, ext)
		finalBasename = fmt.Sprintf("%s_%d%s", nameNoExt, count+1, ext)
	}

	// Update usage count
	s.usedFilenames[basename] = count + 1

	// Create hardlink
	browsePath := filepath.Join(s.SessionDir, finalBasename)
	if err := os.Link(libraryFilePath, browsePath); err != nil {
		return "", fmt.Errorf("hardlink failed: %w", err)
	}

	return finalBasename, nil
}

// GetStats returns the current session statistics
func (s *ImportSession) GetStats() ImportStats {
	return s.stats
}

// Close closes the manifest file and session
func (s *ImportSession) Close() error {
	if s.ManifestFile != nil {
		return s.ManifestFile.Close()
	}
	return nil
}

// writeEvent writes a manifest event as a JSON line
func (s *ImportSession) writeEvent(event ManifestEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Write JSON line with newline
	if _, err := s.ManifestFile.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to manifest: %w", err)
	}

	// Flush to ensure data is written
	return s.ManifestFile.Sync()
}
