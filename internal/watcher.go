package internal

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// EventType represents the type of filesystem event
type EventType int

const (
	EventCreate EventType = iota
	EventDelete
	EventRename
)

// WatchEvent represents a filesystem event we care about
type WatchEvent struct {
	Type    EventType
	Path    string
	OldPath string // For rename events
}

// Watcher wraps fsnotify watcher with media file filtering
type Watcher struct {
	watcher *fsnotify.Watcher
	events  chan *WatchEvent
	errors  chan error
	done    chan bool
}

// NewWatcher creates a new filesystem watcher for the given directories
func NewWatcher(photosDir, videosDir string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher: fsWatcher,
		events:  make(chan *WatchEvent, 100),
		errors:  make(chan error, 10),
		done:    make(chan bool, 1),
	}

	// Add directories to watch recursively
	if err := w.addRecursive(photosDir); err != nil {
		fsWatcher.Close()
		return nil, err
	}

	if videosDir != photosDir {
		if err := w.addRecursive(videosDir); err != nil {
			fsWatcher.Close()
			return nil, err
		}
	}

	// Start processing events in background
	go w.processEvents()

	return w, nil
}

// addRecursive adds a directory and all its subdirectories to the watcher
func (w *Watcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return w.watcher.Add(path)
		}
		return nil
	})
}

// processEvents processes raw fsnotify events and filters/converts them
func (w *Watcher) processEvents() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only process media files
			if !isMediaFile(event.Name) {
				continue
			}

			watchEvent := &WatchEvent{
				Path: event.Name,
			}

			// Convert fsnotify events to our event types
			if event.Op&fsnotify.Create == fsnotify.Create {
				watchEvent.Type = EventCreate
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				watchEvent.Type = EventDelete
			} else if event.Op&fsnotify.Rename == fsnotify.Rename {
				watchEvent.Type = EventRename
				// Note: fsnotify doesn't provide old path for renames
				// This is a limitation we'd need to work around
			} else {
				continue // Skip other event types
			}

			select {
			case w.events <- watchEvent:
			default:
				// Event channel is full, drop event
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			default:
				// Error channel is full, drop error
			}

		case <-w.done:
			return
		}
	}
}

// Events returns the channel of filtered watch events
func (w *Watcher) Events() <-chan *WatchEvent {
	return w.events
}

// Errors returns the channel of watcher errors
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// Close stops the watcher and cleans up resources
func (w *Watcher) Close() error {
	close(w.done)
	return w.watcher.Close()
}

// isMediaFile checks if a file path represents a media file
func isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// Common image extensions
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".heic", ".tiff", ".cr2", ".nef"}
	for _, e := range imageExts {
		if ext == e {
			return true
		}
	}

	// Common video extensions
	videoExts := []string{".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv", ".wmv", ".m4v"}
	for _, e := range videoExts {
		if ext == e {
			return true
		}
	}

	return false
}
