// Package builder provides pluggable code generation from forge.json definitions.
// This is the public API for code generation, following Encore-inspired patterns.
package builder

import (
	"context"
	"fmt"
)

// Builder is the interface for code generation from forge.json definitions.
// Different builders handle different project types (Go services, Angular apps, etc.)
type Builder interface {
	// Name returns the builder identifier (e.g., "go-service", "angular-app")
	Name() string

	// Description returns a human-readable description
	Description() string

	// Parse parses the forge.json and extracts relevant information for this builder
	Parse(ctx context.Context, opts ParseOptions) (*ParseResult, error)

	// Generate produces code from the parsed result
	Generate(ctx context.Context, opts GenerateOptions) error

	// Validate checks if the configuration is valid for this builder
	Validate(ctx context.Context, opts ValidateOptions) error
}

// ParseOptions contains options for parsing forge.json
type ParseOptions struct {
	// ProjectDir is the project directory containing forge.json
	ProjectDir string

	// ForgeJSON is the raw forge.json content (if already loaded)
	ForgeJSON []byte
}

// ParseResult contains the parsed information from forge.json
type ParseResult struct {
	// ProjectName is the name of the project
	ProjectName string

	// ProjectType is the type of project (e.g., "go-service", "angular-app")
	ProjectType string

	// Nodes contains the parsed node definitions
	Nodes []Node

	// Edges contains the parsed edge definitions
	Edges []Edge

	// Metadata contains additional project metadata
	Metadata map[string]interface{}
}

// Node represents a node in the forge.json graph
type Node struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Position Position               `json:"position"`
	Data     map[string]interface{} `json:"data"`
}

// Position represents a node's position on the canvas
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Edge represents a connection between nodes
type Edge struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	SourcePort string `json:"sourcePort,omitempty"`
	TargetPort string `json:"targetPort,omitempty"`
}

// GenerateOptions contains options for code generation
type GenerateOptions struct {
	// ProjectDir is the project directory
	ProjectDir string

	// OutputDir is the output directory (defaults to ProjectDir)
	OutputDir string

	// ParseResult is the result from Parse()
	ParseResult *ParseResult

	// DryRun indicates whether to preview changes without writing
	DryRun bool

	// Verbose enables verbose output
	Verbose bool

	// ProgressFunc is called with progress updates
	ProgressFunc func(percent int, message string)
}

// GeneratedFile represents a file that was/will be generated
type GeneratedFile struct {
	Path     string
	Content  []byte
	IsNew    bool
	Modified bool
}

// ValidateOptions contains options for validation
type ValidateOptions struct {
	// ProjectDir is the project directory
	ProjectDir string

	// ParseResult is the result from Parse()
	ParseResult *ParseResult

	// Strict enables strict validation mode
	Strict bool
}

// ValidationError represents a validation error
type ValidationError struct {
	NodeID  string `json:"nodeId,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Severe  bool   `json:"severe"`
}

func (e ValidationError) Error() string {
	if e.NodeID != "" {
		return fmt.Sprintf("node %s: %s", e.NodeID, e.Message)
	}
	return e.Message
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Registry manages available builders
type Registry struct {
	builders map[string]Builder
}

// NewRegistry creates a new builder registry
func NewRegistry() *Registry {
	return &Registry{
		builders: make(map[string]Builder),
	}
}

// Register adds a builder to the registry
func (r *Registry) Register(builder Builder) error {
	name := builder.Name()
	if _, exists := r.builders[name]; exists {
		return fmt.Errorf("builder %q already registered", name)
	}
	r.builders[name] = builder
	return nil
}

// Get retrieves a builder by name
func (r *Registry) Get(name string) (Builder, error) {
	builder, exists := r.builders[name]
	if !exists {
		return nil, fmt.Errorf("builder %q not found", name)
	}
	return builder, nil
}

// Resolve finds the appropriate builder for a project type
func (r *Registry) Resolve(projectType string) Builder {
	builder, _ := r.builders[projectType]
	return builder
}

// List returns all registered builder names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.builders))
	for name := range r.builders {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry is the global builder registry
var DefaultRegistry = NewRegistry()

// Register registers a builder in the default registry
func Register(builder Builder) error {
	return DefaultRegistry.Register(builder)
}

// Get retrieves a builder from the default registry
func Get(name string) (Builder, error) {
	return DefaultRegistry.Get(name)
}

// Resolve finds the appropriate builder for a project type
func Resolve(projectType string) Builder {
	return DefaultRegistry.Resolve(projectType)
}

// List returns all builders in the default registry
func List() []string {
	return DefaultRegistry.List()
}
