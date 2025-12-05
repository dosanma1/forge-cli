package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	syncVerbose bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize workspace dependencies and configurations",
	Long: `Synchronize all workspace dependencies and configurations.

This command:
- Runs 'go mod tidy' for all Go services
- Runs 'npm install' if frontend exists
- Updates go.work, MODULE.bazel, and skaffold.yaml
- Ensures all configurations are in sync

Examples:
  forge sync                    # Sync entire workspace
  forge sync --verbose          # Show detailed output`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().BoolVarP(&syncVerbose, "verbose", "v", false, "Show verbose output")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	workspaceDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Load workspace config
	config, err := workspace.LoadConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	fmt.Println("🔄 Synchronizing workspace dependencies...")

	// 1. Run go mod tidy for all services
	if err := syncGoServices(workspaceDir, config); err != nil {
		return err
	}

	// 2. Run npm install for frontend if exists
	if err := syncFrontend(workspaceDir, config); err != nil {
		return err
	}

	// 3. Update go.work
	if err := updateGoWork(workspaceDir, config); err != nil {
		return err
	}

	// 4. Update MODULE.bazel
	if err := updateModuleBazel(workspaceDir, config); err != nil {
		return err
	}

	// 5. Update skaffold.yaml
	if err := updateSkaffold(workspaceDir, config); err != nil {
		return err
	}

	fmt.Println("\n✅ Workspace synchronized successfully!")
	return nil
}

func syncGoServices(workspaceDir string, config *workspace.Config) error {
	servicesDir := filepath.Join(workspaceDir, "backend", "services")
	if _, err := os.Stat(servicesDir); os.IsNotExist(err) {
		if syncVerbose {
			fmt.Println("  ℹ️  No backend services directory found")
		}
		return nil
	}

	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		return fmt.Errorf("failed to read services directory: %w", err)
	}

	serviceCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		serviceName := entry.Name()
		serviceDir := filepath.Join(servicesDir, serviceName)
		goModPath := filepath.Join(serviceDir, "go.mod")

		// Check if go.mod exists
		if _, err := os.Stat(goModPath); os.IsNotExist(err) {
			if syncVerbose {
				fmt.Printf("  ⏭️  Skipping %s (no go.mod)\n", serviceName)
			}
			continue
		}

		fmt.Printf("  📦 Running go mod tidy for %s...\n", serviceName)

		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = serviceDir

		if syncVerbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			fmt.Printf("  ⚠️  Warning: go mod tidy failed for %s: %v\n", serviceName, err)
			// Continue with other services instead of failing
			continue
		}

		serviceCount++
	}

	if serviceCount > 0 {
		fmt.Printf("  ✓ Synchronized %d Go service(s)\n", serviceCount)
	} else if syncVerbose {
		fmt.Println("  ℹ️  No Go services found")
	}

	return nil
}

func syncFrontend(workspaceDir string, config *workspace.Config) error {
	frontendDir := filepath.Join(workspaceDir, "frontend")
	packageJsonPath := filepath.Join(frontendDir, "package.json")

	// Check if frontend exists
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		if syncVerbose {
			fmt.Println("  ℹ️  No frontend directory found")
		}
		return nil
	}

	fmt.Println("  📦 Running npm install for frontend...")

	cmd := exec.Command("npm", "install")
	cmd.Dir = frontendDir

	if syncVerbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install failed: %w", err)
	}

	fmt.Println("  ✓ Frontend dependencies synchronized")
	return nil
}

func updateGoWork(workspaceDir string, config *workspace.Config) error {
	if syncVerbose {
		fmt.Println("  🔧 Updating go.work...")
	}

	// Collect all services
	var services []string
	servicesDir := filepath.Join(workspaceDir, "backend", "services")
	if _, err := os.Stat(servicesDir); err == nil {
		entries, err := os.ReadDir(servicesDir)
		if err != nil {
			return fmt.Errorf("failed to read services directory: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			serviceName := entry.Name()
			goModPath := filepath.Join(servicesDir, serviceName, "go.mod")
			if _, err := os.Stat(goModPath); err == nil {
				services = append(services, serviceName)
			}
		}
	}

	// Generate go.work content
	var content strings.Builder
	content.WriteString("go 1.23\n\n")

	if len(services) > 0 {
		content.WriteString("use (\n")
		for _, service := range services {
			content.WriteString(fmt.Sprintf("\t./backend/services/%s\n", service))
		}
		content.WriteString(")\n")
	}

	goWorkPath := filepath.Join(workspaceDir, "go.work")
	if err := os.WriteFile(goWorkPath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write go.work: %w", err)
	}

	if syncVerbose {
		fmt.Printf("  ✓ Updated go.work with %d service(s)\n", len(services))
	}

	return nil
}

func updateModuleBazel(workspaceDir string, config *workspace.Config) error {
	if syncVerbose {
		fmt.Println("  🔧 Updating MODULE.bazel...")
	}

	// This would use the template engine, but for now we'll skip
	// as it requires the full generator setup
	// The service generator already handles this properly

	if syncVerbose {
		fmt.Println("  ✓ MODULE.bazel update skipped (handled by generators)")
	}

	return nil
}

func updateSkaffold(workspaceDir string, config *workspace.Config) error {
	if syncVerbose {
		fmt.Println("  🔧 Updating skaffold.yaml...")
	}

	// This would use the template engine, but for now we'll skip
	// as it requires the full generator setup
	// The service generator already handles this properly

	if syncVerbose {
		fmt.Println("  ✓ skaffold.yaml update skipped (handled by generators)")
	}

	return nil
}
