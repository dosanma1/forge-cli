package cmd

import (
	"fmt"
	"os"

	"github.com/dosanma1/forge-cli/internal/sync"
	"github.com/dosanma1/forge-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	syncDryRun bool
	syncYes    bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize Bazel configuration with forge.json",
	Long: `Regenerates all Bazel configuration files (MODULE.bazel, BUILD.bazel) based on forge.json.

This command will:
  1. Delete all existing BUILD.bazel and MODULE.bazel files
  2. Regenerate MODULE.bazel based on detected languages
  3. Auto-discover and generate BUILD.bazel for all Go packages
  4. Regenerate BUILD.bazel for services defined in forge.json

Use this to recover from broken configurations or when you manually add packages.`,
	Example: `  # Preview changes without applying
  forge sync --dry-run

  # Apply changes without confirmation
  forge sync --yes

  # Interactive mode (default)
  forge sync`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Preview changes without applying them")
	syncCmd.Flags().BoolVarP(&syncYes, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	workspaceRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create syncer
	syncer, err := sync.NewSyncer(workspaceRoot, syncDryRun)
	if err != nil {
		return err
	}

	// Confirm with user unless --yes or --dry-run
	if !syncYes && !syncDryRun {
		fmt.Println("⚠️  This will delete and regenerate all Bazel files.")
		confirm, err := ui.AskConfirm("Continue?", false)
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Operation cancelled")
			return nil
		}
	}

	// Run sync
	fmt.Println("🔄 Synchronizing workspace...")
	report, err := syncer.Sync()
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Print report
	if syncDryRun {
		fmt.Println("\n📋 Dry run results:")
	} else {
		fmt.Println("\n✅ Sync completed!")
	}

	if len(report.DeletedFiles) > 0 {
		fmt.Printf("\n🗑️  Deleted %d files:\n", len(report.DeletedFiles))
		for _, file := range report.DeletedFiles {
			fmt.Printf("   - %s\n", file)
		}
	}

	if len(report.CreatedFiles) > 0 {
		fmt.Printf("\n📝 Created %d files:\n", len(report.CreatedFiles))
		for _, file := range report.CreatedFiles {
			fmt.Printf("   + %s\n", file)
		}
	}

	if len(report.Errors) > 0 {
		fmt.Printf("\n❌ Encountered %d errors:\n", len(report.Errors))
		for _, err := range report.Errors {
			fmt.Printf("   ! %v\n", err)
		}
	}

	if syncDryRun {
		fmt.Println("\n💡 Run without --dry-run to apply changes")
	}

	return nil
}
