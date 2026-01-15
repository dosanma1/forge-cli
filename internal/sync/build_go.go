package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoBuildData contains template data for Go BUILD generation.
type GoBuildData struct {
	PackageName   string
	BinaryName    string
	ImportPath    string
	ImageTag      string
	Files         []string
	TestFiles     []string
	HasTests      bool
	HasMigrations bool
	TestDataDeps  []string
	Modules       []WorkspaceModule
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

	// Check if this package has migrations folder
	migrationsPath := filepath.Join(s.workspaceRoot, pkg.Path, "migrations")
	hasMigrations := false
	if info, err := os.Stat(migrationsPath); err == nil && info.IsDir() {
		hasMigrations = true
	}

	// Determine test data dependencies if package has tests
	var testDataDeps []string
	if len(pkg.TestFiles) > 0 {
		testDataDeps = s.determineTestDataDeps(pkg)
	}

	data := GoBuildData{
		PackageName:   packageName,
		ImportPath:    pkg.ImportPath,
		Files:         pkg.Files,
		TestFiles:     pkg.TestFiles,
		HasTests:      len(pkg.TestFiles) > 0,
		HasMigrations: hasMigrations,
		TestDataDeps:  testDataDeps,
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

	// Check if this package has migrations folder (e.g., cmd/migrator)
	migrationsPath := filepath.Join(s.workspaceRoot, pkg.Path, "migrations")
	hasMigrations := false
	if info, err := os.Stat(migrationsPath); err == nil && info.IsDir() {
		hasMigrations = true
	}

	data := GoBuildData{
		BinaryName:    binaryName,
		ImportPath:    pkg.ImportPath,
		ImageTag:      imageTag,
		Files:         pkg.Files,
		TestFiles:     pkg.TestFiles,
		HasTests:      len(pkg.TestFiles) > 0,
		HasMigrations: hasMigrations,
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

// determineTestDataDeps analyzes test imports to determine which migration filegroups are needed
func (s *Syncer) determineTestDataDeps(pkg *GoPackage) []string {
	var dataDeps []string
	seenDeps := make(map[string]bool)

	// 1. Check test file imports directly - these are primary sources of migration deps
	for _, testFile := range pkg.TestFiles {
		testPath := filepath.Join(s.workspaceRoot, pkg.Path, testFile)
		content, err := os.ReadFile(testPath)
		if err != nil {
			continue
		}

		imports := extractImports(string(content))
		for _, imp := range imports {
			migrationDeps := s.findMigrationDepsForImport(imp)
			for _, dep := range migrationDeps {
				if !seenDeps[dep] {
					dataDeps = append(dataDeps, dep)
					seenDeps[dep] = true
				}
			}
		}
	}

	// 2. Check main package source files for imports that need migrations
	// This finds indirect migration deps (e.g., repository imports test helpers that use migrations)
	for _, srcFile := range pkg.Files {
		srcPath := filepath.Join(s.workspaceRoot, pkg.Path, srcFile)
		content, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}

		imports := extractImports(string(content))
		for _, imp := range imports {
			// Check all imports for migration dependencies
			migrationDeps := s.findMigrationDepsForImport(imp)
			for _, dep := range migrationDeps {
				if !seenDeps[dep] {
					dataDeps = append(dataDeps, dep)
					seenDeps[dep] = true
				}
			}
		}
	}

	// 3. If this package looks like a test helper (contains "test"), also look for service migrations
	if strings.Contains(pkg.Path, "test") || strings.Contains(pkg.Path, "authtest") {
		// Walk up the directory tree looking for cmd/migrator/migrations
		parentPath := pkg.Path
		for {
			parentPath = filepath.Dir(parentPath)
			if parentPath == "." || parentPath == "" {
				break
			}

			migratorPath := filepath.Join(parentPath, "cmd", "migrator", "migrations")
			absPath := filepath.Join(s.workspaceRoot, migratorPath)
			if info, err := os.Stat(absPath); err == nil && info.IsDir() {
				bazelTarget := "//" + filepath.ToSlash(filepath.Join(parentPath, "cmd", "migrator")) + ":migrations"
				if !seenDeps[bazelTarget] {
					dataDeps = append(dataDeps, bazelTarget)
					seenDeps[bazelTarget] = true
				}
				break
			}
		}
	}

	return dataDeps
}

// findMigrationDepsForImport finds migration dependencies for a given import path
func (s *Syncer) findMigrationDepsForImport(importPath string) []string {
	var deps []string

	// Find the package path for this import
	pkgPath := s.findPackagePathForImport(importPath)
	if pkgPath == "" {
		return deps
	}

	// Find migration deps for the package
	return s.findMigrationDepsForPackage(pkgPath)
}

// findPackagePathForImport converts an import path to a file system path relative to workspace root
func (s *Syncer) findPackagePathForImport(importPath string) string {
	// Get all workspace modules to find which one contains this import
	modules, err := s.getWorkspaceModules()
	if err != nil {
		return ""
	}

	for _, mod := range modules {
		if strings.HasPrefix(importPath, mod.ImportPath) {
			// Remove module prefix to get relative path
			relPath := strings.TrimPrefix(importPath, mod.ImportPath)
			relPath = strings.TrimPrefix(relPath, "/")

			// Combine module path with relative import path
			fullPath := filepath.Join(mod.Path, relPath)

			// Verify the path exists
			absPath := filepath.Join(s.workspaceRoot, fullPath)
			if _, err := os.Stat(absPath); err == nil {
				return fullPath
			}
		}
	}

	return ""
}

// findMigrationDepsForPackage finds migration filegroups that should be included for a given package
func (s *Syncer) findMigrationDepsForPackage(pkgPath string) []string {
	var deps []string
	seenDeps := make(map[string]bool)

	// 1. Check if this package itself has migrations folder
	migrationsPath := filepath.Join(s.workspaceRoot, pkgPath, "migrations")
	if info, err := os.Stat(migrationsPath); err == nil && info.IsDir() {
		bazelTarget := "//" + filepath.ToSlash(pkgPath) + ":migrations"
		deps = append(deps, bazelTarget)
		seenDeps[bazelTarget] = true
	}

	// 2. Walk up the directory tree looking for cmd/migrator/migrations
	// This is the only reliable pattern - migrations are always in cmd/migrator/migrations
	parentPath := pkgPath
	for {
		parentPath = filepath.Dir(parentPath)
		if parentPath == "." || parentPath == "" {
			break
		}

		// Check for cmd/migrator/migrations pattern
		migratorPath := filepath.Join(parentPath, "cmd", "migrator", "migrations")
		absPath := filepath.Join(s.workspaceRoot, migratorPath)
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			bazelTarget := "//" + filepath.ToSlash(filepath.Join(parentPath, "cmd", "migrator")) + ":migrations"
			if !seenDeps[bazelTarget] {
				deps = append(deps, bazelTarget)
				seenDeps[bazelTarget] = true
			}
			break
		}
	}

	// 3. If this is a test helper package, check what IT imports for migrations
	// This ensures transitive migration dependencies are included
	pkgFiles := s.getPackageFiles(pkgPath)
	for _, file := range pkgFiles {
		filePath := filepath.Join(s.workspaceRoot, pkgPath, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		imports := extractImports(string(content))
		for _, imp := range imports {
			// Skip if already found
			if seenDeps[imp] {
				continue
			}

			// Only recurse into migration-related packages to avoid infinite loops
			if !strings.Contains(imp, "test") && !strings.Contains(imp, "pgtest") && !strings.Contains(imp, "migrat") {
				continue
			}

			impPkgPath := s.findPackagePathForImport(imp)
			if impPkgPath == "" || impPkgPath == pkgPath {
				continue
			}

			// Recursively find migrations for the imported package
			transitiveDeps := s.findMigrationDepsForPackage(impPkgPath)
			for _, dep := range transitiveDeps {
				if !seenDeps[dep] {
					deps = append(deps, dep)
					seenDeps[dep] = true
				}
			}
		}
	}

	return deps
}

// getPackageFiles returns all non-test .go files in a package
func (s *Syncer) getPackageFiles(pkgPath string) []string {
	var files []string
	absPath := filepath.Join(s.workspaceRoot, pkgPath)
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, entry.Name())
		}
	}

	return files
}

// extractImports parses import statements from Go source code
func extractImports(content string) []string {
	var imports []string
	inImportBlock := false

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Single line import
		if strings.HasPrefix(line, "import \"") {
			imp := strings.TrimPrefix(line, "import \"")
			imp = strings.TrimSuffix(imp, "\"")
			imports = append(imports, imp)
			continue
		}

		// Import block start
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}

		// Import block end
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		// Import inside block
		if inImportBlock && strings.HasPrefix(line, "\"") {
			imp := strings.Trim(line, "\"")
			imports = append(imports, imp)
		}
	}

	return imports
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
