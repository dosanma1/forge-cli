package cmd

import (
	"context"
	"fmt"

	"github.com/dosanma1/forge-cli/internal/bazel"
	"github.com/spf13/cobra"
)

var (
	setupVerbose bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install Bazel and dependencies",
	Long: `Install Bazel (via Bazelisk) and other required dependencies.

This command will:
  - Install Bazelisk to ~/.forge/bazel/
  - Verify the installation
  - Check Bazel version

Examples:
  forge setup           # Install all dependencies
  forge setup --verbose # Show detailed installation output`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolVarP(&setupVerbose, "verbose", "v", false, "Show detailed output")
}

func runSetup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	installer := bazel.NewInstaller(setupVerbose)

	// Check if already installed
	if installer.IsInstalled() {
		fmt.Println("✅ Bazel is already installed")

		// Show version
		version, err := installer.GetVersion(ctx)
		if err == nil {
			fmt.Printf("   Version: %s\n", version)
		}

		fmt.Println("\nRun 'forge setup --update' to check for updates")
		return nil
	}

	// Install Bazel
	fmt.Println("📦 Installing Bazel (via Bazelisk)...")
	if err := installer.Install(ctx); err != nil {
		return fmt.Errorf("failed to install bazel: %w", err)
	}

	return nil
}
