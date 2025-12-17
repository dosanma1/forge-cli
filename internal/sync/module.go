package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/template"
)

// Sync uses the same template data structure as the generator

// GenerateModuleBazel creates MODULE.bazel based on detected languages.
func (s *Syncer) GenerateModuleBazel(languages []string) (string, error) {
	// Detect which language rules are needed
	repoName := ""
	if s.config.Workspace.GitHub != nil && s.config.Workspace.GitHub.Org != "" {
		repoName = fmt.Sprintf("github.com/%s/%s", s.config.Workspace.GitHub.Org, s.config.Workspace.Name)
	}

	// Get Go version from config
	goVersion := "1.24.0" // Default
	if s.config.Workspace.ToolVersions != nil && s.config.Workspace.ToolVersions.Go != "" {
		goVersion = s.config.Workspace.ToolVersions.Go
	}

	// Parse go.work to find library modules with go.mod files
	// Only libraries should be in go_deps, services get dependencies transitively
	var goModules []string
	if contains(languages, "go") {
		modules, err := s.parseGoWorkModules()
		if err != nil {
			return "", fmt.Errorf("failed to parse go.work: %w", err)
		}

		// Build a map of service/app paths from forge.json projects
		serviceAppPaths := make(map[string]bool)
		for _, project := range s.config.Projects {
			// Skip libraries - we want to include them in go_deps
			if project.ProjectType == "library" {
				continue
			}
			// Mark services and applications as excluded
			if project.ProjectType == "service" || project.ProjectType == "application" {
				serviceAppPaths[project.Root] = true
			}
		}

		// Filter to only include libraries (modules not registered as services/apps)
		for _, mod := range modules {
			if !serviceAppPaths[mod] {
				goModules = append(goModules, mod)
			}
		}
	}

	// Determine if there are frontend projects
	hasFrontend := contains(languages, "nestjs") || contains(languages, "angular") || contains(languages, "react")

	data := struct {
		ProjectName   string
		Version       string
		HasGo         bool
		HasJS         bool
		HasFrontend   bool
		WorkspaceRepo string
		GoVersion     string
		GoModules     []string
	}{
		ProjectName:   s.config.Workspace.Name,
		Version:       "0.1.0",
		HasGo:         contains(languages, "go"),
		HasJS:         hasFrontend,
		HasFrontend:   hasFrontend,
		WorkspaceRepo: repoName,
		GoVersion:     goVersion,
		GoModules:     goModules,
	}

	// Use the same template file that forge new uses
	engine := template.NewEngine()
	content, err := engine.RenderTemplate("bazel/MODULE.bazel.tmpl", data)
	if err != nil {
		return "", fmt.Errorf("failed to render MODULE.bazel template: %w", err)
	}

	return content, nil
}

// WriteModuleBazel writes the generated MODULE.bazel to disk.
func (s *Syncer) WriteModuleBazel(content string, report *SyncReport) error {
	modulePath := filepath.Join(s.workspaceRoot, "MODULE.bazel")

	if s.dryRun {
		fmt.Printf("Would write: %s\n", modulePath)
		return nil
	}

	if err := os.WriteFile(modulePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	report.CreatedFiles = append(report.CreatedFiles, modulePath)
	return nil
}

// syncModuleBazel regenerates MODULE.bazel based on detected languages.
func (s *Syncer) syncModuleBazel(languages []string, report *SyncReport) error {
	fmt.Println("📝 Regenerating MODULE.bazel...")

	content, err := s.GenerateModuleBazel(languages)
	if err != nil {
		return err
	}

	return s.WriteModuleBazel(content, report)
}

// runBazelModTidy runs bazel mod tidy to populate use_repo() declarations.
func (s *Syncer) runBazelModTidy() error {
	fmt.Println("🔧 Running bazel mod tidy...")
	cmd := exec.Command("bazel", "mod", "tidy")
	cmd.Dir = s.workspaceRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run bazel mod tidy: %w", err)
	}
	return nil
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// parseGoWorkModules extracts the list of module directories from go.work.
func (s *Syncer) parseGoWorkModules() ([]string, error) {
	goWorkPath := filepath.Join(s.workspaceRoot, "go.work")

	content, err := os.ReadFile(goWorkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No go.work file
		}
		return nil, fmt.Errorf("failed to read go.work: %w", err)
	}

	var modules []string
	lines := strings.Split(string(content), "\n")
	inUseBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for "use" or "use (" blocks
		if strings.HasPrefix(line, "use (") {
			inUseBlock = true
			continue
		}

		if inUseBlock {
			if line == ")" {
				inUseBlock = false
				continue
			}

			// Extract path from use block
			if line != "" && !strings.HasPrefix(line, "//") {
				modulePath := strings.Trim(line, "\"")
				modulePath = strings.TrimPrefix(modulePath, "./")
				modules = append(modules, modulePath)
			}
		} else if strings.HasPrefix(line, "use ") {
			// Single line use statement
			modulePath := strings.TrimPrefix(line, "use ")
			modulePath = strings.Trim(modulePath, "\"")
			modulePath = strings.TrimPrefix(modulePath, "./")
			modules = append(modules, modulePath)
		}
	}

	return modules, nil
}
