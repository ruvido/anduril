package internal

import (
	"errors"
	"strings"
	"testing"
)

func TestCategorizeError_DiskSpace(t *testing.T) {
	err := errors.New("write failed: no space left on device")
	procErr := CategorizeError("/test/file.jpg", err)

	if procErr.Category != ErrorCategoryIO {
		t.Errorf("Expected IO category, got %s", procErr.Category)
	}
	if procErr.Severity != ErrorSeverityCritical {
		t.Errorf("Expected critical severity, got %s", procErr.Severity)
	}
	if !strings.Contains(procErr.Suggestion, "disk space") {
		t.Errorf("Expected disk space suggestion, got: %s", procErr.Suggestion)
	}
}

func TestCategorizeError_Permission(t *testing.T) {
	err := errors.New("open /library/file.jpg: permission denied")
	procErr := CategorizeError("/test/file.jpg", err)

	if procErr.Category != ErrorCategoryIO {
		t.Errorf("Expected IO category, got %s", procErr.Category)
	}
	if procErr.Severity != ErrorSeverityCritical {
		t.Errorf("Expected critical severity, got %s", procErr.Severity)
	}
}

func TestCategorizeError_HashMismatch(t *testing.T) {
	err := errors.New("hash verification failed after copy")
	procErr := CategorizeError("/test/file.jpg", err)

	if procErr.Category != ErrorCategoryHash {
		t.Errorf("Expected hash category, got %s", procErr.Category)
	}
	if procErr.Severity != ErrorSeverityError {
		t.Errorf("Expected error severity, got %s", procErr.Severity)
	}
}

func TestCategorizeError_Metadata(t *testing.T) {
	err := errors.New("failed to read exif data")
	procErr := CategorizeError("/test/file.jpg", err)

	if procErr.Category != ErrorCategoryMetadata {
		t.Errorf("Expected metadata category, got %s", procErr.Category)
	}
	if procErr.Severity != ErrorSeverityWarning {
		t.Errorf("Expected warning severity, got %s", procErr.Severity)
	}
}

func TestErrorStats_ShouldAbort_Critical(t *testing.T) {
	stats := NewErrorStats()

	// Add a critical error
	criticalErr := &ProcessError{
		FilePath: "/test/file.jpg",
		Category: ErrorCategoryIO,
		Severity: ErrorSeverityCritical,
	}
	stats.Add(criticalErr)

	shouldAbort, reason := stats.ShouldAbort()
	if !shouldAbort {
		t.Error("Expected abort on critical error")
	}
	if !strings.Contains(reason, "Critical") {
		t.Errorf("Expected 'Critical' in reason, got: %s", reason)
	}
}

func TestErrorStats_ShouldAbort_ConsecutiveErrors(t *testing.T) {
	stats := NewErrorStats()

	// Add 10 consecutive errors
	for i := 0; i < 10; i++ {
		err := &ProcessError{
			FilePath: "/test/file.jpg",
			Category: ErrorCategoryIO,
			Severity: ErrorSeverityError,
		}
		stats.Add(err)
		stats.Consecutive++
	}

	shouldAbort, reason := stats.ShouldAbort()
	if !shouldAbort {
		t.Error("Expected abort after 10 consecutive errors")
	}
	if !strings.Contains(reason, "10 consecutive") {
		t.Errorf("Expected '10 consecutive' in reason, got: %s", reason)
	}
}

func TestErrorStats_ResetConsecutive(t *testing.T) {
	stats := NewErrorStats()

	// Add some consecutive errors
	for i := 0; i < 5; i++ {
		err := &ProcessError{
			FilePath: "/test/file.jpg",
			Category: ErrorCategoryIO,
			Severity: ErrorSeverityError,
		}
		stats.Add(err)
		stats.Consecutive++
	}

	if stats.Consecutive != 5 {
		t.Errorf("Expected 5 consecutive errors, got %d", stats.Consecutive)
	}

	// Reset
	stats.ResetConsecutive()

	if stats.Consecutive != 0 {
		t.Errorf("Expected 0 consecutive errors after reset, got %d", stats.Consecutive)
	}
}

func TestErrorStats_GenerateReport(t *testing.T) {
	stats := NewErrorStats()

	// Add various errors
	stats.Add(&ProcessError{
		FilePath:    "/test/file1.jpg",
		Category:    ErrorCategoryIO,
		Severity:    ErrorSeverityError,
		OriginalErr: errors.New("I/O error"),
		Suggestion:  "Check disk health",
	})

	stats.Add(&ProcessError{
		FilePath:    "/test/file2.jpg",
		Category:    ErrorCategoryHash,
		Severity:    ErrorSeverityError,
		OriginalErr: errors.New("hash mismatch"),
		Suggestion:  "File corrupted",
	})

	report := stats.GenerateReport()

	// Verify report contains expected sections
	if !strings.Contains(report, "Import encountered") {
		t.Error("Report missing main header")
	}
	if !strings.Contains(report, "Error categories") {
		t.Error("Report missing categories section")
	}
	if !strings.Contains(report, "Recent errors") {
		t.Error("Report missing recent errors section")
	}
	if !strings.Contains(report, "Suggested next steps") {
		t.Error("Report missing suggestions section")
	}

	// Verify it shows the errors
	if !strings.Contains(report, "file1.jpg") {
		t.Error("Report missing first error")
	}
	if !strings.Contains(report, "Check disk health") {
		t.Error("Report missing suggestion")
	}
}

func TestErrorStats_ByCategory(t *testing.T) {
	stats := NewErrorStats()

	// Add different categories
	stats.Add(&ProcessError{Category: ErrorCategoryIO, Severity: ErrorSeverityError, OriginalErr: errors.New("test")})
	stats.Add(&ProcessError{Category: ErrorCategoryIO, Severity: ErrorSeverityError, OriginalErr: errors.New("test")})
	stats.Add(&ProcessError{Category: ErrorCategoryHash, Severity: ErrorSeverityError, OriginalErr: errors.New("test")})

	if stats.ByCategory[ErrorCategoryIO] != 2 {
		t.Errorf("Expected 2 IO errors, got %d", stats.ByCategory[ErrorCategoryIO])
	}
	if stats.ByCategory[ErrorCategoryHash] != 1 {
		t.Errorf("Expected 1 hash error, got %d", stats.ByCategory[ErrorCategoryHash])
	}
}
