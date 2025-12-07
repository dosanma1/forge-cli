package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/ui"
	"github.com/spf13/cobra"
)

var setupHooksCmd = &cobra.Command{
	Use:   "setup-hooks",
	Short: "Setup git hooks and code quality tools",
	Long: `Setup git hooks using Husky, lint-staged, and commitlint.

This will install and configure:
- Husky for git hooks
- lint-staged for pre-commit linting
- commitlint for commit message validation
- Prettier for code formatting
- ESLint for linting

Examples:
  forge setup-hooks`,
	RunE: runSetupHooks,
}

func init() {
	rootCmd.AddCommand(setupHooksCmd)
}

func runSetupHooks(cmd *cobra.Command, args []string) error {
	fmt.Println(ui.TitleStyle.Render("🔧 Setup Git Hooks & Code Quality"))
	fmt.Println()

	// Check if in a workspace
	if _, err := os.Stat("forge.json"); os.IsNotExist(err) {
		return fmt.Errorf("not in a Forge workspace. Run this command from the workspace root")
	}

	// Check for Node.js
	if !isNodeInstalled() {
		return fmt.Errorf("Node.js not found. Please install Node.js first")
	}

	// Check for git
	if !isGitRepo() {
		fmt.Println(ui.WarningStyle.Render("⚠️  Not a git repository. Initializing..."))
		if err := initGit(); err != nil {
			return fmt.Errorf("failed to initialize git: %w", err)
		}
	}

	// Ask what to setup
	_, options, err := ui.AskMultiSelect("Select tools to install:", []string{
		"Husky (git hooks)",
		"lint-staged (pre-commit)",
		"commitlint (commit messages)",
		"Prettier (formatting)",
		"ESLint (linting)",
	}, []int{})
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}

	if len(options) == 0 {
		fmt.Println(ui.WarningStyle.Render("No tools selected"))
		return nil
	}

	setupHusky := contains(options, "Husky (git hooks)")
	setupLintStaged := contains(options, "lint-staged (pre-commit)")
	setupCommitlint := contains(options, "commitlint (commit messages)")
	setupPrettier := contains(options, "Prettier (formatting)")
	setupESLint := contains(options, "ESLint (linting)")

	// Check if package.json exists
	frontendDir := ""

	if !fileExists("package.json") {
		// Look for frontend/package.json
		if fileExists("frontend/package.json") {
			frontendDir = "frontend"
		} else {
			// Create root package.json
			fmt.Println("Creating root package.json...")
			if err := createRootPackageJSON(); err != nil {
				return err
			}
		}
	}

	workDir := "."
	if frontendDir != "" {
		workDir = frontendDir
	}

	// Install packages
	packages := []string{}
	if setupHusky {
		packages = append(packages, "husky")
	}
	if setupLintStaged {
		packages = append(packages, "lint-staged")
	}
	if setupCommitlint {
		packages = append(packages, "@commitlint/cli", "@commitlint/config-conventional")
	}
	if setupPrettier {
		packages = append(packages, "prettier")
	}
	if setupESLint {
		packages = append(packages, "eslint")
	}

	if len(packages) > 0 {
		fmt.Printf("\n%s\n", ui.SubtitleStyle.Render("Installing packages..."))
		if err := installNpmPackages(workDir, packages, true); err != nil {
			return err
		}
	}

	// Setup Husky
	if setupHusky {
		fmt.Printf("\n%s\n", ui.SubtitleStyle.Render("Setting up Husky..."))
		if err := setupHuskyHooks(workDir); err != nil {
			return err
		}
	}

	// Setup lint-staged
	if setupLintStaged && setupHusky {
		fmt.Printf("\n%s\n", ui.SubtitleStyle.Render("Configuring lint-staged..."))
		if err := createLintStagedConfig(); err != nil {
			return err
		}
	}

	// Setup commitlint
	if setupCommitlint {
		fmt.Printf("\n%s\n", ui.SubtitleStyle.Render("Configuring commitlint..."))
		if err := createCommitlintConfig(); err != nil {
			return err
		}
	}

	// Setup Prettier
	if setupPrettier {
		fmt.Printf("\n%s\n", ui.SubtitleStyle.Render("Configuring Prettier..."))
		if err := createPrettierConfig(); err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Println(ui.SuccessStyle.Render("✓ Git hooks and code quality tools setup complete!"))

	return nil
}

func isNodeInstalled() bool {
	_, err := exec.LookPath("node")
	return err == nil
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

func initGit() error {
	cmd := exec.Command("git", "init")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createRootPackageJSON() error {
	content := `{
  "name": "workspace-root",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "prepare": "husky"
  },
  "devDependencies": {}
}
`
	return os.WriteFile("package.json", []byte(content), 0644)
}

func installNpmPackages(dir string, packages []string, dev bool) error {
	args := []string{"install"}
	if dev {
		args = append(args, "--save-dev")
	}
	args = append(args, packages...)

	cmd := exec.Command("npm", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupHuskyHooks(dir string) error {
	// Initialize husky
	cmd := exec.Command("npx", "husky", "init")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Create pre-commit hook
	huskyDir := filepath.Join(dir, ".husky")
	preCommitPath := filepath.Join(huskyDir, "pre-commit")
	preCommitContent := `#!/usr/bin/env sh
. "$(dirname "$0")/_/husky.sh"

npx lint-staged
`
	if err := os.WriteFile(preCommitPath, []byte(preCommitContent), 0755); err != nil {
		return err
	}

	// Create commit-msg hook
	commitMsgPath := filepath.Join(huskyDir, "commit-msg")
	commitMsgContent := `#!/usr/bin/env sh
. "$(dirname "$0")/_/husky.sh"

npx --no -- commitlint --edit "$1"
`
	if err := os.WriteFile(commitMsgPath, []byte(commitMsgContent), 0755); err != nil {
		return err
	}

	return nil
}

func createLintStagedConfig() error {
	content := `{
  "*.{ts,js,json,md}": ["prettier --write"],
  "*.{ts,js}": ["eslint --fix"]
}
`
	return os.WriteFile(".lintstagedrc.json", []byte(content), 0644)
}

func createCommitlintConfig() error {
	content := `module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'type-enum': [
      2,
      'always',
      [
        'feat',
        'fix',
        'docs',
        'style',
        'refactor',
        'test',
        'chore',
        'perf',
        'ci',
        'build',
        'revert'
      ]
    ]
  }
};
`
	return os.WriteFile("commitlint.config.js", []byte(content), 0644)
}

func createPrettierConfig() error {
	content := `{
  "semi": true,
  "trailingComma": "es5",
  "singleQuote": true,
  "printWidth": 100,
  "tabWidth": 2,
  "useTabs": false,
  "arrowParens": "avoid",
  "endOfLine": "lf"
}
`
	prettierrc := os.WriteFile(".prettierrc", []byte(content), 0644)

	ignoreContent := `node_modules
dist
build
coverage
.next
bazel-*
*.min.js
*.bundle.js
`
	prettierIgnore := os.WriteFile(".prettierignore", []byte(ignoreContent), 0644)

	if prettierrc != nil {
		return prettierrc
	}
	return prettierIgnore
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
