package internal

import (
	"fmt"
	"strings"
)

// ErrorCategory represents the type of error encountered
type ErrorCategory string

const (
	ErrorCategoryIO          ErrorCategory = "io_error"           // File system, permissions, disk space
	ErrorCategoryHash        ErrorCategory = "hash_mismatch"      // Corruption during copy
	ErrorCategoryMetadata    ErrorCategory = "metadata_error"     // EXIF/metadata extraction failed
	ErrorCategoryUnsupported ErrorCategory = "unsupported_format" // Unrecognized file format
	ErrorCategoryUnknown     ErrorCategory = "unknown_error"      // Unexpected errors
)

// ErrorSeverity indicates how critical the error is
type ErrorSeverity string

const (
	ErrorSeverityCritical ErrorSeverity = "critical" // System-level issues (disk full, permissions)
	ErrorSeverityError    ErrorSeverity = "error"    // File-level issues (corruption, unreadable)
	ErrorSeverityWarning  ErrorSeverity = "warning"  // Recoverable issues (low confidence date)
)

// ProcessError represents a categorized error during file processing
type ProcessError struct {
	FilePath     string
	Category     ErrorCategory
	Severity     ErrorSeverity
	OriginalErr  error
	Context      map[string]string // Additional context (hash, destination, etc.)
	Suggestion   string            // User-friendly suggestion to fix
}

func (e *ProcessError) Error() string {
	return fmt.Sprintf("[%s/%s] %s: %v", e.Severity, e.Category, e.FilePath, e.OriginalErr)
}

// CategorizeError analyzes an error and returns a ProcessError with category and severity
func CategorizeError(filePath string, err error) *ProcessError {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())
	procErr := &ProcessError{
		FilePath:    filePath,
		OriginalErr: err,
		Context:     make(map[string]string),
	}

	// Categorize based on error message
	switch {
	// Disk/Filesystem errors (CRITICAL)
	case strings.Contains(errStr, "no space left"):
		procErr.Category = ErrorCategoryIO
		procErr.Severity = ErrorSeverityCritical
		procErr.Suggestion = "Free up disk space on the destination drive and retry the import"

	case strings.Contains(errStr, "permission denied"):
		procErr.Category = ErrorCategoryIO
		procErr.Severity = ErrorSeverityCritical
		procErr.Suggestion = "Check file permissions on both source and destination directories"

	case strings.Contains(errStr, "read-only file system"):
		procErr.Category = ErrorCategoryIO
		procErr.Severity = ErrorSeverityCritical
		procErr.Suggestion = "Destination filesystem is read-only - check mount options"

	case strings.Contains(errStr, "too many open files"):
		procErr.Category = ErrorCategoryIO
		procErr.Severity = ErrorSeverityCritical
		procErr.Suggestion = "System file descriptor limit reached - increase ulimit or restart"

	// Hash/Corruption errors (ERROR)
	case strings.Contains(errStr, "hash verification failed"):
		procErr.Category = ErrorCategoryHash
		procErr.Severity = ErrorSeverityError
		procErr.Suggestion = "File may be corrupted - verify source file integrity or try re-importing"

	case strings.Contains(errStr, "hash mismatch"):
		procErr.Category = ErrorCategoryHash
		procErr.Severity = ErrorSeverityError
		procErr.Suggestion = "Data corruption detected during copy - check disk health"

	// I/O errors (ERROR)
	case strings.Contains(errStr, "input/output error"):
		procErr.Category = ErrorCategoryIO
		procErr.Severity = ErrorSeverityError
		procErr.Suggestion = "I/O error - check disk health with SMART tools"

	case strings.Contains(errStr, "no such file"):
		procErr.Category = ErrorCategoryIO
		procErr.Severity = ErrorSeverityError
		procErr.Suggestion = "Source file disappeared during import - check if external drive disconnected"

	// Metadata errors (WARNING - file can still be copied)
	case strings.Contains(errStr, "exif") || strings.Contains(errStr, "metadata"):
		procErr.Category = ErrorCategoryMetadata
		procErr.Severity = ErrorSeverityWarning
		procErr.Suggestion = "File will be copied to noexif folder - metadata could not be extracted"

	// Unsupported format
	case strings.Contains(errStr, "unsupported") || strings.Contains(errStr, "unknown format"):
		procErr.Category = ErrorCategoryUnsupported
		procErr.Severity = ErrorSeverityWarning
		procErr.Suggestion = "File format not recognized - will be skipped"

	// Default: unknown error
	default:
		procErr.Category = ErrorCategoryUnknown
		procErr.Severity = ErrorSeverityError
		procErr.Suggestion = "Unexpected error - check logs for details"
	}

	return procErr
}

