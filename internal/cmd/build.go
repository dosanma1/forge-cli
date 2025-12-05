package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dosanma1/forge-cli/internal/bazel"
	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	buildVerbose   bool
	buildConfig    string
	buildService   string
	buildPush      bool
	buildCI        bool
	buildRegistry  string
	buildPlatforms string
)

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build services using Bazel",
	Long: `Build one or more services in your workspace using Bazel.

Bazel automatically detects changed files and only rebuilds affected targets.
Use --push to build and push Docker images to the registry.

Environments (--config):
  local   - Fast builds, no optimization (default)
  dev     - Some optimization, source maps
  prod    - Full optimization, no debug info

Examples:
  forge build                            # Build all (local config)
  forge build --config=prod              # Build all for production
  forge build --push                     # Build and push Docker images
  forge build --ci                       # CI mode (clean logs, prod config)
  forge build api-server                 # Build specific service
  forge build api-server worker          # Build multiple services
  forge build --config=dev --verbose     # Dev build with details
  forge build --push --platforms=linux/amd64,linux/arm64  # Multi-arch`,
	RunE: runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVarP(&buildVerbose, "verbose", "v", false, "Show detailed build output")
	buildCmd.Flags().StringVarP(&buildConfig, "config", "c", "local", "Build configuration (local|dev|prod)")
	buildCmd.Flags().StringVarP(&buildService, "service", "s", "", "Build specific service")
	buildCmd.Flags().BoolVar(&buildPush, "push", false, "Build and push Docker images to registry")
	buildCmd.Flags().BoolVar(&buildCI, "ci", false, "CI mode (clean logs, prod config, no progress)")
	buildCmd.Flags().StringVar(&buildRegistry, "registry", "", "Override Docker registry from forge.json")
	buildCmd.Flags().StringVar(&buildPlatforms, "platforms", "linux/amd64", "Target platforms for multi-arch builds")
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// CI mode overrides
	if buildCI {
		buildConfig = "prod"
		buildVerbose = false
	}

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

	// Load workspace config
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Determine registry
	registry := buildRegistry
	if registry == "" && config.Workspace.Docker != nil && config.Workspace.Docker.Registry != "" {
		registry = config.Workspace.Docker.Registry
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
			if buildPush {
				targets = append(targets, fmt.Sprintf("//backend/services/%s:image", service))
			} else {
				targets = append(targets, serviceToTarget(service))
			}
		}
	} else if buildService != "" {
		// Build from --service flag
		if buildPush {
			targets = append(targets, fmt.Sprintf("//backend/services/%s:image", buildService))
		} else {
			targets = append(targets, serviceToTarget(buildService))
		}
	} else {
		// Build everything
		if buildPush {
			// Find all image targets
			targets = append(targets, "//backend/services/...:image")
		} else {
			targets = append(targets, "//...")
		}
	}

	// Build Bazel args with environment config
	var bazelFlags []string
	bazelFlags = append(bazelFlags, fmt.Sprintf("--config=%s", buildConfig))

	// CI mode flags
	if buildCI {
		bazelFlags = append(bazelFlags, "--noshow_progress", "--color=no")
	}

	// Add remote cache if configured (removed - now handled per-project)
	// Auto-detect GitHub Actions cache
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		// GitHub Actions automatically mounts ~/.cache/bazel via actions/cache
		// Bazel will use local cache automatically
		if buildVerbose {
			fmt.Println("  ℹ️  Detected GitHub Actions - using local cache from actions/cache")
		}
	}

	// Parallel workers (removed - Bazel auto-detects optimal parallelism)

	// Add platforms for multi-arch builds
	if buildPush && buildPlatforms != "" {
		// Convert to Bazel platform format
		platforms := strings.Split(buildPlatforms, ",")
		for _, platform := range platforms {
			platform = strings.TrimSpace(platform)
			bazelPlatform := convertToBazelPlatform(platform)
			if bazelPlatform != "" {
				bazelFlags = append(bazelFlags, fmt.Sprintf("--platforms=%s", bazelPlatform))
			}
		}
		if buildVerbose {
			fmt.Printf("  ℹ️  Building for platforms: %s\n", buildPlatforms)
		}
	}

	// Add registry define for Docker builds
	if buildPush && registry != "" {
		bazelFlags = append(bazelFlags, fmt.Sprintf("--define=REGISTRY=%s", registry))
		if buildVerbose {
			fmt.Printf("  ℹ️  Using registry: %s\n", registry)
		}
	}

	// Show user-friendly message
	configEmoji := map[string]string{
		"local": "🏠",
		"dev":   "🔧",
		"prod":  "🚀",
	}
	emoji := configEmoji[buildConfig]

	if buildCI {
		emoji = "🤖"
	}

	if buildVerbose {
		fmt.Printf("%s Building with config '%s': %s\n", emoji, buildConfig, strings.Join(targets, ", "))
	} else {
		serviceNames := extractServiceNames(args)
		if len(serviceNames) == 0 {
			if buildPush {
				fmt.Printf("%s Building and pushing images [%s]...\n", emoji, buildConfig)
			} else {
				fmt.Printf("%s Building all services [%s]...\n", emoji, buildConfig)
			}
		} else {
			if buildPush {
				fmt.Printf("%s Building and pushing: %s [%s]\n", emoji, strings.Join(serviceNames, ", "), buildConfig)
			} else {
				fmt.Printf("%s Building: %s [%s]\n", emoji, strings.Join(serviceNames, ", "), buildConfig)
			}
		}
	}

	// Execute build via Bazel
	if err := executor.BuildWithFlags(ctx, bazelFlags, targets); err != nil {
		// Translate Bazel error to user-friendly message
		translator := bazel.NewErrorTranslator()
		friendlyError := translator.Translate(err.Error())
		return fmt.Errorf("❌ Build failed:\n%s", friendlyError)
	}

	// Push images if requested
	if buildPush {
		if registry == "" {
			return fmt.Errorf("❌ Registry not configured. Add 'build.registry' to forge.json or use --registry flag")
		}

		fmt.Println("\n📤 Pushing images to registry...")

		// Get git commit SHA for tagging
		gitSHA := getGitCommitSHA()
		if gitSHA == "" {
			gitSHA = "latest"
		}

		// Push each built image
		for _, target := range targets {
			imageName := extractImageName(target, args)
			if imageName == "" {
				continue
			}

			imageTag := fmt.Sprintf("%s/%s:%s", registry, imageName, gitSHA)

			// Tag and push
			if err := pushImage(imageName, imageTag); err != nil {
				return fmt.Errorf("❌ Failed to push %s: %w", imageName, err)
			}

			if !buildCI {
				fmt.Printf("  ✅ Pushed: %s\n", imageTag)
			}
		}
	}

	if buildCI {
		fmt.Println("✅ Build completed")
	} else {
		fmt.Println("✅ Build completed successfully!")
	}
	return nil
}

