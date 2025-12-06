package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/bazel"
	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	buildVerbose      bool
	buildEnv          string
	buildService      string
	buildPush         bool
	buildCI           bool
	buildRegistry     string
	buildPlatforms    string
	buildNoSync       bool
	buildFrontendOnly bool
	buildServicesOnly bool
)

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build services using Bazel",
	Long: `Build one or more services in your workspace using Bazel.

Bazel automatically detects changed files and only rebuilds affected targets.
Use --push to build and push Docker images to the registry.

Environments (--env):
  local      - Fast builds, no optimization (default)
  development - Some optimization, source maps
  production  - Full optimization, minified, no debug info

Examples:
  forge build                            # Build all (services + frontend, local env)
  forge build --env=production           # Build all for production
  forge build --push                     # Build and push Docker images
  forge build --ci                       # CI mode (clean logs, production env)
  forge build api-server                 # Build specific service
  forge build api-server worker          # Build multiple services
  forge build --env=development --verbose # Dev build with details
  forge build --push --platforms=linux/amd64,linux/arm64  # Multi-arch
  forge build --frontend-only            # Build only frontend apps
  forge build --services-only            # Build only backend services
  forge build --frontend-only web-app    # Build specific frontend app`,
	RunE: runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().BoolVarP(&buildVerbose, "verbose", "v", false, "Show detailed build output")
	buildCmd.Flags().StringVarP(&buildEnv, "env", "e", "production", "Build environment (local|development|production)")
	buildCmd.Flags().StringVarP(&buildService, "service", "s", "", "Build specific service")
	buildCmd.Flags().BoolVar(&buildPush, "push", false, "Build and push Docker images to registry")
	buildCmd.Flags().BoolVar(&buildCI, "ci", false, "CI mode (clean logs, prod config, no progress)")
	buildCmd.Flags().StringVar(&buildRegistry, "registry", "", "Override Docker registry from forge.json")
	buildCmd.Flags().StringVar(&buildPlatforms, "platforms", "linux/amd64", "Target platforms for multi-arch builds")
	buildCmd.Flags().BoolVar(&buildNoSync, "no-sync", false, "Skip automatic dependency synchronization")
	buildCmd.Flags().BoolVar(&buildFrontendOnly, "frontend-only", false, "Build only frontend applications")
	buildCmd.Flags().BoolVar(&buildServicesOnly, "services-only", false, "Build only backend services (skip frontend)")
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// CI mode overrides
	if buildCI {
		buildEnv = "prod"
		buildVerbose = false
	}

	// Get workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("not in a forge workspace: %w", err)
	}

	// Auto-sync dependencies unless --no-sync is specified
	if !buildNoSync {
		fmt.Println("🔄 Syncing dependencies...")
		if err := runSyncQuiet(workspaceRoot); err != nil {
			fmt.Printf("⚠️  Warning: Dependency sync failed: %v\n", err)
			fmt.Println("   Build may fail. Try running 'forge sync' manually")
			fmt.Println("   Or use --no-sync to skip this step\n")
		} else {
			fmt.Println("✓ Dependencies synchronized\n")
		}
	}

	// Load workspace config
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Validate environment exists in forge.json
	if _, exists := config.Environments[buildEnv]; !exists {
		availableEnvs := []string{}
		for env := range config.Environments {
			availableEnvs = append(availableEnvs, env)
		}
		return fmt.Errorf("environment '%s' not found in forge.json. Available: %v", buildEnv, availableEnvs)
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

	// Check for mutually exclusive flags
	if buildFrontendOnly && buildServicesOnly {
		return fmt.Errorf("cannot use --frontend-only and --services-only together")
	}

	if len(args) > 0 {
		// Build specific services or frontend apps
		if buildFrontendOnly {
			// Build specific frontend apps
			for _, appName := range args {
				targets = append(targets, fmt.Sprintf("//frontend/projects/%s:build", appName))
			}
		} else {
			// Build specific services
			for _, service := range args {
				if buildPush {
					targets = append(targets, fmt.Sprintf("//backend/services/%s:image", service))
				} else {
					targets = append(targets, serviceToTarget(service))
				}
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
		// Build everything (or filtered subset)
		if buildFrontendOnly {
			// Build all frontend apps
			targets = append(targets, "//frontend/...")
		} else if buildServicesOnly {
			// Build only backend services
			if buildPush {
				targets = append(targets, "//backend/services/...:image")
			} else {
				targets = append(targets, "//backend/...")
			}
		} else {
			// Build everything (services + frontend)
			if buildPush {
				// Push only applies to services with images
				targets = append(targets, "//backend/services/...:image")
			} else {
				targets = append(targets, "//...")
			}
		}
	}

	// Build Bazel args with environment config
	var bazelFlags []string

	// Resolve Angular configuration for frontend builds
	angularConfig := resolveAngularConfig(config, buildEnv, buildVerbose)

	// Pass resolved Angular config to Bazel for frontend builds
	bazelFlags = append(bazelFlags, fmt.Sprintf("--define=ENV=%s", angularConfig))

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
	envEmoji := map[string]string{
		"local":       "🏠",
		"development": "🔧",
		"production":  "🚀",
	}
	emoji := envEmoji[buildEnv]

	if buildCI {
		emoji = "🤖"
	}

	if buildVerbose {
		fmt.Printf("%s Building with environment '%s': %s\n", emoji, buildEnv, strings.Join(targets, ", "))
	} else {
		serviceNames := extractServiceNames(args)
		if len(serviceNames) == 0 {
			if buildPush {
				fmt.Printf("%s Building and pushing images [%s]...\n", emoji, buildEnv)
			} else {
				fmt.Printf("%s Building all services [%s]...\n", emoji, buildEnv)
			}
		} else {
			if buildPush {
				fmt.Printf("%s Building and pushing: %s [%s]\n", emoji, strings.Join(serviceNames, ", "), buildEnv)
			} else {
				fmt.Printf("%s Building: %s [%s]\n", emoji, strings.Join(serviceNames, ", "), buildEnv)
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

	// Traverse up the directory tree
	for {
		// Check for forge.json
		forgeJsonPath := filepath.Join(dir, "forge.json")
		if _, err := os.Stat(forgeJsonPath); err == nil {
			return dir, nil
		}

		// Check for MODULE.bazel (fallback)
		moduleBazelPath := filepath.Join(dir, "MODULE.bazel")
		if _, err := os.Stat(moduleBazelPath); err == nil {
			return dir, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("not a forge workspace (no forge.json found)")
}

// runSyncQuiet runs sync without verbose output
func runSyncQuiet(workspaceDir string) error {
	// Load workspace config
	config, err := workspace.LoadConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Run go mod tidy at workspace root first
	if err := syncWorkspaceRootQuiet(workspaceDir); err != nil {
		return err
	}

	// Run go mod tidy for all services
	if err := syncGoServicesQuiet(workspaceDir, config); err != nil {
		return err
	}

	// Run npm install for frontend if exists
	if err := syncFrontendQuiet(workspaceDir, config); err != nil {
		return err
	}

	return nil
}

// syncWorkspaceRootQuiet runs go mod tidy at workspace root quietly
func syncWorkspaceRootQuiet(workspaceDir string) error {
	goModPath := filepath.Join(workspaceDir, "go.mod")

	// Check if workspace has a go.mod
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = workspaceDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed at workspace root: %w", err)
	}

	return nil
}

// syncGoServicesQuiet runs go mod tidy quietly
func syncGoServicesQuiet(workspaceDir string, config *workspace.Config) error {
	servicesDir := filepath.Join(workspaceDir, "backend", "services")
	if _, err := os.Stat(servicesDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		return fmt.Errorf("failed to read services directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		serviceName := entry.Name()
		serviceDir := filepath.Join(servicesDir, serviceName)
		goModPath := filepath.Join(serviceDir, "go.mod")

		if _, err := os.Stat(goModPath); os.IsNotExist(err) {
			continue
		}

		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = serviceDir

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go mod tidy failed for %s: %w", serviceName, err)
		}
	}

	return nil
}

// syncFrontendQuiet runs npm install quietly
func syncFrontendQuiet(workspaceDir string, config *workspace.Config) error {
	frontendDir := filepath.Join(workspaceDir, "frontend")
	packageJsonPath := filepath.Join(frontendDir, "package.json")

	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("npm", "install")
	cmd.Dir = frontendDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install failed: %w", err)
	}

	return nil
}

// resolveAngularConfig resolves the Angular configuration name for a given environment
// It checks all Angular projects for their environmentMapper and returns the mapped config
// Defaults to "production" if not mapped
func resolveAngularConfig(config *workspace.Config, env string, verbose bool) string {
	// Find first Angular project with environmentMapper
	for name, project := range config.Projects {
		if project.Type == workspace.ProjectTypeAngular && project.Build != nil && project.Build.EnvironmentMapper != nil {
			if angularConfig, ok := project.Build.EnvironmentMapper[env]; ok {
				if verbose {
					fmt.Printf("  ℹ️  Angular project '%s': using config '%s' for environment '%s'\n", name, angularConfig, env)
				}
				return angularConfig
			}
		}
	}

	// Default to production if no mapping found
	if verbose {
		fmt.Printf("  ℹ️  No environment mapping found, defaulting to 'production'\n")
	}
	return "production"
}
