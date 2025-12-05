package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/dosanma1/forge-cli/internal/bazel"
	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	testVerbose  bool
	testService  string
	testCI       bool
	testConfig   string
	testCoverage bool
)

var testCmd = &cobra.Command{
	Use:   "test [service...]",
	Short: "Run tests using Bazel",
	Long: `Run tests for one or more services using Bazel.

Examples:
  forge test                       # Run all tests
  forge test api-server            # Test specific service
  forge test --verbose             # Show detailed test output
  forge test --ci                  # Run in CI mode (no cache, fail fast)
  forge test --coverage            # Generate coverage report
  forge test --config=dev          # Test with dev configuration`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.Flags().BoolVarP(&testVerbose, "verbose", "v", false, "Show detailed test output")
	testCmd.Flags().StringVarP(&testService, "service", "s", "", "Test specific service")
	testCmd.Flags().BoolVar(&testCI, "ci", false, "Run in CI mode (no cache, fail fast)")
	testCmd.Flags().StringVarP(&testConfig, "config", "c", "local", "Test configuration (local|dev|prod)")
	testCmd.Flags().BoolVar(&testCoverage, "coverage", false, "Generate coverage report")
}

func runTest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate config
	validConfigs := map[string]bool{"local": true, "dev": true, "prod": true}
	if !validConfigs[testConfig] {
		return fmt.Errorf("invalid config: %s (must be local, dev, or prod)", testConfig)
	}

	// Get workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("not in a forge workspace: %w", err)
	}

	// Load workspace config (not used currently but may be needed for future test configuration)
	_, err = workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Create Bazel executor
	executor, err := bazel.NewExecutor(workspaceRoot, testVerbose)
	if err != nil {
		return err
	}

	// Determine what to test
	var targets []string

	if len(args) > 0 {
		// Test specific services
		for _, service := range args {
			targets = append(targets, serviceToTarget(service))
		}
	} else if testService != "" {
		// Test from --service flag
		targets = append(targets, serviceToTarget(testService))
	} else {
		// Test everything
		targets = append(targets, "//...")
	}

	// Build test flags
	var testFlags []string
	testFlags = append(testFlags, fmt.Sprintf("--config=%s", testConfig))

	// Add CI flags
	if testCI {
		testFlags = append(testFlags, "--config=ci")
		testFlags = append(testFlags, "--nocache_test_results")
		testFlags = append(testFlags, "--test_output=all")
	}

	// Add coverage flags
	if testCoverage {
		testFlags = append(testFlags, "--coverage_report_generator=@bazel_tools//tools/test/CoverageOutputGenerator/java/com/google/devtools/coverageoutputgenerator:Main")
		testFlags = append(testFlags, "--combined_report=lcov")
		testFlags = append(testFlags, "--instrumentation_filter=//...")
	}

	// Remote cache removed - Bazel uses local cache by default

	// Show user-friendly message
	serviceNames := extractServiceNames(args)
	if len(serviceNames) == 0 {
		fmt.Printf("🧪 Running all tests [%s]...\n", testConfig)
	} else {
		fmt.Printf("🧪 Testing: %s [%s]\n", strings.Join(serviceNames, ", "), testConfig)
	}

	if testCoverage {
		fmt.Println("  📊 Coverage report will be generated")
	}

	// Execute tests with flags
	if err := executor.TestWithFlags(ctx, testFlags, targets); err != nil {
		// Translate Bazel error to user-friendly message
		translator := bazel.NewErrorTranslator()
		friendlyError := translator.Translate(err.Error())
		return fmt.Errorf("❌ Tests failed:\n%s", friendlyError)
	}

	fmt.Println("✅ All tests passed!")

	if testCoverage {
		fmt.Println("📊 Coverage report: bazel-out/_coverage/_coverage_report.dat")
	}

	return nil
}
