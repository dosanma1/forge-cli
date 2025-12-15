package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/builder"
	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	buildVerbose  bool
	buildEnv      string
	buildPush     bool
	buildPlatform string
)

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build services using Skaffold and Bazel",
	Long: `Build one or more services in your workspace using Skaffold with Bazel.

Skaffold orchestrates Bazel builds based on your forge.json configuration.
Each configuration (local, development, production) becomes a Skaffold profile
with environment-specific build settings.

Use --push to build and push Docker images to the registry.

Examples:
  forge build                            # Build all services using default config
  forge build --env=production           # Build all for production
  forge build --push                     # Build and push Docker images
  forge build api-server                 # Build specific service
  forge build api-server worker          # Build multiple services
  forge build --env=development --verbose # Dev build with details
  forge build --platform=linux/arm64     # Build for specific platform`,
	RunE: runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVarP(&buildVerbose, "verbose", "v", false, "Show detailed build output")
	buildCmd.Flags().StringVarP(&buildEnv, "env", "e", "", "Build environment/profile (local, development, production)")
	buildCmd.Flags().BoolVar(&buildPush, "push", false, "Build and push Docker images to registry")
	buildCmd.Flags().StringVar(&buildPlatform, "platform", "linux/amd64", "Target platform for builds")
}

func runBuild(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Using direct builder (not Skaffold)")
	ctx := context.Background()

	// Get workspace root
	workspaceRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load forge.json (with validation)
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load forge.json: %w", err)
	}

	// Determine which projects to build
	projectNames := args
	if len(projectNames) == 0 {
		// Build all projects
		for name := range config.Projects {
			projectNames = append(projectNames, name)
		}
	}

	// Validate that all specified projects exist
	for _, projectName := range projectNames {
		if _, exists := config.Projects[projectName]; !exists {
			return fmt.Errorf("project %q not found in forge.json", projectName)
		}
	}

	// Build all projects using their configured builders
	// Build command ALWAYS uses direct builders (never Skaffold)
	for _, projectName := range projectNames {
		project := config.Projects[projectName]

		if project.Architect == nil || project.Architect.Build == nil {
			return fmt.Errorf("project %s has no build configuration", projectName)
		}

		// Determine configuration
		buildConfig := buildEnv
		if buildConfig == "" && project.Architect.Build.DefaultConfiguration != "" {
			buildConfig = project.Architect.Build.DefaultConfiguration
		}
		if buildConfig == "" {
			buildConfig = "production"
		}

		// Get builder
		builderName := project.Architect.Build.Builder
		projectBuilder, err := builder.GetBuilder(builderName)
		if err != nil {
			return fmt.Errorf("failed to get builder for %s: %w", projectName, err)
		}

		if buildVerbose {
			fmt.Printf("🔨 Building %s with %s (configuration: %s)\n", projectName, builderName, buildConfig)
		}

		// Get project absolute path
		projectAbsPath := filepath.Join(workspaceRoot, project.Root)

		// Get build options and configuration options
		buildOpts := project.Architect.Build.Options
		var configOpts map[string]interface{}
		if project.Architect.Build.Configurations != nil {
			if cfg, ok := project.Architect.Build.Configurations[buildConfig]; ok {
				if typedCfg, ok := cfg.(map[string]interface{}); ok {
					configOpts = typedCfg
				}
			}
		}

		// Build using the configured builder
		opts := &builder.BuildOptions{
			ProjectRoot:          projectAbsPath,
			Configuration:        buildConfig,
			Options:              buildOpts,
			ConfigurationOptions: configOpts,
			Verbose:              buildVerbose,
			Platform:             buildPlatform,
			WorkspaceRoot:        workspaceRoot,
		}

		artifact, err := projectBuilder.Build(ctx, opts)
		if err != nil {
			return fmt.Errorf("❌ Build failed for %s: %w", projectName, err)
		}

		if buildVerbose {
			fmt.Printf("✅ Built %s: %s at %s\n", projectName, artifact.Type, artifact.Path)
		}
	}

	fmt.Printf("✅ All builds completed successfully!\n")
	return nil
}

// findAngularWorkspaceRoot finds the directory containing angular.json
// by walking up from the project root
func findAngularWorkspaceRoot(workspaceRoot, projectRoot string) string {
	// Start from the project's absolute path
	projectAbsPath := filepath.Join(workspaceRoot, projectRoot)

	// Walk up directories looking for angular.json
	currentPath := projectAbsPath
	for {
		angularJsonPath := filepath.Join(currentPath, "angular.json")
		if _, err := os.Stat(angularJsonPath); err == nil {
			return currentPath
		}

		// Move up one directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath || parentPath == "/" {
			// Reached root without finding angular.json
			// Return workspace root as fallback
			return workspaceRoot
		}
		currentPath = parentPath
	}
}
