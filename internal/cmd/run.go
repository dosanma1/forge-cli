package cmd

import (
	"context"
	"fmt"

	"github.com/dosanma1/forge-cli/internal/bazel"
	"github.com/spf13/cobra"
)

var (
	runVerbose bool
)

var runCmd = &cobra.Command{
	Use:   "run <service>",
	Short: "Run a service locally",
	Long: `Run a service in your workspace locally.

Examples:
  forge run api-server        # Run api-server service
  forge run --verbose worker  # Run with detailed output`,
	Args: cobra.ExactArgs(1),
	RunE: runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVarP(&runVerbose, "verbose", "v", false, "Show detailed output")
}

func runRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	service := args[0]

	// Get workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("not in a forge workspace: %w", err)
	}

	// Create Bazel executor
	executor, err := bazel.NewExecutor(workspaceRoot, runVerbose)
	if err != nil {
		return err
	}

	// Convert service to target
	target := serviceToTarget(service)

	fmt.Printf("🚀 Running service: %s\n", service)

	// Execute run
	if err := executor.Run(ctx, target); err != nil {
		// Translate Bazel error to user-friendly message
		translator := bazel.NewErrorTranslator()
		friendlyError := translator.Translate(err.Error())
		return fmt.Errorf("❌ Run failed:\n%s", friendlyError)
	}

	return nil
}
