package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "anduril/internal"
)

var (
    formatFlag      string
    mediaOnlyFlag   bool
    duplicatesFlag  bool
    maxDepthFlag    int
    includeHiddenFlag bool
)

var analyticsCmd = &cobra.Command{
    Use:   "analytics [folder]",
    Short: "Analyze folder contents with focus on media files",
    Long: `Scan and analyze folder contents to understand file types, detect projects,
and provide insights about media files. Skips common cache/build folders for performance.`,
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        folder := args[0]

        // Verify folder exists and is directory
        info, err := os.Stat(folder)
        if err != nil || !info.IsDir() {
            return fmt.Errorf("folder does not exist or is not a directory: %s", folder)
        }

        // Load config for file type detection
        conf, err := internal.LoadConfig()
        if err != nil {
            return err
        }

        // Create analytics options
        options := &internal.AnalyticsOptions{
            MaxDepth:      maxDepthFlag,
            IncludeHidden: includeHiddenFlag,
            MediaOnly:     mediaOnlyFlag,
            FindDuplicates: duplicatesFlag,
            Format:        formatFlag,
        }

        // Run analytics
        results, err := internal.AnalyzeFolder(folder, conf, options)
        if err != nil {
            return fmt.Errorf("failed to analyze folder: %w", err)
        }

        // Display results
        return internal.DisplayAnalytics(results, options)
    },
}

func init() {
    analyticsCmd.Flags().StringVar(&formatFlag, "format", "table", "Output format: table, json")
    analyticsCmd.Flags().BoolVar(&mediaOnlyFlag, "media-only", false, "Focus only on media files analysis")
    analyticsCmd.Flags().BoolVar(&duplicatesFlag, "duplicates", false, "Include duplicate detection (slower)")
    analyticsCmd.Flags().IntVar(&maxDepthFlag, "max-depth", 0, "Maximum recursion depth (0 = unlimited)")
    analyticsCmd.Flags().BoolVar(&includeHiddenFlag, "include-hidden", false, "Include hidden files and folders")

    rootCmd.AddCommand(analyticsCmd)
}