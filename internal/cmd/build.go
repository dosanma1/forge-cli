package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dosanma1/forge-cli/internal/bazel"
	"github.com/spf13/cobra"
)

var (
	buildVerbose bool
	buildPush    bool
	buildService string
)

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build services",
	Long: `Build one or more services in your workspace.

Examples:
  forge build                    # Build all services
  forge build api-server         # Build specific service
  forge build api-server worker  # Build multiple services
  forge build --push             # Build and push to registry`,
	RunE: runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVarP(&buildVerbose, "verbose", "v", false, "Show detailed build output")
	buildCmd.Flags().BoolVar(&buildPush, "push", false, "Push images to registry after build")
	buildCmd.Flags().StringVarP(&buildService, "service", "s", "", "Build specific service")
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("not in a forge workspace: %w", err)
	}

	// Create Bazel executor
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

	// Show user-friendly message
	if buildVerbose {
		fmt.Printf("🔨 Building targets: %s\n", strings.Join(targets, ", "))
	} else {
		serviceNames := extractServiceNames(args)
		if len(serviceNames) == 0 {
			fmt.Println("🔨 Building all services...")
		} else {
			fmt.Printf("🔨 Building: %s\n", strings.Join(serviceNames, ", "))
		}
	}

	// Execute build
	if err := executor.Build(ctx, targets); err != nil {
		// Translate Bazel error to user-friendly message
		translator := bazel.NewErrorTranslator()
		friendlyError := translator.Translate(err.Error())
		return fmt.Errorf("❌ Build failed:\n%s", friendlyError)
	}

	// Push if requested
	if buildPush {
		fmt.Println("📤 Pushing images to registry...")
		// TODO: Implement image push
		fmt.Println("⚠️  Push not yet implemented")
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
	// Look for .forge.yaml or MODULE.bazel
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check for .forge.yaml
		if _, err := os.Stat(fmt.Sprintf("%s/.forge.yaml", dir)); err == nil {
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

	return "", fmt.Errorf("not a forge workspace (no .forge.yaml found)")
}
