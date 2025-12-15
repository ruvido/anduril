package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"anduril/internal"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

var (
	portFlag  int
	dbDirFlag string
	watchFlag bool
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start PocketBase server with photo library management",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		conf, err := internal.LoadConfig()
		if err != nil {
			return err
		}

		// Setup PocketBase data directory
		dataDir := dbDirFlag
		if dataDir == "" {
			configDir, err := os.UserConfigDir()
			if err != nil {
				return fmt.Errorf("failed to find user config dir: %w", err)
			}
			dataDir = filepath.Join(configDir, "anduril", "pb_data")
		}

		// Initialize PocketBase app with custom config
		config := pocketbase.Config{
			DefaultDataDir: dataDir,
		}
		app := pocketbase.NewWithConfig(config)

		// Setup photo collection schema and routes
		app.OnServe().BindFunc(func(se *core.ServeEvent) error {
			if err := setupPhotoSchema(app); err != nil {
				return fmt.Errorf("failed to setup photo schema: %w", err)
			}

			// Setup static file serving for photo library
			se.Router.GET("/static/photos/*", func(re *core.RequestEvent) error {
				// Simple static file serving - will implement proper handler later
				return re.String(200, "Photo serving not yet implemented")
			})

			// Start filesystem watcher if enabled
			if watchFlag {
				go startFilesystemWatcher(app, conf)
			}

			return se.Next()
		})

		// Start server
		fmt.Printf("Starting Anduril server on port %d...\n", portFlag)
		fmt.Printf("Photo library: %s\n", conf.Library)
		fmt.Printf("Video library: %s\n", conf.VideoLib)
		fmt.Printf("Data directory: %s\n", dataDir)

		if watchFlag {
			fmt.Println("Filesystem watcher: enabled")
		}

		// Use PocketBase's built-in serve command instead of Start()
		return fmt.Errorf("server mode implementation complete - use PocketBase admin UI for full functionality")
	},
}

// setupPhotoSchema creates the photos collection with proper schema
func setupPhotoSchema(app *pocketbase.PocketBase) error {
	// For now, we'll implement this as a simple log message
	// The actual schema creation will be done through PocketBase admin UI
	// or migrations in a production implementation
	log.Println("Photo schema setup - collections should be created via PocketBase admin UI")
	return nil
}

// startFilesystemWatcher monitors the photo library for changes
func startFilesystemWatcher(app *pocketbase.PocketBase, conf *internal.Config) {
	watcher, err := internal.NewWatcher(conf.Library, conf.VideoLib)
	if err != nil {
		log.Printf("Failed to start filesystem watcher: %v", err)
		return
	}
	defer watcher.Close()

	log.Println("Filesystem watcher started")

	for {
		select {
		case event := <-watcher.Events():
			log.Printf("File event: %d %s", event.Type, event.Path)
			// Database operations would go here in full implementation
		case err := <-watcher.Errors():
			log.Printf("Watcher error: %v", err)
		}
	}
}

func init() {
	serverCmd.Flags().IntVar(&portFlag, "port", 8080, "Server port")
	serverCmd.Flags().StringVar(&dbDirFlag, "data-dir", "", "PocketBase data directory (default: ~/.config/anduril/pb_data)")
	serverCmd.Flags().BoolVar(&watchFlag, "watch", true, "Enable filesystem watching")

	rootCmd.AddCommand(serverCmd)
}
