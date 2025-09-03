package cmd

import (
    "fmt"
    "os"
    "runtime"
    "sync"
    "time"

    "github.com/spf13/cobra"
    "anduril/internal"
)

var (
    userFlag         string
    libraryFlag      string
    videolibraryFlag string
    dryRunFlag       bool
    useExifTool      bool
    workersFlag      int
    batchSizeFlag    int
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

        // Process files in parallel with optional batching
        if batchSizeFlag > 0 {
            if err := processFilesBatched(files, conf, user, dryRunFlag, workersFlag, batchSizeFlag); err != nil {
                return fmt.Errorf("failed to process files in batches: %w", err)
            }
        } else {
            if err := processFilesParallel(files, conf, user, dryRunFlag, workersFlag); err != nil {
                return fmt.Errorf("failed to process files: %w", err)
            }
        }

        return nil
    },
}

// processFilesParallel processes files concurrently with progress reporting
func processFilesParallel(files []string, conf *internal.Config, user string, dryRun bool, workers int) error {
    if workers <= 0 {
        workers = runtime.NumCPU()
    }
    
    // Create job channel and error collection
    jobs := make(chan string, len(files))
    var wg sync.WaitGroup
    var mu sync.Mutex
    var errors []error
    
    // Progress tracking
    processed := 0
    total := len(files)
    startTime := time.Now()
    
    // Start workers
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for filePath := range jobs {
                if err := internal.ProcessFile(filePath, conf, user, dryRun); err != nil {
                    mu.Lock()
                    errors = append(errors, fmt.Errorf("processing %s: %w", filePath, err))
                    mu.Unlock()
                }
                
                // Update progress
                mu.Lock()
                processed++
                if processed%50 == 0 || processed == total {
                    elapsed := time.Since(startTime)
                    rate := float64(processed) / elapsed.Seconds()
                    eta := time.Duration(float64(total-processed)/rate) * time.Second
                    fmt.Printf("Progress: %d/%d files (%.1f/s, ETA: %v)\n", 
                        processed, total, rate, eta.Round(time.Second))
                }
                mu.Unlock()
            }
        }()
    }
    
    // Send jobs
    for _, file := range files {
        jobs <- file
    }
    close(jobs)
    
    // Wait for completion
    wg.Wait()
    
    // Report final stats
    elapsed := time.Since(startTime)
    rate := float64(total) / elapsed.Seconds()
    fmt.Printf("Completed: %d files in %v (%.1f files/sec)\n", total, elapsed.Round(time.Second), rate)
    
    if len(errors) > 0 {
        fmt.Printf("Encountered %d errors during processing\n", len(errors))
        for _, err := range errors {
            fmt.Printf("Error: %v\n", err)
        }
    }
    
    return nil
}

// processFilesBatched processes files in batches with batch metadata extraction
func processFilesBatched(files []string, conf *internal.Config, user string, dryRun bool, workers int, batchSize int) error {
    if workers <= 0 {
        workers = runtime.NumCPU()
    }
    
    total := len(files)
    processed := 0
    startTime := time.Now()
    
    // Process files in batches
    for i := 0; i < len(files); i += batchSize {
        end := i + batchSize
        if end > len(files) {
            end = len(files)
        }
        
        batch := files[i:end]
        fmt.Printf("Processing batch %d-%d of %d files...\n", i+1, end, total)
        
        // For batches with ExifTool, pre-extract metadata
        if conf.UseExifTool {
            if _, err := internal.BatchExtractMetadata(batch); err != nil {
                fmt.Printf("Warning: batch metadata extraction failed: %v\n", err)
            }
        }
        
        // Process batch in parallel
        jobs := make(chan string, len(batch))
        var wg sync.WaitGroup
        var mu sync.Mutex
        var errors []error
        
        // Start workers for this batch
        for w := 0; w < workers; w++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                for filePath := range jobs {
                    if err := internal.ProcessFile(filePath, conf, user, dryRun); err != nil {
                        mu.Lock()
                        errors = append(errors, fmt.Errorf("processing %s: %w", filePath, err))
                        mu.Unlock()
                    }
                    
                    mu.Lock()
                    processed++
                    mu.Unlock()
                }
            }()
        }
        
        // Send batch jobs
        for _, file := range batch {
            jobs <- file
        }
        close(jobs)
        wg.Wait()
        
        // Progress update
        elapsed := time.Since(startTime)
        rate := float64(processed) / elapsed.Seconds()
        remaining := total - processed
        eta := time.Duration(float64(remaining)/rate) * time.Second
        
        fmt.Printf("Progress: %d/%d files (%.1f/s, ETA: %v)\n", 
            processed, total, rate, eta.Round(time.Second))
        
        if len(errors) > 0 {
            fmt.Printf("Batch errors: %d\n", len(errors))
        }
    }
    
    elapsed := time.Since(startTime)
    rate := float64(total) / elapsed.Seconds()
    fmt.Printf("Completed: %d files in %v (%.1f files/sec)\n", total, elapsed.Round(time.Second), rate)
    
    return nil
}

func init() {
    importCmd.Flags().StringVar(&userFlag, "user", "", "User folder under library")
    importCmd.Flags().StringVar(&libraryFlag, "library", "", "Root library folder")
    importCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Show files without copying")
    importCmd.Flags().BoolVar(&useExifTool, "exiftool", false, "Force to use exiftool binary")
    importCmd.Flags().IntVar(&workersFlag, "workers", 0, "Number of parallel workers (default: CPU count)")
    importCmd.Flags().IntVar(&batchSizeFlag, "batch-size", 0, "Process files in batches for better ExifTool performance (default: no batching)")

    rootCmd.AddCommand(importCmd)
}