// serviceToTarget converts a service name to a Bazel target.
func serviceToTarget(serviceName string) string {
	// Convert service name to Bazel target
	// Example: "api-server" -> "//backend/services/api-server/..."
	return fmt.Sprintf("//backend/services/%s/...", serviceName)
}

// extractServiceNames extracts user-friendly service names from args.
func extractServiceNames(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	return args
}

// extractImageName extracts the service name from a Bazel target for Docker image tagging.
func extractImageName(target string, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	// Extract from target: //backend/services/api-server:image -> api-server
	parts := strings.Split(target, "/")
	for i, part := range parts {
		if part == "services" && i+1 < len(parts) {
			serviceName := parts[i+1]
			serviceName = strings.Split(serviceName, ":")[0]
			return serviceName
		}
	}
	return ""
}

// getGitCommitSHA returns the current git commit SHA.
func getGitCommitSHA() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// pushImage tags and pushes a Docker image to the registry.
func pushImage(imageName, imageTag string) error {
	// For Bazel-built images, they're already in the local Docker daemon
	// We need to tag and push them

	// Tag the image
	tagCmd := exec.Command("docker", "tag", fmt.Sprintf("bazel/%s:image", imageName), imageTag)
	if err := tagCmd.Run(); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}

	// Push the image
	pushCmd := exec.Command("docker", "push", imageTag)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	return nil
}

// convertToBazelPlatform converts Docker platform format to Bazel platform format.
func convertToBazelPlatform(platform string) string {
	platformMap := map[string]string{
		"linux/amd64": "@io_bazel_rules_go//go/toolchain:linux_amd64",
		"linux/arm64": "@io_bazel_rules_go//go/toolchain:linux_arm64",
	}
	if bazelPlatform, ok := platformMap[platform]; ok {
		return bazelPlatform
	}
	return ""
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
