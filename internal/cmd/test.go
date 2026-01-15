package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

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
	startTime := time.Now()

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

	// Determine what to test
	var targets []string
	if len(args) > 0 {
		// Test specific projects
		for _, projectName := range args {
			target, err := projectToTestTarget(config, projectName)
			if err != nil {
				return err
			}
			targets = append(targets, target)
		}
	} else if testService != "" {
		target, err := projectToTestTarget(config, testService)
		if err != nil {
			return err
		}
		targets = append(targets, target)
	} else {
		targets = append(targets, "//...")
	}

	// Build test command
	cmdArgs := []string{"test"}
	cmdArgs = append(cmdArgs, targets...)
	cmdArgs = append(cmdArgs, "--test_output=errors")

	if testVerbose {
		cmdArgs = append(cmdArgs, "--test_output=all")
		cmdArgs = append(cmdArgs, "--test_arg=-test.v")
		cmdArgs = append(cmdArgs, "--nocache_test_results")
	}

	if testCoverage {
		cmdArgs = append(cmdArgs, "--coverage_report_generator=@bazel_tools//tools/test/CoverageOutputGenerator/java/com/google/devtools/coverageoutputgenerator:Main")
		cmdArgs = append(cmdArgs, "--combined_report=lcov")
		cmdArgs = append(cmdArgs, "--instrumentation_filter=//...")
	}

	// Show header
	fmt.Printf("\n🧪 Running tests...\n\n")

	// Run bazel test
	bazelCmd := exec.Command("bazel", cmdArgs...)
	bazelCmd.Dir = workspaceRoot

	var results testResults
	var output []byte

	if testVerbose {
		// In verbose mode, stream output directly to terminal
		bazelCmd.Stdout = cmd.OutOrStdout()
		bazelCmd.Stderr = cmd.ErrOrStderr()
		err = bazelCmd.Run()
		duration := time.Since(startTime)
		if err != nil {
			fmt.Printf("\n❌ Tests failed (see output above)\n")
			fmt.Printf("   Total time: %.1fs\n", duration.Seconds())
			return fmt.Errorf("test execution failed")
		}
		fmt.Printf("\n✅ Tests passed!\n")
		fmt.Printf("   Total time: %.1fs\n", duration.Seconds())
		return nil
	}

	// In non-verbose mode, capture output for parsing
	output, err = bazelCmd.CombinedOutput()
	outputStr := string(output)

	// Parse test results
	results = parseTestResults(outputStr)

	// Print summary
	duration := time.Since(startTime)
	printTestSummary(results, duration)

	// Show failed test details
	if len(results.failed) > 0 {
		fmt.Println("\n❌ Failed tests:")
		for _, fail := range results.failed {
			fmt.Printf("  • %s\n", fail.name)
			if fail.logPath != "" {
				fmt.Printf("    Log: %s\n", fail.logPath)
			}
		}

		// Suggest fixes
		fmt.Println("\n💡 Tips:")
		if containsBazelError(outputStr) {
			fmt.Println("  • Some tests failed due to Bazel issues. Try running: forge sync")
		}
		fmt.Println("  • View detailed logs at the paths above")
		fmt.Println("  • Run with --verbose to see full output")

		return fmt.Errorf("%d test(s) failed", len(results.failed))
	}

	if testCoverage {
		fmt.Println("\n📊 Coverage report: bazel-out/_coverage/_coverage_report.dat")
	}

	return nil
}

type testResults struct {
	passed []testResult
	failed []testResult
	total  int
	cached int
}

type testResult struct {
	name     string
	duration string
	logPath  string
}

func parseTestResults(output string) testResults {
	results := testResults{}

	// Parse individual test results
	passedRe := regexp.MustCompile(`^(//[^\s]+)\s+.*PASSED.*in\s+(\S+)`)
	failedRe := regexp.MustCompile(`^(//[^\s]+)\s+.*FAILED.*in\s+(\S+)`)
	logRe := regexp.MustCompile(`\s+(/[^\s]+/test\.log)`)

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if matches := passedRe.FindStringSubmatch(line); matches != nil {
			results.passed = append(results.passed, testResult{
				name:     matches[1],
				duration: matches[2],
			})
			if strings.Contains(line, "cached") {
				results.cached++
			}
		} else if matches := failedRe.FindStringSubmatch(line); matches != nil {
			tr := testResult{
				name:     matches[1],
				duration: matches[2],
			}
			// Check next line for log path
			if i+1 < len(lines) {
				if logMatches := logRe.FindStringSubmatch(lines[i+1]); logMatches != nil {
					tr.logPath = logMatches[1]
				}
			}
			results.failed = append(results.failed, tr)
		}
	}

	results.total = len(results.passed) + len(results.failed)
	return results
}

func printTestSummary(results testResults, duration time.Duration) {
	fmt.Println(strings.Repeat("─", 50))

	if len(results.failed) == 0 {
		fmt.Printf("✅ All tests passed! (%d/%d)\n", len(results.passed), results.total)
	} else {
		fmt.Printf("📊 Test Results: %d passed, %d failed (total: %d)\n",
			len(results.passed), len(results.failed), results.total)
	}

	if results.cached > 0 {
		fmt.Printf("   %d test(s) cached\n", results.cached)
	}

	fmt.Printf("   Total time: %.1fs\n", duration.Seconds())
}

func containsBazelError(output string) bool {
	bazelErrors := []string{
		"no such target",
		"no such package",
		"missing dependencies",
		"cannot load",
		"BUILD file",
	}

	lower := strings.ToLower(output)
	for _, errPattern := range bazelErrors {
		if strings.Contains(lower, strings.ToLower(errPattern)) {
			return true
		}
	}
	return false
}

func projectToTestTarget(config *workspace.Config, projectName string) (string, error) {
	// If already a Bazel target, return as-is
	if strings.HasPrefix(projectName, "//") {
		return projectName, nil
	}

	// Look up project in config
	project, exists := config.Projects[projectName]
	if !exists {
		return "", fmt.Errorf("project %q not found in forge.json", projectName)
	}

	// Check if project has a test configuration
	if project.Architect != nil && project.Architect.Test != nil && project.Architect.Test.Options != nil {
		if target, ok := project.Architect.Test.Options["target"].(string); ok {
			// Construct full target from root + target
			return fmt.Sprintf("//%s%s", project.Root, target), nil
		}
	}

	// Check if project has build configuration with target (fallback)
	if project.Architect != nil && project.Architect.Build != nil && project.Architect.Build.Options != nil {
		if target, ok := project.Architect.Build.Options["target"].(string); ok {
			// Construct full target from root + target
			return fmt.Sprintf("//%s%s", project.Root, target), nil
		}
	}

	// Default: use root + /...
	return fmt.Sprintf("//%s/...", project.Root), nil
}
