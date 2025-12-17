package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoBuildData contains template data for Go BUILD generation.
type GoBuildData struct {
	PackageName string
	BinaryName  string
	ImportPath  string
	ImageTag    string
	Files       []string
	TestFiles   []string
	HasTests    bool
	Modules     []WorkspaceModule
}

// WorkspaceModule represents a Go module in the workspace
type WorkspaceModule struct {
	ImportPath string
	Path       string
}

// GenerateGoBuild creates BUILD.bazel content for a Go package.
func (s *Syncer) GenerateGoBuild(pkg *GoPackage) (string, error) {
	// Determine if this is a service root (has go.mod)
	goModPath := filepath.Join(s.workspaceRoot, pkg.Path, "go.mod")
	isServiceRoot := false
	if _, err := os.Stat(goModPath); err == nil {
		isServiceRoot = true
	}

	// Service root: gazelle config only
	if isServiceRoot {
		// Parse go.work to get workspace modules
		modules, err := s.getWorkspaceModules()
		if err != nil {
			fmt.Printf("⚠️  Failed to parse workspace modules: %v\n", err)
			modules = []WorkspaceModule{} // Continue without modules
		}

		data := struct {
			ImportPath string
			Modules    []WorkspaceModule
		}{
			ImportPath: pkg.ImportPath,
			Modules:    modules,
		}

		content, err := s.engine.RenderTemplate("bazel/go-root.BUILD.bazel.tmpl", data)
		if err != nil {
			return "", fmt.Errorf("failed to render template: %w", err)
		}

		return content, nil
	}

	// Main package: binary BUILD
	if pkg.IsMain {
		return s.generateGoBinaryBuild(pkg)
	}

	// Library package: library BUILD
	return s.generateGoLibraryBuild(pkg)
}

// generateGoLibraryBuild creates BUILD.bazel for a Go library.
func (s *Syncer) generateGoLibraryBuild(pkg *GoPackage) (string, error) {
	packageName := filepath.Base(pkg.Path)
	if packageName == "." {
		packageName = filepath.Base(s.workspaceRoot)
	}

	data := GoBuildData{
		PackageName: packageName,
		ImportPath:  pkg.ImportPath,
		Files:       pkg.Files,
		TestFiles:   pkg.TestFiles,
		HasTests:    len(pkg.TestFiles) > 0,
	}

	content, err := s.engine.RenderTemplate("bazel/go-library.BUILD.bazel.tmpl", data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return content, nil
}

// generateGoBinaryBuild creates BUILD.bazel for a Go binary (main package).
func (s *Syncer) generateGoBinaryBuild(pkg *GoPackage) (string, error) {
	// Binary name is the directory name
	binaryName := filepath.Base(pkg.Path)
	if binaryName == "." {
		binaryName = filepath.Base(s.workspaceRoot)
	}

	// Image tag uses workspace name and binary name
	imageTag := fmt.Sprintf("%s/%s:latest", s.config.Workspace.Name, binaryName)

	data := GoBuildData{
		BinaryName: binaryName,
		ImportPath: pkg.ImportPath,
		ImageTag:   imageTag,
		Files:      pkg.Files,
		TestFiles:  pkg.TestFiles,
		HasTests:   len(pkg.TestFiles) > 0,
	}

	content, err := s.engine.RenderTemplate("bazel/go-binary.BUILD.bazel.tmpl", data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return content, nil
}

// WriteGoBuild writes a BUILD.bazel file for a Go package.
func (s *Syncer) WriteGoBuild(pkg *GoPackage, content string, report *SyncReport) error {
	buildPath := filepath.Join(s.workspaceRoot, pkg.Path, "BUILD.bazel")

	if s.dryRun {
		fmt.Printf("Would write: %s\n", buildPath)
		return nil
	}

	// Create directory if needed
	dir := filepath.Dir(buildPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(buildPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write BUILD.bazel: %w", err)
	}

	report.CreatedFiles = append(report.CreatedFiles, buildPath)
	return nil
}

// syncGoBuildFiles regenerates BUILD.bazel for all Go packages.
func (s *Syncer) syncGoBuildFiles(report *SyncReport) error {
	packages, err := s.DiscoverGoPackages()
	if err != nil {
		return fmt.Errorf("failed to discover Go packages: %w", err)
	}

	fmt.Printf("📦 Found %d Go packages\n", len(packages))

	for _, pkg := range packages {
		relPath := pkg.Path
		if relPath == "." {
			relPath = "root"
		}

		if pkg.IsMain {
			fmt.Printf("   🔹 %s (binary)\n", relPath)
		} else if strings.Contains(pkg.Path, string(filepath.Separator)) {
			fmt.Printf("   🔸 %s (library)\n", relPath)
		} else {
			fmt.Printf("   🔸 %s (service root)\n", relPath)
		}

		content, err := s.GenerateGoBuild(pkg)
		if err != nil {
			return fmt.Errorf("failed to generate BUILD for %s: %w", pkg.Path, err)
		}

		if err := s.WriteGoBuild(pkg, content, report); err != nil {
			return fmt.Errorf("failed to write BUILD for %s: %w", pkg.Path, err)
		}
	}

	return nil
}

// getWorkspaceModules parses go.work to find all workspace modules
func (s *Syncer) getWorkspaceModules() ([]WorkspaceModule, error) {
	goWorkPath := filepath.Join(s.workspaceRoot, "go.work")
	content, err := os.ReadFile(goWorkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.work: %w", err)
	}

	var modules []WorkspaceModule
	lines := strings.Split(string(content), "\n")
	inUseBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for use block start
		if strings.HasPrefix(line, "use (") {
			inUseBlock = true
			continue
		}

		// Check for use block end
		if inUseBlock && line == ")" {
			inUseBlock = false
			continue
		}

		// Parse use directive (either in block or standalone)
		var modulePath string
		if inUseBlock {
			modulePath = strings.Trim(line, `"`)
		} else if strings.HasPrefix(line, "use ") {
			// Remove "use " prefix and quotes
			modulePath = strings.TrimPrefix(line, "use ")
			modulePath = strings.Trim(modulePath, `"`)
		}

		if modulePath != "" && modulePath != "." {
			// Read go.mod to get the module import path
			goModPath := filepath.Join(s.workspaceRoot, modulePath, "go.mod")
			goModContent, err := os.ReadFile(goModPath)
			if err != nil {
				continue // Skip if can't read go.mod
			}

			// Extract module path from go.mod
			for _, goModLine := range strings.Split(string(goModContent), "\n") {
				if strings.HasPrefix(goModLine, "module ") {
					importPath := strings.TrimSpace(strings.TrimPrefix(goModLine, "module "))
					modules = append(modules, WorkspaceModule{
						ImportPath: importPath,
						Path:       modulePath,
					})
					break
				}
			}
		}
	}

	return modules, nil
}
