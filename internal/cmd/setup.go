package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	setupVerbose bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Check required tools and their versions",
	Long: `Check if all required tools for Forge are installed and show their versions.

This command will verify:
  - Essential build tools (Bazel, Skaffold, Docker, Helm, kubectl)
  - Language runtimes (Go, Node.js)
  - Cloud platform tools (gcloud, firebase)
  - Framework CLIs (Angular, NestJS)
  - Protocol buffer tools (protoc or buf)
  - Local Kubernetes (Kind)

Examples:
  forge setup           # Check all required tools
  forge setup --verbose # Show detailed output`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolVarP(&setupVerbose, "verbose", "v", false, "Show detailed output")
}

type Tool struct {
	Name               string
	Command            string
	VersionFlag        string
	Required           bool
	Category           string
	RecommendedVersion string
}

func runSetup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	tools := []Tool{
		// Essential Tools
		{Name: "Bazel", Command: "bazel", VersionFlag: "version", Required: true, Category: "Essential", RecommendedVersion: "7.0+"},
		{Name: "Skaffold", Command: "skaffold", VersionFlag: "version", Required: true, Category: "Essential", RecommendedVersion: "v2.10+"},
		{Name: "Docker", Command: "docker", VersionFlag: "--version", Required: true, Category: "Essential", RecommendedVersion: "24.0+"},
		{Name: "Helm", Command: "helm", VersionFlag: "version --short", Required: true, Category: "Essential", RecommendedVersion: "v3.13+"},
		{Name: "kubectl", Command: "kubectl", VersionFlag: "version --client --short", Required: true, Category: "Essential", RecommendedVersion: "v1.28+"},
		{Name: "Go", Command: "go", VersionFlag: "version", Required: true, Category: "Essential", RecommendedVersion: "1.21+"},
		{Name: "Node.js", Command: "node", VersionFlag: "--version", Required: true, Category: "Essential", RecommendedVersion: "v20+"},

		// Cloud Tools
		{Name: "gcloud", Command: "gcloud", VersionFlag: "version --format=value(version)", Required: false, Category: "Cloud", RecommendedVersion: "latest"},
		{Name: "Firebase", Command: "firebase", VersionFlag: "--version", Required: false, Category: "Cloud", RecommendedVersion: "13.0+"},

		// Framework CLIs
		{Name: "Angular CLI", Command: "ng", VersionFlag: "version", Required: false, Category: "Frameworks", RecommendedVersion: "18.0+"},
		{Name: "NestJS CLI", Command: "nest", VersionFlag: "--version", Required: false, Category: "Frameworks", RecommendedVersion: "10.0+"},

		// Protocol Buffers
		{Name: "protoc", Command: "protoc", VersionFlag: "--version", Required: false, Category: "Protocol Buffers", RecommendedVersion: "25.0+"},
		{Name: "buf", Command: "buf", VersionFlag: "version", Required: false, Category: "Protocol Buffers", RecommendedVersion: "1.28+"},

		// Local Kubernetes
		{Name: "Kind", Command: "kind", VersionFlag: "version", Required: false, Category: "Local Development", RecommendedVersion: "0.20+"},
	}

	fmt.Println("🔍 Checking required tools...\n")

	categories := make(map[string][]Tool)
	for _, tool := range tools {
		categories[tool.Category] = append(categories[tool.Category], tool)
	}

	allInstalled := true
	requiredMissing := []string{}

	categoryOrder := []string{"Essential", "Cloud", "Frameworks", "Protocol Buffers", "Local Development"}

	for _, category := range categoryOrder {
		tools := categories[category]
		if len(tools) == 0 {
			continue
		}

		fmt.Printf("📦 %s Tools:\n", category)
		for _, tool := range tools {
			installed, version := checkTool(ctx, tool)

			if installed {
				fmt.Printf("   ✅ %s: %s (recommended: %s)\n", tool.Name, version, tool.RecommendedVersion)
			} else {
				if tool.Required {
					fmt.Printf("   ❌ %s: Not installed (REQUIRED - recommended: %s)\n", tool.Name, tool.RecommendedVersion)
					requiredMissing = append(requiredMissing, tool.Name)
					allInstalled = false
				} else {
					fmt.Printf("   ⚠️  %s: Not installed (optional - recommended: %s)\n", tool.Name, tool.RecommendedVersion)
				}
			}
		}
		fmt.Println()
	}

	if !allInstalled {
		fmt.Println("❌ Missing required tools:")
		for _, tool := range requiredMissing {
			fmt.Printf("   - %s\n", tool)
		}
		fmt.Println("\nPlease install the missing tools. See the installation guide:")
		fmt.Println("https://github.com/dosanma1/forge-cli#prerequisites")
		return fmt.Errorf("missing required tools")
	}

	fmt.Println("✅ All required tools are installed!")
	fmt.Println("\nYou're ready to use Forge! Try:")
	fmt.Println("   forge new my-project")

	return nil
}

func checkTool(ctx context.Context, tool Tool) (bool, string) {
	// Check if command exists
	_, err := exec.LookPath(tool.Command)
	if err != nil {
		return false, ""
	}

	// Get version
	versionArgs := strings.Split(tool.VersionFlag, " ")
	cmd := exec.CommandContext(ctx, tool.Command, versionArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Tool exists but version command failed, still mark as installed
		return true, "installed (version unknown)"
	}

	version := strings.TrimSpace(string(output))

	// Clean up version output for specific tools
	switch tool.Command {
	case "go":
		// "go version go1.21.5 darwin/arm64" -> "go1.21.5"
		parts := strings.Fields(version)
		if len(parts) >= 3 {
			version = parts[2]
		}
	case "node":
		// Version already clean: "v20.10.0"
	case "ng":
		// Extract just the version line from Angular CLI output
		lines := strings.Split(version, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Angular CLI:") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					version = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	case "gcloud":
		// Already formatted by --format flag
	case "firebase":
		// Version already clean
	case "bazel":
		// Extract version from "Bazelisk version: ..." or "Build label: ..."
		lines := strings.Split(version, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Build label:") {
				version = strings.TrimPrefix(line, "Build label: ")
				break
			}
		}
	case "skaffold":
		// "v2.10.1" - keep as is
	case "docker":
		// "Docker version 24.0.7, build afdd53b" -> "24.0.7"
		if strings.Contains(version, "version") {
			parts := strings.Split(version, " ")
			if len(parts) >= 3 {
				version = strings.TrimSuffix(parts[2], ",")
			}
		}
	case "helm":
		// "v3.13.1+g3547a4b" -> "v3.13.1"
		// Handle warnings by looking for version pattern
		lines := strings.Split(version, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Skip warning lines
			if strings.HasPrefix(line, "WARNING:") || strings.HasPrefix(line, "Error:") {
				continue
			}
			// Look for version pattern (v followed by numbers)
			if strings.HasPrefix(line, "v") {
				version = line
				if idx := strings.Index(version, "+"); idx > 0 {
					version = version[:idx]
				}
				break
			}
		}
	case "kubectl":
		// Extract version from output
		if strings.Contains(version, "Client Version:") {
			parts := strings.Split(version, ":")
			if len(parts) >= 2 {
				version = strings.TrimSpace(parts[1])
			}
		}
	}

	// Limit version string length
	if len(version) > 100 {
		lines := strings.Split(version, "\n")
		if len(lines) > 0 {
			version = lines[0]
		}
		if len(version) > 100 {
			version = version[:100] + "..."
		}
	}

	return true, version
}
