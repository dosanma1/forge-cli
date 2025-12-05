package bazel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Executor handles Bazel command execution.
type Executor struct {
	workspaceRoot string
	bazelPath     string
	verbose       bool
}

// NewExecutor creates a new Bazel executor.
func NewExecutor(workspaceRoot string, verbose bool) (*Executor, error) {
	// Find bazelisk or bazel
	bazelPath, err := findBazel()
	if err != nil {
		return nil, fmt.Errorf("bazel not found: %w (run 'forge setup' to install)", err)
	}

	return &Executor{
		workspaceRoot: workspaceRoot,
		bazelPath:     bazelPath,
		verbose:       verbose,
	}, nil
}

// Build executes a Bazel build command.
// Build executes a Bazel build command.
func (e *Executor) Build(ctx context.Context, targets []string) error {
	args := []string{"build"}
	args = append(args, targets...)
	return e.execute(ctx, args)
}

// BuildWithFlags executes a Bazel build command with custom flags.
func (e *Executor) BuildWithFlags(ctx context.Context, flags []string, targets []string) error {
	args := []string{"build"}
	args = append(args, flags...)
	args = append(args, targets...)
	return e.execute(ctx, args)
}

// Test executes a Bazel test command.
func (e *Executor) Test(ctx context.Context, targets []string) error {
	args := []string{"test", "--test_output=errors"}
	args = append(args, targets...)
	return e.execute(ctx, args)
}

// TestWithFlags executes a Bazel test command with custom flags.
func (e *Executor) TestWithFlags(ctx context.Context, flags []string, targets []string) error {
	args := []string{"test"}
	args = append(args, flags...)
	args = append(args, targets...)
	return e.execute(ctx, args)
}

// Run executes a Bazel run command.
func (e *Executor) Run(ctx context.Context, target string) error {
	return e.execute(ctx, []string{"run", target})
}

// Query executes a Bazel query.
func (e *Executor) Query(ctx context.Context, query string) ([]string, error) {
	cmd := exec.CommandContext(ctx, e.bazelPath, "query", query)
	cmd.Dir = e.workspaceRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines, nil
}

// execute runs a Bazel command with proper output handling.
func (e *Executor) execute(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, e.bazelPath, args...)
	cmd.Dir = e.workspaceRoot

	if e.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// Capture output for progress parsing
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bazel command failed: %w", err)
	}

	return nil
}

// findBazel locates bazelisk or bazel binary.
func findBazel() (string, error) {
	// Try bazelisk first (recommended)
	if path, err := exec.LookPath("bazelisk"); err == nil {
		return path, nil
	}

	// Fall back to bazel
	if path, err := exec.LookPath("bazel"); err == nil {
		return path, nil
	}

	// Check forge-managed installation
	forgeHome := os.Getenv("HOME")
	forgeBazel := filepath.Join(forgeHome, ".forge", "bazel", "bin", "bazelisk")
	if _, err := os.Stat(forgeBazel); err == nil {
		return forgeBazel, nil
	}

	return "", fmt.Errorf("bazel not found in PATH or ~/.forge/bazel/")
}

// Version returns the Bazel version.
func (e *Executor) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, e.bazelPath, "version")

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}
