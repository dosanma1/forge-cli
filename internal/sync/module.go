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

	// Extract Go dependencies from go.mod files
	var goDependencies []string
	if contains(languages, "go") && len(goModules) > 0 {
		deps, err := s.extractGoModDependencies(goModules)
		if err != nil {
			return "", fmt.Errorf("failed to extract go dependencies: %w", err)
		}
		goDependencies = deps
	}

	// Determine if there are frontend projects
	hasFrontend := contains(languages, "nestjs") || contains(languages, "angular") || contains(languages, "react")

	data := struct {
		ProjectName      string
		Version          string
		HasGo            bool
		HasJS            bool
		HasFrontend      bool
		WorkspaceRepo    string
		GoVersion        string
		GoModules        []string
		GoDependencies   []string
	}{
		ProjectName:      s.config.Workspace.Name,
		Version:          "0.1.0",
		HasGo:            contains(languages, "go"),
		HasJS:            hasFrontend,
		HasFrontend:      hasFrontend,
		WorkspaceRepo:    repoName,
		GoVersion:        goVersion,
		GoModules:        goModules,
		GoDependencies:   goDependencies,
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

// fixModuleBazelDependencies adds missing indirect dependencies to MODULE.bazel.
// This is needed because bazel mod tidy only includes direct dependencies by default,
// but misses blank imports like database drivers.
func (s *Syncer) fixModuleBazelDependencies() error {
	fmt.Println("🔧 Adding missing indirect dependencies...")
	
	modulePath := filepath.Join(s.workspaceRoot, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	// Extract current use_repo dependencies
	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	
	// Find the use_repo line for go_deps
	var useRepoLineIndex = -1
	var currentDeps []string
	
	for i, line := range lines {
		if strings.Contains(line, "use_repo(go_deps,") {
			useRepoLineIndex = i
			// Extract current dependencies
			// Handle both single line and multi-line use_repo
			useRepoContent := line
			
			// If it doesn't end with ), it might be multi-line
			if !strings.Contains(line, ")") {
				for j := i + 1; j < len(lines); j++ {
					useRepoContent += " " + strings.TrimSpace(lines[j])
					if strings.Contains(lines[j], ")") {
						break
					}
				}
			}
			
			// Parse dependencies from use_repo line
			start := strings.Index(useRepoContent, "(go_deps,")
			end := strings.LastIndex(useRepoContent, ")")
			if start != -1 && end != -1 {
				depsStr := useRepoContent[start+len("(go_deps,"):end]
				depsStr = strings.TrimSpace(depsStr)
				if depsStr != "" {
					// Split by comma and clean up
					for _, dep := range strings.Split(depsStr, ",") {
						dep = strings.TrimSpace(dep)
						dep = strings.Trim(dep, `"`)
						if dep != "" {
							currentDeps = append(currentDeps, dep)
						}
					}
				}
			}
			break
		}
	}
	
	// Essential dependencies that should always be included (common blank imports)
	essentialDeps := []string{
		"com_github_lib_pq", // PostgreSQL driver
	}
	
	// Add missing essential dependencies
	depsMap := make(map[string]bool)
	for _, dep := range currentDeps {
		depsMap[dep] = true
	}
	
	needsUpdate := false
	for _, dep := range essentialDeps {
		if !depsMap[dep] {
			currentDeps = append(currentDeps, dep)
			depsMap[dep] = true
			needsUpdate = true
		}
	}
	
	if needsUpdate && useRepoLineIndex != -1 {
		// Sort dependencies for consistency
		// (keep existing order, just add new ones at the end)
		
		// Rebuild use_repo line
		newUseRepoLine := `use_repo(go_deps`
		for _, dep := range currentDeps {
			newUseRepoLine += `, "` + dep + `"`
		}
		newUseRepoLine += `)`
		
		// Replace the line(s)
		newLines := make([]string, 0, len(lines))
		i := 0
		for i < len(lines) {
			if i == useRepoLineIndex {
				newLines = append(newLines, newUseRepoLine)
				// Skip any continuation lines
				for i < len(lines) && !strings.Contains(lines[i], ")") {
					i++
				}
				i++ // Skip the closing line
			} else {
				newLines = append(newLines, lines[i])
				i++
			}
		}
		
		// Write back to file
		newContent := strings.Join(newLines, "\n")
		if err := os.WriteFile(modulePath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write MODULE.bazel: %w", err)
		}
		
		fmt.Printf("✅ Added missing dependencies: %v\n", essentialDeps)
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

// extractGoModDependencies parses go.mod files and extracts all dependencies.
func (s *Syncer) extractGoModDependencies(modules []string) ([]string, error) {
	depMap := make(map[string]bool)

	for _, module := range modules {
		goModPath := filepath.Join(s.workspaceRoot, module, "go.mod")
		content, err := os.ReadFile(goModPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read %s: %w", goModPath, err)
		}

		lines := strings.Split(string(content), "\n")
		inRequireBlock := false

		for _, line := range lines {
			line = strings.TrimSpace(line)

			// Check for "require (" block
			if strings.HasPrefix(line, "require (") {
				inRequireBlock = true
				continue
			}

			if inRequireBlock {
				if line == ")" {
					inRequireBlock = false
					continue
				}

				// Parse dependency line: "github.com/lib/pq v1.10.9"
				if line != "" && !strings.HasPrefix(line, "//") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						modulePath := parts[0]
						// Convert to Bazel repository name
						repoName := goModuleToRepoName(modulePath)
						depMap[repoName] = true
					}
				}
			} else if strings.HasPrefix(line, "require ") {
				// Single line require statement
				line = strings.TrimPrefix(line, "require ")
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					modulePath := parts[0]
					repoName := goModuleToRepoName(modulePath)
					depMap[repoName] = true
				}
			}
		}
	}

	// Convert map to sorted slice
	deps := make([]string, 0, len(depMap))
	for dep := range depMap {
		deps = append(deps, dep)
	}

	return deps, nil
}

// goModuleToRepoName converts a Go module path to a Bazel repository name.
// e.g., "github.com/lib/pq" -> "com_github_lib_pq"
func goModuleToRepoName(modulePath string) string {
	// Remove version suffixes like /v2, /v3
	if idx := strings.LastIndex(modulePath, "/v"); idx != -1 {
		if len(modulePath) > idx+2 {
			// Check if it's a version suffix
			rest := modulePath[idx+2:]
			isVersion := true
			for _, ch := range rest {
				if ch < '0' || ch > '9' {
					isVersion = false
					break
				}
			}
			if isVersion {
				modulePath = modulePath[:idx]
			}
		}
	}

	// Replace dots and slashes with underscores
	name := strings.ReplaceAll(modulePath, ".", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "-", "_")

	// Reverse domain notation (github.com -> com_github)
	parts := strings.Split(modulePath, "/")
	if len(parts) >= 2 {
		domain := parts[0]
		domainParts := strings.Split(domain, ".")
		// Reverse domain parts
		for i, j := 0, len(domainParts)-1; i < j; i, j = i+1, j-1 {
			domainParts[i], domainParts[j] = domainParts[j], domainParts[i]
		}
		reversedDomain := strings.Join(domainParts, "_")
		
		// Reconstruct with reversed domain
		result := reversedDomain
		for i := 1; i < len(parts); i++ {
			result += "_" + strings.ReplaceAll(strings.ReplaceAll(parts[i], ".", "_"), "-", "_")
		}
		return result
	}

	return name
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
