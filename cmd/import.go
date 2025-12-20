package cmd

import (
	"fmt"
	"os"
	"time"

	"anduril/internal"
	"github.com/spf13/cobra"
)

var (
	userFlag         string
	libraryFlag      string
	videolibraryFlag string
	dryRunFlag       bool
	useExifTool      bool
	useHardlinks     bool
)

var importCmd = &cobra.Command{
	Use:   "import [folder]",
	Short: "Import media files from folder",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		folder := args[0]

		info, err := os.Stat(folder)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("folder does not exist or is not a directory: %s", folder)
		}

		// Load config
		conf, err := internal.LoadConfig()
		if err != nil {
			return err
		}

		// Override config with command line flags
		if useExifTool {
			conf.UseExifTool = true
		}
		if useHardlinks {
			conf.UseHardlinks = true
		}

		// Determine user and library
		user := userFlag
		library := libraryFlag
		videolibrary := videolibraryFlag
		if user == "" {
			user = conf.User
		}
		if library == "" {
			library = conf.Library
		}
		if videolibrary == "" {
			videolibrary = conf.VideoLib
		}

		if user == "" || library == "" {
			return fmt.Errorf("missing --user or --library and no defaults set")
		}

		// Update config with resolved values so all code paths use the correct paths
		conf.Library = library
		conf.VideoLib = videolibrary

		// Show configuration being used
		fmt.Println("Configuration:")
		fmt.Printf("  User: %s\n", user)
		fmt.Printf("  Library: %s\n", library)
		fmt.Printf("  Video Library: %s\n", videolibrary)
		fmt.Printf("  ExifTool: %v\n", conf.UseExifTool)
		fmt.Printf("  Hardlinks: %v\n", conf.UseHardlinks)
		fmt.Println()

		logger, err := internal.NewLogger("anduril.log")
		if err != nil {
			return err
		}
		defer logger.Close()
		defer internal.CloseExifTool() // Ensure ExifTool cleanup

		// Scan media files using config
		files, err := internal.ScanMediaFiles(folder, conf)
		if err != nil {
			return err
		}

		fmt.Printf("Found %d media files\n", len(files))
		if dryRunFlag {
			fmt.Println("Dry run mode: no files will be copied")
		}

		// Test hardlink support before starting (if --link is used)
		if conf.UseHardlinks {
			fmt.Println("Testing hardlink support...")
			// Test against image library
			if err := internal.TestHardlinkSupport(folder, library); err != nil {
				return err
			}
			// Test against video library if different
			if videolibrary != "" && videolibrary != library {
				if err := internal.TestHardlinkSupport(folder, videolibrary); err != nil {
					return err
				}
			}
			fmt.Println("Hardlink support: OK")
		}

		// Process files sequentially with progress reporting
		if err := processFiles(files, conf, user, folder, dryRunFlag); err != nil {
			return fmt.Errorf("failed to process files: %w", err)
		}

		return nil
	},
}