// ErrorStats tracks error statistics during import
type ErrorStats struct {
	Total        int
	Critical     int
	Errors       int
	Warnings     int
	ByCategory   map[ErrorCategory]int
	LastErrors   []*ProcessError // Last 5 errors for quick diagnosis
	Consecutive  int             // Consecutive errors (for circuit breaker)
}

func NewErrorStats() *ErrorStats {
	return &ErrorStats{
		ByCategory: make(map[ErrorCategory]int),
		LastErrors: make([]*ProcessError, 0, 5),
	}
}

func (s *ErrorStats) Add(err *ProcessError) {
	s.Total++
	s.ByCategory[err.Category]++

	switch err.Severity {
	case ErrorSeverityCritical:
		s.Critical++
	case ErrorSeverityError:
		s.Errors++
	case ErrorSeverityWarning:
		s.Warnings++
	}

	// Keep last 5 errors
	if len(s.LastErrors) >= 5 {
		s.LastErrors = s.LastErrors[1:]
	}
	s.LastErrors = append(s.LastErrors, err)
}

func (s *ErrorStats) ResetConsecutive() {
	s.Consecutive = 0
}

// ShouldAbort returns true if import should be aborted based on error patterns
func (s *ErrorStats) ShouldAbort() (bool, string) {
	// Critical errors: abort immediately
	if s.Critical > 0 {
		return true, "Critical system error detected - aborting to prevent data loss"
	}

	// 10 consecutive errors: likely systemic issue
	if s.Consecutive >= 10 {
		return true, "10 consecutive errors detected - likely systemic issue (disk full, permissions, etc.)"
	}

	// More than 50% error rate (with minimum 20 processed files)
	// This is checked externally based on total processed files

	return false, ""
}

// GenerateReport creates a human-readable error report
func (s *ErrorStats) GenerateReport() string {
	var report strings.Builder

	report.WriteString(fmt.Sprintf("\n❌ Import encountered %d errors:\n\n", s.Total))

	// Breakdown by severity
	if s.Critical > 0 {
		report.WriteString(fmt.Sprintf("  🔴 Critical: %d (system-level issues)\n", s.Critical))
	}
	if s.Errors > 0 {
		report.WriteString(fmt.Sprintf("  🟠 Errors:   %d (file-level issues)\n", s.Errors))
	}
	if s.Warnings > 0 {
		report.WriteString(fmt.Sprintf("  🟡 Warnings: %d (recoverable issues)\n", s.Warnings))
	}

	report.WriteString("\n")

	// Breakdown by category
	report.WriteString("Error categories:\n")
	for cat, count := range s.ByCategory {
		report.WriteString(fmt.Sprintf("  • %s: %d\n", cat, count))
	}

	report.WriteString("\n")

	// Last few errors with suggestions
	report.WriteString("Recent errors:\n")
	for i, err := range s.LastErrors {
		report.WriteString(fmt.Sprintf("\n%d. %s\n", i+1, err.FilePath))
		report.WriteString(fmt.Sprintf("   Category: %s | Severity: %s\n", err.Category, err.Severity))
		report.WriteString(fmt.Sprintf("   Error: %v\n", err.OriginalErr))
		if err.Suggestion != "" {
			report.WriteString(fmt.Sprintf("   💡 Suggestion: %s\n", err.Suggestion))
		}
	}

	// General suggestions based on error patterns
	report.WriteString("\n")
	report.WriteString(s.generateSuggestions())

	return report.String()
}

func (s *ErrorStats) generateSuggestions() string {
	var suggestions strings.Builder
	suggestions.WriteString("Suggested next steps:\n")

	// IO errors
	if s.ByCategory[ErrorCategoryIO] > 0 {
		suggestions.WriteString("  • Check disk space and permissions\n")
		suggestions.WriteString("  • Verify source media (SD card, external drive) is properly connected\n")
	}

	// Hash errors
	if s.ByCategory[ErrorCategoryHash] > 0 {
		suggestions.WriteString("  • Run disk health check (SMART diagnostics)\n")
		suggestions.WriteString("  • Verify source files are not corrupted\n")
	}

	// Metadata errors
	if s.ByCategory[ErrorCategoryMetadata] > s.Total/2 {
		suggestions.WriteString("  • Many metadata errors - consider using --exiftool flag for better compatibility\n")
	}

	// High consecutive errors
	if s.Consecutive >= 5 {
		suggestions.WriteString("  • Multiple consecutive errors suggest systemic issue - check system resources\n")
	}

	suggestions.WriteString("  • Check session manifest for detailed error log\n")

	return suggestions.String()
}
