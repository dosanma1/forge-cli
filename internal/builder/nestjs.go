// Package builder provides NestJS-specific build implementation.
package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// NestJSBuilder implements the Builder interface for NestJS projects
type NestJSBuilder struct{}

// NewNestJSBuilder creates a new NestJS builder
func NewNestJSBuilder() *NestJSBuilder {
	return &NestJSBuilder{}
}

// Name returns the builder name
func (b *NestJSBuilder) Name() string {
	return "@forge/nestjs:build"
}

// Build executes the NestJS build
func (b *NestJSBuilder) Build(ctx context.Context, opts *BuildOptions) error {
	if err := b.Validate(opts); err != nil {
		return err
	}

	// Extract NestJS-specific options
	nodeVersion := getStringOption(opts.Options, "nodeVersion", "")
	registry := getStringOption(opts.Options, "registry", "")
	dockerfile := getStringOption(opts.Options, "dockerfile", "Dockerfile")
	tsconfig := getStringOption(opts.Options, "tsconfig", "tsconfig.json")

	// Merge configuration-specific options
	if opts.ConfigurationOptions != nil {
		if v, ok := opts.ConfigurationOptions["registry"].(string); ok && v != "" {
			registry = v
		}
	}

	if opts.Verbose {
		fmt.Printf("Building NestJS project at %s\n", opts.ProjectRoot)
		fmt.Printf("  Node Version: %s\n", nodeVersion)
		fmt.Printf("  Registry: %s\n", registry)
		fmt.Printf("  Dockerfile: %s\n", dockerfile)
		fmt.Printf("  Configuration: %s\n", opts.Configuration)
	}

	// Build using npm/nest or Docker
	if b.useBazel(opts.ProjectRoot) {
		return b.buildWithBazel(ctx, opts)
	}

	if registry != "" {
		return b.buildWithDocker(ctx, opts, registry, dockerfile)
	}

	return b.buildWithNest(ctx, opts, tsconfig)
}

// Validate validates the build options
func (b *NestJSBuilder) Validate(opts *BuildOptions) error {
	if opts.ProjectRoot == "" {
		return fmt.Errorf("project root is required")
	}

	if _, err := os.Stat(opts.ProjectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project root does not exist: %s", opts.ProjectRoot)
	}

	// Check for package.json
	packageJSON := filepath.Join(opts.ProjectRoot, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found in project root")
	}

	return nil
}

// useBazel checks if the project uses Bazel
func (b *NestJSBuilder) useBazel(projectRoot string) bool {
	buildFile := filepath.Join(projectRoot, "BUILD.bazel")
	_, err := os.Stat(buildFile)
	return err == nil
}

// buildWithBazel builds using Bazel
func (b *NestJSBuilder) buildWithBazel(ctx context.Context, opts *BuildOptions) error {
	cmd := exec.CommandContext(ctx, "bazel", "build", "//...")
	cmd.Dir = opts.ProjectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bazel build failed: %w", err)
	}

	return nil
}

// buildWithDocker builds using Docker
func (b *NestJSBuilder) buildWithDocker(ctx context.Context, opts *BuildOptions, registry, dockerfile string) error {
	projectName := filepath.Base(opts.ProjectRoot)
	imageName := fmt.Sprintf("%s/%s", registry, projectName)

	args := []string{"build", "-t", imageName}
	if dockerfile != "" {
		args = append(args, "-f", dockerfile)
	}
	args = append(args, ".")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = opts.ProjectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	if opts.Verbose {
		fmt.Printf("Successfully built image: %s\n", imageName)
	}

	return nil
}

// buildWithNest builds using NestJS CLI
func (b *NestJSBuilder) buildWithNest(ctx context.Context, opts *BuildOptions, tsconfig string) error {
	// Run nest build
	args := []string{"run", "build"}

	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = opts.ProjectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nest build failed: %w", err)
	}

	if opts.Verbose {
		fmt.Printf("Successfully built NestJS project\n")
	}

	return nil
}

func init() {
	// Register the NestJS builder in the default registry
	Register(NewNestJSBuilder())
}