// processFiles processes files sequentially with progress reporting
func processFiles(files []string, conf *internal.Config, user, inputDir string, dryRun bool) error {
	total := len(files)
	startTime := time.Now()
	errorStats := internal.NewErrorStats()
	successCount := 0

	// Create import session (unless dry-run)
	var session *internal.ImportSession
	if !dryRun {
		var err error
		session, err = internal.NewImportSession(conf.Library, conf.VideoLib, user, inputDir)
		if err != nil {
			return fmt.Errorf("failed to create import session: %w", err)
		}
		defer session.Close()

		// Log session start
		if err := session.LogSessionStart(total); err != nil {
			return fmt.Errorf("failed to log session start: %w", err)
		}

		fmt.Printf("Import session: %s\n", session.ID)
		fmt.Printf("Browse imported files: %s\n\n", session.SessionDir)
	}

	for i, filePath := range files {
		if err := internal.ProcessFile(filePath, conf, user, dryRun, session); err != nil {
			// Categorize the error
			procErr := internal.CategorizeError(filePath, err)
			errorStats.Add(procErr)
			errorStats.Consecutive++

			// Log detailed error to session
			if session != nil {
				session.LogDetailedError(filePath, procErr)
			}

			// Check if we should abort
			if shouldAbort, reason := errorStats.ShouldAbort(); shouldAbort {
				fmt.Printf("\n⚠️  ABORTING IMPORT: %s\n", reason)
				fmt.Printf("Processed: %d/%d files before abort\n", i+1, total)
				return fmt.Errorf("import aborted: %s", reason)
			}

			// Check error rate threshold (50% errors with at least 20 files processed)
			processed := i + 1
			if processed >= 20 && errorStats.Total > processed/2 {
				fmt.Printf("\n⚠️  ABORTING IMPORT: Error rate too high (%d/%d = %.1f%%)\n",
					errorStats.Total, processed, float64(errorStats.Total)/float64(processed)*100)
				fmt.Printf("This suggests a systemic problem - check system resources and permissions\n")
				return fmt.Errorf("import aborted: error rate exceeds 50%%")
			}
		} else {
			// Success - reset consecutive error counter
			successCount++
			errorStats.ResetConsecutive()
		}

		// Update progress every 10 files or at the end
		processed := i + 1
		if processed%10 == 0 || processed == total {
			elapsed := time.Since(startTime)
			rate := float64(processed) / elapsed.Seconds()
			remaining := total - processed
			var eta time.Duration
			if rate > 0 {
				eta = time.Duration(float64(remaining)/rate) * time.Second
			}

			// Show error count in progress if errors occurred
			if errorStats.Total > 0 {
				fmt.Printf("Progress: %d/%d files (%.1f/s, ETA: %v) | Errors: %d\n",
					processed, total, rate, eta.Round(time.Second), errorStats.Total)
			} else {
				fmt.Printf("Progress: %d/%d files (%.1f/s, ETA: %v)\n",
					processed, total, rate, eta.Round(time.Second))
			}
		}
	}

	// Log session end
	if session != nil {
		stats := session.GetStats()
		stats.TotalScanned = total
		if err := session.LogSessionEnd(stats); err != nil {
			fmt.Printf("Warning: failed to log session end: %v\n", err)
		}
	}

	// Report final stats
	elapsed := time.Since(startTime)
	rate := float64(total) / elapsed.Seconds()
	fmt.Printf("\n✅ Completed: %d files in %v (%.1f files/sec)\n", total, elapsed.Round(time.Second), rate)

	if session != nil {
		stats := session.GetStats()
		fmt.Printf("\nImport Summary:\n")
		fmt.Printf("  ✓ Copied:            %d files\n", stats.Copied)
		if stats.CopiedTimestamped > 0 {
			fmt.Printf("  ✓ Timestamped:       %d files\n", stats.CopiedTimestamped)
		}
		if stats.SkippedDuplicate > 0 {
			fmt.Printf("  ⊘ Skipped (duplicates): %d files\n", stats.SkippedDuplicate)
		}
		if errorStats.Total > 0 {
			fmt.Printf("  ✗ Errors:            %d files\n", errorStats.Total)
		}
		fmt.Printf("\n📁 Browse session: %s\n", session.SessionDir)
	}

	// Show detailed error report if errors occurred
	if errorStats.Total > 0 {
		fmt.Print(errorStats.GenerateReport())
		return fmt.Errorf("import completed with %d errors (%.1f%% success rate)",
			errorStats.Total, float64(successCount)/float64(total)*100)
	}

	return nil
}

func init() {
	importCmd.Flags().StringVar(&userFlag, "user", "", "User folder under library")
	importCmd.Flags().StringVar(&libraryFlag, "library", "", "Root library folder")
	importCmd.Flags().StringVar(&videolibraryFlag, "videolibrary", "", "Video library folder")
	importCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Show files without copying")
	importCmd.Flags().BoolVar(&useExifTool, "exiftool", false, "Force to use exiftool binary")
	importCmd.Flags().BoolVar(&useHardlinks, "link", false, "Use hardlinks instead of copying (instant, no extra space)")

	rootCmd.AddCommand(importCmd)
}
