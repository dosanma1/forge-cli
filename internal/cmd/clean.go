package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	cleanCache bool
	cleanDeep  bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean build artifacts and caches",
	Long: `Clean build artifacts and caches in the workspace.

Use --cache to remove project-local caches (.forge/cache, .angular/cache) and run bazel clean --expunge.
Use --deep to additionally remove global caches (~/.cache/bazel, ~/go/pkg/mod/cache, ~/.npm) with confirmation.`,
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanCache, "cache", false, "Remove all caches (project-local and Bazel)")
	cleanCmd.Flags().BoolVar(&cleanDeep, "deep", false, "Remove global caches (requires confirmation)")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("not in a forge workspace: %w", err)
	}

	if !cleanCache && !cleanDeep {
		return fmt.Errorf("no cleaning operation specified. Use --cache or --deep")
	}

	if cleanCache {
		if err := cleanProjectCaches(workspaceRoot); err != nil {
			return err
		}

		if err := cleanBazelCache(workspaceRoot); err != nil {
			return err
		}
	}

	if cleanDeep {
		if err := cleanGlobalCaches(); err != nil {
			return err
		}
	}

	fmt.Println("✅ Clean completed successfully")
	return nil
}

func cleanProjectCaches(workspaceRoot string) error {
	caches := []string{
		filepath.Join(workspaceRoot, ".forge", "cache"),
		filepath.Join(workspaceRoot, "frontend", ".angular", "cache"),
	}

	for _, cache := range caches {
		if _, err := os.Stat(cache); err == nil {
			fmt.Printf("🗑️  Removing %s...\n", cache)
			if err := os.RemoveAll(cache); err != nil {
				return fmt.Errorf("failed to remove %s: %w", cache, err)
			}
			fmt.Printf("   ✓ Removed %s\n", cache)
		}
	}

	return nil
}

func cleanBazelCache(workspaceRoot string) error {
	fmt.Println("🗑️  Running bazel clean --expunge...")

	cmd := exec.Command("bazel", "clean", "--expunge")
	cmd.Dir = workspaceRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bazel clean failed: %w", err)
	}

	fmt.Println("   ✓ Bazel cache cleaned")
	return nil
}

func cleanGlobalCaches() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	globalCaches := []string{
		filepath.Join(homeDir, ".cache", "bazel"),
		filepath.Join(homeDir, "go", "pkg", "mod", "cache"),
		filepath.Join(homeDir, ".npm"),
	}

	// Show what will be deleted
	fmt.Println("\n⚠️  Deep clean will remove the following global caches:")
	for _, cache := range globalCaches {
		if info, err := os.Stat(cache); err == nil && info.IsDir() {
			fmt.Printf("   - %s\n", cache)
		}
	}

	// Confirm with user
	fmt.Print("\nThis will free disk space but may slow down future builds. Continue? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Remove global caches
	for _, cache := range globalCaches {
		if _, err := os.Stat(cache); err == nil {
			fmt.Printf("🗑️  Removing %s...\n", cache)
			if err := os.RemoveAll(cache); err != nil {
				fmt.Printf("   ⚠️  Failed to remove %s: %v\n", cache, err)
			} else {
				fmt.Printf("   ✓ Removed %s\n", cache)
			}
		}
	}

	return nil
}
