package sync

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// GoPackage represents a discovered Go package.
type GoPackage struct {
	Path       string   // Relative path from workspace root
	ImportPath string   // Full import path (e.g., github.com/org/repo/path)
	IsMain     bool     // True if package main
	Files      []string // Go source files
	TestFiles  []string // Go test files
	HasSubdirs bool     // True if has subdirectories with Go packages
}

// DiscoverGoPackages finds all Go packages in the workspace.
func (s *Syncer) DiscoverGoPackages() ([]*GoPackage, error) {
	var packages []*GoPackage
	processedDirs := make(map[string]bool)

	// Get module path from go.work or go.mod at root
	modulePath, err := s.getGoModulePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get module path: %w", err)
	}

	err = filepath.WalkDir(s.workspaceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			name := d.Name()

			// Skip bazel output and common ignored directories
			if name == "bazel-bin" || name == "bazel-out" || name == "bazel-testlogs" ||
				name == ".git" || name == "node_modules" || name == "vendor" ||
				strings.HasPrefix(name, "bazel-") {
				return filepath.SkipDir
			}

			// Skip hidden directories except workspace root
			if path != s.workspaceRoot && len(name) > 0 && name[0] == '.' {
				return filepath.SkipDir
			}

			// Check for go.mod (service root)
			goModPath := filepath.Join(path, "go.mod")
			if _, err := os.Stat(goModPath); err == nil && !processedDirs[path] {
				processedDirs[path] = true
				relPath, _ := filepath.Rel(s.workspaceRoot, path)

				// Build import path for service
				importPath := filepath.Join(modulePath, relPath)
				importPath = strings.ReplaceAll(importPath, string(filepath.Separator), "/")

				packages = append(packages, &GoPackage{
					Path:       relPath,
					ImportPath: importPath,
					IsMain:     false,
					Files:      []string{},
					TestFiles:  []string{},
					HasSubdirs: true,
				})
			}

			return nil
		}

		// Only process .go files
		if filepath.Ext(d.Name()) != ".go" {
			return nil
		}

		// Get package directory
		pkgDir := filepath.Dir(path)

		// Skip if we already processed this package
		if processedDirs[pkgDir] {
			return nil
		}
		processedDirs[pkgDir] = true

		// Discover package
		pkg, err := s.discoverGoPackage(pkgDir, modulePath)
		if err != nil {
			fmt.Printf("⚠️  Warning: failed to process package at %s: %v\n", pkgDir, err)
			return nil
		}

		if pkg != nil {
			packages = append(packages, pkg)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return packages, nil
}

// getGoModulePath extracts the module path from go.work or go.mod.
func (s *Syncer) getGoModulePath() (string, error) {
	// Try go.work first
	workPath := filepath.Join(s.workspaceRoot, "go.work")
	if _, err := os.Stat(workPath); err == nil {
		// For workspaces, build import path from GitHub org + workspace name
		if s.config.Workspace.GitHub != nil && s.config.Workspace.GitHub.Org != "" {
			return fmt.Sprintf("github.com/%s/%s", s.config.Workspace.GitHub.Org, s.config.Workspace.Name), nil
		}
	}

	// Try go.mod
	modPath := filepath.Join(s.workspaceRoot, "go.mod")
	if content, err := os.ReadFile(modPath); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
			}
		}
	}

	return "", fmt.Errorf("no go.work or go.mod found")
}

// discoverGoPackage analyzes a directory to extract Go package information.
func (s *Syncer) discoverGoPackage(pkgDir, modulePath string) (*GoPackage, error) {
	relPath, err := filepath.Rel(s.workspaceRoot, pkgDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path: %w", err)
	}

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	var testFiles []string
	var isMain bool
	hasSubdirs := false

	// Check for subdirectories
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			hasSubdirs = true
			break
		}
	}

	// Parse Go files
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}

		filename := entry.Name()

		// Skip test files for now
		if strings.HasSuffix(filename, "_test.go") {
			testFiles = append(testFiles, filename)
			continue
		}

		files = append(files, filename)

		// Parse file to check if it's package main
		if !isMain {
			fullPath := filepath.Join(pkgDir, filename)
			f, err := parser.ParseFile(fset, fullPath, nil, parser.PackageClauseOnly)
			if err == nil && f.Name.Name == "main" {
				isMain = true
			}
		}
	}

	// Skip if no Go files
	if len(files) == 0 && len(testFiles) == 0 {
		return nil, nil
	}

	// Build import path
	importPath := filepath.Join(modulePath, relPath)
	importPath = strings.ReplaceAll(importPath, string(filepath.Separator), "/")

	return &GoPackage{
		Path:       relPath,
		ImportPath: importPath,
		IsMain:     isMain,
		Files:      files,
		TestFiles:  testFiles,
		HasSubdirs: hasSubdirs,
	}, nil
}
