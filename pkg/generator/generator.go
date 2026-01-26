// Package generator provides public APIs for code generation.
// This package re-exports types from internal/generator for external use.
package generator

import (
	"context"

	internal "github.com/dosanma1/forge-cli/internal/generator"
)

// Generator defines the interface for all generators.
type Generator = internal.Generator

// GeneratorOptions contains common options for all generators.
type GeneratorOptions = internal.GeneratorOptions

// WorkspaceGenerator generates a new Forge workspace.
type WorkspaceGenerator struct {
	gen *internal.WorkspaceGenerator
}

// NewWorkspaceGenerator creates a new workspace generator.
func NewWorkspaceGenerator() *WorkspaceGenerator {
	return &WorkspaceGenerator{
		gen: internal.NewWorkspaceGenerator(),
	}
}

// Generate creates a new workspace.
func (g *WorkspaceGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	return g.gen.Generate(ctx, opts)
}

// Name returns the generator name.
func (g *WorkspaceGenerator) Name() string {
	return g.gen.Name()
}

// Description returns the generator description.
func (g *WorkspaceGenerator) Description() string {
	return g.gen.Description()
}

// ServiceGenerator generates a new Go microservice.
type ServiceGenerator struct {
	gen *internal.ServiceGenerator
}

// NewServiceGenerator creates a new service generator.
func NewServiceGenerator() *ServiceGenerator {
	return &ServiceGenerator{
		gen: internal.NewServiceGenerator(),
	}
}

// Generate creates a new service.
func (g *ServiceGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	return g.gen.Generate(ctx, opts)
}

// Name returns the generator name.
func (g *ServiceGenerator) Name() string {
	return g.gen.Name()
}

// Description returns the generator description.
func (g *ServiceGenerator) Description() string {
	return g.gen.Description()
}

// FrontendGenerator generates a new Angular application.
type FrontendGenerator struct {
	gen *internal.FrontendGenerator
}

// NewFrontendGenerator creates a new frontend generator.
func NewFrontendGenerator() *FrontendGenerator {
	return &FrontendGenerator{
		gen: internal.NewFrontendGenerator(),
	}
}

// Generate creates a new Angular application.
func (g *FrontendGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	return g.gen.Generate(ctx, opts)
}

// Name returns the generator name.
func (g *FrontendGenerator) Name() string {
	return g.gen.Name()
}

// Description returns the generator description.
func (g *FrontendGenerator) Description() string {
	return g.gen.Description()
}

// NestJSServiceGenerator generates a new NestJS service.
type NestJSServiceGenerator struct {
	gen *internal.NestJSServiceGenerator
}

// NewNestJSServiceGenerator creates a new NestJS service generator.
func NewNestJSServiceGenerator() *NestJSServiceGenerator {
	return &NestJSServiceGenerator{
		gen: internal.NewNestJSServiceGenerator(),
	}
}

// Generate creates a new NestJS service.
func (g *NestJSServiceGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	return g.gen.Generate(ctx, opts)
}

// Name returns the generator name.
func (g *NestJSServiceGenerator) Name() string {
	return g.gen.Name()
}

// Description returns the generator description.
func (g *NestJSServiceGenerator) Description() string {
	return g.gen.Description()
}
