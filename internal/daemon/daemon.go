package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dosanma1/forge-cli/pkg/builder"
	"github.com/dosanma1/forge-cli/pkg/generator"
	"google.golang.org/grpc"
)

// Config contains daemon configuration
type Config struct {
	// SocketPath is the Unix socket path for gRPC communication
	SocketPath string

	// WorkspaceDir is the workspace directory to serve
	WorkspaceDir string

	// Version is the daemon version
	Version string
}

// DefaultConfig returns default daemon configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		SocketPath:   filepath.Join(homeDir, ".forge", "daemon.sock"),
		WorkspaceDir: ".",
		Version:      "1.0.0",
	}
}

// Daemon is the Forge daemon server
type Daemon struct {
	config     *Config
	server     *grpc.Server
	listener   net.Listener
	watcher    *Watcher
	startTime  time.Time

	// Event subscribers
	subscribers   map[string]chan FileEvent
	subscribersMu sync.RWMutex

	// Shutdown coordination
	done chan struct{}
	mu   sync.RWMutex
}

// New creates a new daemon instance
func New(config *Config) *Daemon {
	return &Daemon{
		config:      config,
		subscribers: make(map[string]chan FileEvent),
		done:        make(chan struct{}),
	}
}

// Start starts the daemon server
func (d *Daemon) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure socket directory exists
	socketDir := filepath.Dir(d.config.SocketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Remove existing socket file if present
	if _, err := os.Stat(d.config.SocketPath); err == nil {
		if err := os.Remove(d.config.SocketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket: %w", err)
		}
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", d.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	d.listener = listener

	// Create gRPC server
	d.server = grpc.NewServer()

	// Register the daemon service
	// Note: We'd normally use generated proto code here, but for now we'll skip registration
	// and implement the service methods directly

	d.startTime = time.Now()

	// Start the gRPC server in a goroutine
	go func() {
		if err := d.server.Serve(listener); err != nil {
			// Log error but don't crash - server may have been stopped intentionally
			fmt.Fprintf(os.Stderr, "gRPC server error: %v\n", err)
		}
	}()

	// Start file watcher if workspace dir is set
	if d.config.WorkspaceDir != "" {
		if err := d.startWatcher(ctx); err != nil {
			return fmt.Errorf("failed to start watcher: %w", err)
		}
	}

	return nil
}

// Stop stops the daemon server
func (d *Daemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	close(d.done)

	// Stop watcher
	if d.watcher != nil {
		d.watcher.Stop()
	}

	// Stop gRPC server
	if d.server != nil {
		d.server.GracefulStop()
	}

	// Close listener
	if d.listener != nil {
		d.listener.Close()
	}

	// Remove socket file
	os.Remove(d.config.SocketPath)

	return nil
}

// startWatcher starts the file watcher
func (d *Daemon) startWatcher(ctx context.Context) error {
	config := DefaultWatcherConfig(d.config.WorkspaceDir)
	watcher, err := NewWatcher(config)
	if err != nil {
		return err
	}

	d.watcher = watcher

	if err := watcher.Start(ctx); err != nil {
		return err
	}

	// Forward watcher events to subscribers
	go d.forwardEvents(ctx)

	return nil
}

// forwardEvents forwards file events to all subscribers
func (d *Daemon) forwardEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.done:
			return
		case event := <-d.watcher.Events():
			d.broadcastEvent(event)
		}
	}
}

// broadcastEvent sends an event to all subscribers
func (d *Daemon) broadcastEvent(event FileEvent) {
	d.subscribersMu.RLock()
	defer d.subscribersMu.RUnlock()

	for _, ch := range d.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// Subscribe creates a new event subscription
func (d *Daemon) Subscribe(id string) <-chan FileEvent {
	d.subscribersMu.Lock()
	defer d.subscribersMu.Unlock()

	ch := make(chan FileEvent, 100)
	d.subscribers[id] = ch
	return ch
}

// Unsubscribe removes an event subscription
func (d *Daemon) Unsubscribe(id string) {
	d.subscribersMu.Lock()
	defer d.subscribersMu.Unlock()

	if ch, ok := d.subscribers[id]; ok {
		close(ch)
		delete(d.subscribers, id)
	}
}

// Status returns the daemon status
func (d *Daemon) Status() *StatusInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	activeWatchers := 0
	if d.watcher != nil && d.watcher.IsRunning() {
		activeWatchers = 1
	}

	return &StatusInfo{
		Running:        d.server != nil,
		Version:        d.config.Version,
		UptimeSeconds:  int64(time.Since(d.startTime).Seconds()),
		WorkspaceDir:   d.config.WorkspaceDir,
		ActiveWatchers: activeWatchers,
	}
}

