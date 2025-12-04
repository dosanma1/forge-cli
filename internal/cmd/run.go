package cmd
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
	Long: `Run a service locally for development.

Examples:
  forge run api-server        # Run the api-server service
  forge run worker --verbose  # Run with detailed output`,









































}	return nil	}		return fmt.Errorf("❌ Failed to run service:\n%s", friendlyError)		friendlyError := translator.Translate(err.Error())		translator := bazel.NewErrorTranslator()	if err := executor.Run(ctx, target); err != nil {	// Execute run	fmt.Println()	fmt.Println("   Press Ctrl+C to stop")	fmt.Printf("🚀 Starting %s...\n", serviceName)	target := fmt.Sprintf("//backend/services/%s/cmd/server:server", serviceName)	// Convert service name to Bazel run target	}		return err	if err != nil {	executor, err := bazel.NewExecutor(workspaceRoot, runVerbose)	// Create Bazel executor	}		return fmt.Errorf("not in a forge workspace: %w", err)	if err != nil {	workspaceRoot, err := findWorkspaceRoot()	// Get workspace root	serviceName := args[0]	ctx := context.Background()func runRun(cmd *cobra.Command, args []string) error {}	runCmd.Flags().BoolVarP(&runVerbose, "verbose", "v", false, "Show detailed output")	rootCmd.AddCommand(runCmd)func init() {}	RunE: runRun,	Args: cobra.ExactArgs(1),