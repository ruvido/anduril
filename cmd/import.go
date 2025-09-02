package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "anduril/internal"
)

var (
    userFlag    string
    libraryFlag string
	videolibraryFlag string
    dryRunFlag  bool
	useExifTool bool
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

        // Scan media files using config
        files, err := internal.ScanMediaFiles(folder, conf)
        if err != nil {
            return err
        }

        fmt.Printf("Found %d media files\n", len(files))
        if dryRunFlag {
            fmt.Println("Dry run mode: no files will be copied")
        }

        // Process each file
        for _, f := range files {
            // if err := internal.ProcessFile(f, conf, user, library, dryRunFlag); err != nil {
            if err := internal.ProcessFile(f, conf, user, dryRunFlag); err != nil {
                fmt.Printf("Error processing %s: %v\n", f, err)
            }
        }

        return nil
    },
}

func init() {
    importCmd.Flags().StringVar(&userFlag, "user", "", "User folder under library")
    importCmd.Flags().StringVar(&libraryFlag, "library", "", "Root library folder")
    importCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Show files without copying")
    importCmd.Flags().BoolVar(&useExifTool, "exiftool", false, "Force to use exiftool binary")

    rootCmd.AddCommand(importCmd)
}
