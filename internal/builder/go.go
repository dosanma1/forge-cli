// Package builder provides Go-specific build implementation.
package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GoBuilder implements the Builder interface for Go projects
type GoBuilder struct{}

// NewGoBuilder creates a new Go builder
func NewGoBuilder() *GoBuilder {
	return &GoBuilder{}
}

// Name returns the builder name
func (b *GoBuilder) Name() string {
	return "@forge/go:build"
}

// Build executes the Go build
func (b *GoBuilder) Build(ctx context.Context, opts *BuildOptions) error {
	if err := b.Validate(opts); err != nil {
		return err
	}

	// Extract Go-specific options
	goVersion := getStringOption(opts.Options, "goVersion", "")
	registry := getStringOption(opts.Options, "registry", "")
	dockerfile := getStringOption(opts.Options, "dockerfile", "Dockerfile")
	ldflags := getStringOption(opts.Options, "ldflags", "")
	race := getBoolOption(opts.Options, "race", false)
	tags := getStringSliceOption(opts.Options, "tags", nil)

	// Merge configuration-specific options
	if opts.ConfigurationOptions != nil {
		if v, ok := opts.ConfigurationOptions["registry"].(string); ok && v != "" {
			registry = v
		}
	}

	if opts.Verbose {
		fmt.Printf("Building Go project at %s\n", opts.ProjectRoot)
		fmt.Printf("  Go Version: %s\n", goVersion)
		fmt.Printf("  Registry: %s\n", registry)
		fmt.Printf("  Dockerfile: %s\n", dockerfile)
		fmt.Printf("  Configuration: %s\n", opts.Configuration)
	}

	// Build the Docker image using Bazel or Docker
	if b.useBazel(opts.ProjectRoot) {
		return b.buildWithBazel(ctx, opts)
	}

	return b.buildWithDocker(ctx, opts, registry, dockerfile, ldflags, race, tags)
}

// Validate validates the build options
func (b *GoBuilder) Validate(opts *BuildOptions) error {
	if opts.ProjectRoot == "" {
		return fmt.Errorf("project root is required")
	}

	if _, err := os.Stat(opts.ProjectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project root does not exist: %s", opts.ProjectRoot)
	}

	return nil
}

// useBazel checks if the project uses Bazel
func (b *GoBuilder) useBazel(projectRoot string) bool {
	buildFile := filepath.Join(projectRoot, "BUILD.bazel")
	_, err := os.Stat(buildFile)
	return err == nil
}

// buildWithBazel builds using Bazel
func (b *GoBuilder) buildWithBazel(ctx context.Context, opts *BuildOptions) error {
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
func (b *GoBuilder) buildWithDocker(ctx context.Context, opts *BuildOptions, registry, dockerfile, ldflags string, race bool, tags []string) error {
	// Get the project name from the directory
	projectName := filepath.Base(opts.ProjectRoot)
	imageName := fmt.Sprintf("%s/%s", registry, projectName)

	// Build Docker image
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

// Helper functions to extract typed values from map[string]interface{}
func getStringOption(opts map[string]interface{}, key, defaultValue string) string {
	if v, ok := opts[key].(string); ok {
		return v
	}
	return defaultValue
}

func getBoolOption(opts map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := opts[key].(bool); ok {
		return v
	}
	return defaultValue
}

func getStringSliceOption(opts map[string]interface{}, key string, defaultValue []string) []string {
	if v, ok := opts[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return defaultValue
}

func init() {
	// Register the Go builder in the default registry
	Register(NewGoBuilder())
}
