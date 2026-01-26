// Package daemon provides the Forge daemon server for hot reload and file watching.
package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEventType represents the type of file system event
type FileEventType int

const (
	FileEventCreated FileEventType = iota + 1
	FileEventModified
	FileEventDeleted
	FileEventRenamed
)

func (t FileEventType) String() string {
	switch t {
	case FileEventCreated:
		return "created"
	case FileEventModified:
		return "modified"
	case FileEventDeleted:
		return "deleted"
	case FileEventRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Type      FileEventType
	Timestamp time.Time
}

// WatcherConfig contains configuration for the file watcher
type WatcherConfig struct {
	// ProjectDir is the root directory to watch
	ProjectDir string

	// Patterns are glob patterns to match (e.g., "*.go", "forge.json")
	Patterns []string

	// IgnorePatterns are patterns to ignore (e.g., ".git", "node_modules")
	IgnorePatterns []string

	// Debounce is the debounce duration for rapid events
	Debounce time.Duration
}

// DefaultWatcherConfig returns default watcher configuration
func DefaultWatcherConfig(projectDir string) *WatcherConfig {
	return &WatcherConfig{
		ProjectDir: projectDir,
		Patterns:   []string{"*.go", "forge.json", "*.proto"},
		IgnorePatterns: []string{
			".git",
			"node_modules",
			"vendor",
			"dist",
			"build",
			".idea",
			".vscode",
			"*.test",
			"*_test.go",
		},
		Debounce: 100 * time.Millisecond,
	}
}

// Watcher watches for file changes in a project directory
type Watcher struct {
	config   *WatcherConfig
	watcher  *fsnotify.Watcher
	events   chan FileEvent
	errors   chan error
	done     chan struct{}
	mu       sync.RWMutex
	running  bool

	// Debouncing
	pending   map[string]*pendingEvent
	pendingMu sync.Mutex
}

type pendingEvent struct {
	event FileEvent
	timer *time.Timer
}

// NewWatcher creates a new file watcher
func NewWatcher(config *WatcherConfig) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		config:  config,
		watcher: fsWatcher,
		events:  make(chan FileEvent, 100),
		errors:  make(chan error, 10),
		done:    make(chan struct{}),
		pending: make(map[string]*pendingEvent),
	}, nil
}

// Start begins watching for file changes
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	// Add the project directory recursively
	if err := w.addRecursive(w.config.ProjectDir); err != nil {
		return err
	}

	// Start the event processing goroutine
	go w.processEvents(ctx)

	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	w.running = false
	close(w.done)

	return w.watcher.Close()
}

// Events returns the channel of file events
func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// Errors returns the channel of errors
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// addRecursive adds a directory and all subdirectories to the watcher
func (w *Watcher) addRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() {
			for _, pattern := range w.config.IgnorePatterns {
				if matched, _ := filepath.Match(pattern, info.Name()); matched {
					return filepath.SkipDir
				}
			}
			return w.watcher.Add(path)
		}

		return nil
	})
}

// processEvents processes fsnotify events and emits debounced FileEvents
func (w *Watcher) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			default:
			}
		}
	}
}

// handleEvent handles a single fsnotify event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Skip if doesn't match any pattern
	if !w.matchesPattern(event.Name) {
		return
	}

	// Skip if matches ignore pattern
	if w.shouldIgnore(event.Name) {
		return
	}

	// Convert fsnotify event to FileEvent
	var eventType FileEventType
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		eventType = FileEventCreated
	case event.Op&fsnotify.Write == fsnotify.Write:
		eventType = FileEventModified
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = FileEventDeleted
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = FileEventRenamed
	default:
		return
	}

	fileEvent := FileEvent{
		Path:      event.Name,
		Type:      eventType,
		Timestamp: time.Now(),
	}

	// Debounce the event
	w.debounce(fileEvent)
}

// debounce debounces file events
func (w *Watcher) debounce(event FileEvent) {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	// Cancel existing pending event for this path
	if pending, ok := w.pending[event.Path]; ok {
		pending.timer.Stop()
	}

	// Create new pending event
	timer := time.AfterFunc(w.config.Debounce, func() {
		w.pendingMu.Lock()
		delete(w.pending, event.Path)
		w.pendingMu.Unlock()

		select {
		case w.events <- event:
		default:
			// Channel full, drop event
		}
	})

	w.pending[event.Path] = &pendingEvent{
		event: event,
		timer: timer,
	}
}

// matchesPattern checks if a file matches any of the watch patterns
func (w *Watcher) matchesPattern(path string) bool {
	if len(w.config.Patterns) == 0 {
		return true
	}

	base := filepath.Base(path)
	for _, pattern := range w.config.Patterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	return false
}

// shouldIgnore checks if a file should be ignored
func (w *Watcher) shouldIgnore(path string) bool {
	// Check each path component
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		for _, pattern := range w.config.IgnorePatterns {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}

	return false
}

// IsRunning returns whether the watcher is running
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}
