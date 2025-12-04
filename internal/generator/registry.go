// Package generator provides the generator framework for creating projects and components.
package generator

import (
	"context"
	"fmt"
)

// Generator defines the interface for all generators.
type Generator interface {
	// Name returns the name of the generator.
	Name() string

	// Description returns a human-readable description.
	Description() string

	// Generate executes the generator with the given context and options.
	Generate(ctx context.Context, opts GeneratorOptions) error
}

// GeneratorOptions contains common options for all generators.
type GeneratorOptions struct {
	// OutputDir is the base output directory.
	OutputDir string

	// Name is the name of the thing being generated (project, service, handler, etc.).
	Name string

	// Data contains generator-specific data.
	Data map[string]interface{}

	// DryRun indicates whether to perform a dry run (preview only).
	DryRun bool
}

// Registry manages available generators.
type Registry struct {
	generators map[string]Generator
}

// NewRegistry creates a new generator registry.
func NewRegistry() *Registry {
	return &Registry{
		generators: make(map[string]Generator),
	}
}

// Register adds a generator to the registry.
func (r *Registry) Register(generator Generator) error {
	name := generator.Name()
	if _, exists := r.generators[name]; exists {
		return fmt.Errorf("generator %q already registered", name)
	}

	r.generators[name] = generator
	return nil
}

// Get retrieves a generator by name.
func (r *Registry) Get(name string) (Generator, error) {
	generator, exists := r.generators[name]
	if !exists {
		return nil, fmt.Errorf("generator %q not found", name)
	}

	return generator, nil
}

// List returns all registered generator names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.generators))
	for name := range r.generators {
		names = append(names, name)
	}
	return names
}

// Has checks if a generator is registered.
func (r *Registry) Has(name string) bool {
	_, exists := r.generators[name]
	return exists
}

// DefaultRegistry is the global generator registry.
var DefaultRegistry = NewRegistry()

// Register registers a generator in the default registry.
func Register(generator Generator) error {
	return DefaultRegistry.Register(generator)
}

// Get retrieves a generator from the default registry.
func Get(name string) (Generator, error) {
	return DefaultRegistry.Get(name)
}

// List returns all generators in the default registry.
func List() []string {
	return DefaultRegistry.List()
}
