package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dosanma1/forge-cli/internal/bazel"
	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	buildVerbose bool
	buildConfig  string
	buildService string
)

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build services using Bazel",
	Long: `Build one or more services in your workspace using Bazel.

This command builds your backend and frontend code only. 
Docker images and deployment are handled by 'forge deploy'.

Environments (--config):
  local   - Fast builds, no optimization (default)
  dev     - Some optimization, source maps
  prod    - Full optimization, no debug info

Examples:
  forge build                        # Build all (local config)
  forge build --config=prod          # Build all for production
  forge build api-server             # Build specific service
  forge build api-server worker      # Build multiple services
  forge build --config=dev --verbose # Dev build with details`,
	RunE: runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVarP(&buildVerbose, "verbose", "v", false, "Show detailed build output")
	buildCmd.Flags().StringVarP(&buildConfig, "config", "c", "local", "Build configuration (local|dev|prod)")
	buildCmd.Flags().StringVarP(&buildService, "service", "s", "", "Build specific service")
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate config
	validConfigs := map[string]bool{"local": true, "dev": true, "prod": true}
	if !validConfigs[buildConfig] {
		return fmt.Errorf("invalid config: %s (must be local, dev, or prod)", buildConfig)
	}

	// Get workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("not in a forge workspace: %w", err)
	}

	// Load workspace config to check for remote cache
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Create Bazel executor with config
	executor, err := bazel.NewExecutor(workspaceRoot, buildVerbose)
	if err != nil {
		return err
	}

	// Determine what to build
	var targets []string

	if len(args) > 0 {
		// Build specific services
		for _, service := range args {
			targets = append(targets, serviceToTarget(service))
		}
	} else if buildService != "" {
		// Build from --service flag
		targets = append(targets, serviceToTarget(buildService))
	} else {
		// Build everything
		targets = append(targets, "//...")
	}

	// Build Bazel args with environment config
	var bazelFlags []string
	bazelFlags = append(bazelFlags, fmt.Sprintf("--config=%s", buildConfig))

	// Add remote cache if configured
	if config.Build != nil && config.Build.Cache != nil && config.Build.Cache.RemoteURL != "" {
		bazelFlags = append(bazelFlags, fmt.Sprintf("--remote_cache=%s", config.Build.Cache.RemoteURL))
		if buildVerbose {
			fmt.Printf("  Using remote cache: %s\n", config.Build.Cache.RemoteURL)
		}
	}

	// Add parallel workers if configured
	if config.Build != nil && config.Build.Parallel != nil && config.Build.Parallel.Workers > 0 {
		bazelFlags = append(bazelFlags, fmt.Sprintf("--jobs=%d", config.Build.Parallel.Workers))
		if buildVerbose {
			fmt.Printf("  Using %d parallel workers\n", config.Build.Parallel.Workers)
		}
	}

	// Show user-friendly message
	configEmoji := map[string]string{
		"local": "🏠",
		"dev":   "🔧",
		"prod":  "🚀",
	}
	emoji := configEmoji[buildConfig]

	if buildVerbose {
		fmt.Printf("%s Building with config '%s': %s\n", emoji, buildConfig, strings.Join(targets, ", "))
	} else {
		serviceNames := extractServiceNames(args)
		if len(serviceNames) == 0 {
			fmt.Printf("%s Building all services [%s]...\n", emoji, buildConfig)
		} else {
			fmt.Printf("%s Building: %s [%s]\n", emoji, strings.Join(serviceNames, ", "), buildConfig)
		}
	}

	// Execute build via Bazel
	if err := executor.BuildWithFlags(ctx, bazelFlags, targets); err != nil {
		// Translate Bazel error to user-friendly message
		translator := bazel.NewErrorTranslator()
		friendlyError := translator.Translate(err.Error())
		return fmt.Errorf("❌ Build failed:\n%s", friendlyError)
	}

	fmt.Println("✅ Build completed successfully!")
	return nil
}

// serviceToTarget converts a service name to a Bazel target.
func serviceToTarget(serviceName string) string {
	// Convert service name to Bazel target
	// Example: "api-server" -> "//backend/services/api-server/cmd/server:image"
	return fmt.Sprintf("//backend/services/%s/...", serviceName)
}

// extractServiceNames extracts user-friendly service names from args.
func extractServiceNames(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	return args
}

// findWorkspaceRoot finds the root of the forge workspace.
func findWorkspaceRoot() (string, error) {
	// Look for forge.json or MODULE.bazel
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check for forge.json
		if _, err := os.Stat(fmt.Sprintf("%s/forge.json", dir)); err == nil {
			return dir, nil
		}

		// Check for MODULE.bazel (fallback)
		if _, err := os.Stat(fmt.Sprintf("%s/MODULE.bazel", dir)); err == nil {
			return dir, nil
		}

		// Move up one directory
		parent := fmt.Sprintf("%s/..", dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("not a forge workspace (no forge.json found)")
}
