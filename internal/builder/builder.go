// Package builder provides build system abstraction for different languages and frameworks.
package builder

import (
	"context"
	"fmt"
)

// Builder is the interface that all language/framework-specific builders must implement.
type Builder interface {
	// Name returns the builder name (e.g., "@forge/go:build", "@forge/angular:build")
	Name() string

	// Build executes the build with the given options and configuration
	Build(ctx context.Context, opts *BuildOptions) error

	// Validate validates the build options
	Validate(opts *BuildOptions) error
}

// BuildOptions contains the options for a build operation
type BuildOptions struct {
	// ProjectRoot is the absolute path to the project root
	ProjectRoot string

	// Configuration is the name of the configuration to use (e.g., "production", "development")
	Configuration string

	// Options are the builder-specific options from forge.json
	Options map[string]interface{}

	// ConfigurationOptions are the configuration-specific overrides
	ConfigurationOptions map[string]interface{}

	// Environment variables
	Env map[string]string

	// Verbose output
	Verbose bool
}

// Registry holds all registered builders
type Registry struct {
	builders map[string]Builder
}

// NewRegistry creates a new builder registry
func NewRegistry() *Registry {
	return &Registry{
		builders: make(map[string]Builder),
	}
}

// Register registers a builder
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
