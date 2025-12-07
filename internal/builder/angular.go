// Package builder provides Angular-specific build implementation.
package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// AngularBuilder implements the Builder interface for Angular projects
type AngularBuilder struct{}

// NewAngularBuilder creates a new Angular builder
func NewAngularBuilder() *AngularBuilder {
	return &AngularBuilder{}
}

// Name returns the builder name
func (b *AngularBuilder) Name() string {
	return "@forge/angular:build"
}

// Build executes the Angular build
func (b *AngularBuilder) Build(ctx context.Context, opts *BuildOptions) error {
	if err := b.Validate(opts); err != nil {
		return err
	}

	// Extract Angular-specific options
	outputPath := getStringOption(opts.Options, "outputPath", "dist")
	optimization := getBoolOption(opts.Options, "optimization", false)
	sourceMap := getBoolOption(opts.Options, "sourceMap", true)
	envMapper := getMapOption(opts.Options, "environmentMapper", nil)

	// Map forge configuration to Angular configuration
	angularConfig := opts.Configuration
	if envMapper != nil {
		if mapped, ok := envMapper[opts.Configuration].(string); ok {
			angularConfig = mapped
		}
	}

	// Merge configuration-specific options
	if opts.ConfigurationOptions != nil {
		if v, ok := opts.ConfigurationOptions["outputPath"].(string); ok && v != "" {
			outputPath = v
		}
		if v, ok := opts.ConfigurationOptions["optimization"].(bool); ok {
			optimization = v
		}
	}

	if opts.Verbose {
		fmt.Printf("Building Angular project at %s\n", opts.ProjectRoot)
		fmt.Printf("  Output Path: %s\n", outputPath)
		fmt.Printf("  Configuration: %s (mapped to %s)\n", opts.Configuration, angularConfig)
		fmt.Printf("  Optimization: %v\n", optimization)
	}

	// Check if using Bazel
	if b.useBazel(opts.ProjectRoot) {
		return b.buildWithBazel(ctx, opts)
	}

	return b.buildWithNg(ctx, opts, angularConfig, outputPath, optimization, sourceMap)
}

// Validate validates the build options
func (b *AngularBuilder) Validate(opts *BuildOptions) error {
	if opts.ProjectRoot == "" {
		return fmt.Errorf("project root is required")
	}

	if _, err := os.Stat(opts.ProjectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project root does not exist: %s", opts.ProjectRoot)
	}

	// Check for angular.json
	angularJSON := filepath.Join(opts.ProjectRoot, "angular.json")
	if _, err := os.Stat(angularJSON); os.IsNotExist(err) {
		// Try parent directories for workspace setup
		parent := filepath.Dir(opts.ProjectRoot)
		angularJSON = filepath.Join(parent, "angular.json")
		if _, err := os.Stat(angularJSON); os.IsNotExist(err) {
			return fmt.Errorf("angular.json not found in project root or parent")
		}
	}

	return nil
}

// useBazel checks if the project uses Bazel
func (b *AngularBuilder) useBazel(projectRoot string) bool {
	buildFile := filepath.Join(projectRoot, "BUILD.bazel")
	_, err := os.Stat(buildFile)
	return err == nil
}

// buildWithBazel builds using Bazel
func (b *AngularBuilder) buildWithBazel(ctx context.Context, opts *BuildOptions) error {
	cmd := exec.CommandContext(ctx, "bazel", "build", "//...")
	cmd.Dir = opts.ProjectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bazel build failed: %w", err)
	}

	return nil
}

// buildWithNg builds using Angular CLI
func (b *AngularBuilder) buildWithNg(ctx context.Context, opts *BuildOptions, angularConfig, outputPath string, optimization, sourceMap bool) error {
	// Determine the project name from the directory
	projectName := filepath.Base(opts.ProjectRoot)

	args := []string{"build", projectName}

	// Add configuration
	if angularConfig != "" {
		args = append(args, "--configuration="+angularConfig)
	}

	// Add output path
	if outputPath != "" {
		args = append(args, "--output-path="+outputPath)
	}

	// Add optimization flag if explicitly set
	if optimization {
		args = append(args, "--optimization=true")
	}

	// Add source map flag
	if !sourceMap {
		args = append(args, "--source-map=false")
	}

	cmd := exec.CommandContext(ctx, "ng", args...)

	// Run from the directory containing angular.json
	angularJSONDir := b.findAngularJSONDir(opts.ProjectRoot)
	cmd.Dir = angularJSONDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ng build failed: %w", err)
	}

	if opts.Verbose {
		fmt.Printf("Successfully built Angular project: %s\n", projectName)
	}

	return nil
}

// findAngularJSONDir finds the directory containing angular.json
func (b *AngularBuilder) findAngularJSONDir(projectRoot string) string {
	angularJSON := filepath.Join(projectRoot, "angular.json")
	if _, err := os.Stat(angularJSON); err == nil {
		return projectRoot
	}

	// Try parent directory
	parent := filepath.Dir(projectRoot)
	angularJSON = filepath.Join(parent, "angular.json")
	if _, err := os.Stat(angularJSON); err == nil {
		return parent
	}

	// Default to project root
	return projectRoot
}

func getMapOption(opts map[string]interface{}, key string, defaultValue map[string]interface{}) map[string]interface{} {
	if v, ok := opts[key].(map[string]interface{}); ok {
		return v
	}
	return defaultValue
}

func init() {
	// Register the Angular builder in the default registry
	Register(NewAngularBuilder())
}