// StatusInfo contains daemon status information
type StatusInfo struct {
	Running        bool
	Version        string
	UptimeSeconds  int64
	WorkspaceDir   string
	ActiveWatchers int
}

// Generate triggers code generation for a project
func (d *Daemon) Generate(ctx context.Context, projectDir string, dryRun bool, progressFunc func(int, string)) error {
	// Get the appropriate builder
	b := builder.Resolve("go-service")
	if b == nil {
		return fmt.Errorf("no builder found for go-service")
	}

	// Parse the forge.json
	parseResult, err := b.Parse(ctx, builder.ParseOptions{
		ProjectDir: projectDir,
	})
	if err != nil {
		return fmt.Errorf("failed to parse forge.json: %w", err)
	}

	// Generate code
	return b.Generate(ctx, builder.GenerateOptions{
		ProjectDir:   projectDir,
		ParseResult:  parseResult,
		DryRun:       dryRun,
		ProgressFunc: progressFunc,
	})
}

// CreateWorkspace creates a new workspace
func (d *Daemon) CreateWorkspace(ctx context.Context, name, path string, progressFunc func(int, string)) error {
	gen := generator.NewWorkspaceGenerator()

	if progressFunc != nil {
		progressFunc(0, "Creating workspace...")
	}

	err := gen.Generate(ctx, generator.GeneratorOptions{
		OutputDir: path,
		Name:      name,
	})

	if progressFunc != nil {
		if err != nil {
			progressFunc(100, fmt.Sprintf("Failed: %v", err))
		} else {
			progressFunc(100, "Workspace created successfully")
		}
	}

	return err
}

// Validate validates a project's forge.json
func (d *Daemon) Validate(ctx context.Context, projectDir string, strict bool) (*ValidationResult, error) {
	// Get the appropriate builder
	b := builder.Resolve("go-service")
	if b == nil {
		return nil, fmt.Errorf("no builder found for go-service")
	}

	// Parse the forge.json
	parseResult, err := b.Parse(ctx, builder.ParseOptions{
		ProjectDir: projectDir,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse forge.json: %w", err)
	}

	// Validate
	err = b.Validate(ctx, builder.ValidateOptions{
		ProjectDir:  projectDir,
		ParseResult: parseResult,
		Strict:      strict,
	})

	if err != nil {
		if vr, ok := err.(*builder.ValidationResult); ok {
			return &ValidationResult{
				Valid:  vr.Valid,
				Errors: convertValidationErrors(vr.Errors),
			}, nil
		}
		return nil, err
	}

	return &ValidationResult{Valid: true}, nil
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool
	Errors []ValidationErrorInfo
}

// ValidationErrorInfo contains validation error details
type ValidationErrorInfo struct {
	NodeID  string
	Field   string
	Message string
	Severe  bool
}

func convertValidationErrors(errors []builder.ValidationError) []ValidationErrorInfo {
	result := make([]ValidationErrorInfo, len(errors))
	for i, e := range errors {
		result[i] = ValidationErrorInfo{
			NodeID:  e.NodeID,
			Field:   e.Field,
			Message: e.Message,
			Severe:  e.Severe,
		}
	}
	return result
}

// SocketPath returns the socket path for connecting to this daemon
func (d *Daemon) SocketPath() string {
	return d.config.SocketPath
}

// Client creates a gRPC client connected to this daemon
func (d *Daemon) Client() (*grpc.ClientConn, error) {
	return grpc.Dial(
		"unix://"+d.config.SocketPath,
		grpc.WithInsecure(),
	)
}
