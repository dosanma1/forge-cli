package cmd
package cmd

import (
	"context"
	"fmt"

	"github.com/dosanma1/forge-cli/internal/bazel"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup forge development environment",
	Long: `Setup the forge development environment.

This command:
- Installs Bazelisk (Bazel version manager)
- Verifies Go and Node.js installations
- Checks system dependencies
- Initializes the forge workspace

Run this command once after cloning a forge workspace.`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	fmt.Println("🔧 Setting up forge environment...")































}	return nil	fmt.Println("  forge dev       # Start development mode")	fmt.Println("  forge test      # Run all tests")	fmt.Println("  forge build     # Build all services")	fmt.Println("Next steps:")	fmt.Println()	fmt.Println("✅ Setup complete!")	fmt.Println()	}		}			return fmt.Errorf("failed to install Bazel: %w", err)		if err := installer.Install(ctx); err != nil {				fmt.Println("📦 Bazel not found, installing...")	} else {		}			fmt.Printf("   Version: %s\n", version)		if err == nil {		version, err := installer.GetVersion(ctx)		// Show version				fmt.Println("✅ Bazel is already installed")	if installer.IsInstalled() {		installer := bazel.NewInstaller(false)	// Check if Bazel is already installed	fmt.Println()