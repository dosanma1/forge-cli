package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// SyncReport contains the results of a sync operation.
type SyncReport struct {
	DeletedFiles []string
	CreatedFiles []string
	Errors       []error
}

// Syncer handles workspace synchronization operations.
type Syncer struct {
	workspaceRoot string
	config        *workspace.Config
	engine        *template.Engine
	dryRun        bool
}

// NewSyncer creates a new Syncer instance.
func NewSyncer(workspaceRoot string, dryRun bool) (*Syncer, error) {
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace config: %w", err)
	}

	return &Syncer{
		workspaceRoot: workspaceRoot,
		config:        config,
		engine:        template.NewEngine(),
		dryRun:        dryRun,
	}, nil
}

// Sync performs a full workspace synchronization following the Bazel bzlmod workflow.
func (s *Syncer) Sync() (*SyncReport, error) {
	report := &SyncReport{
		DeletedFiles: []string{},
		CreatedFiles: []string{},
		Errors:       []error{},
	}

	fmt.Println("🚀 Starting Bazel workspace sync...")
	fmt.Println()

	// Detect Go projects from forge.json
	goProjects := s.getGoProjects()

	if len(goProjects) == 0 {
		fmt.Println("⚠️  No Go projects found in forge.json")
		return report, nil
	}

	fmt.Printf("🔍 Found %d Go project(s):\n", len(goProjects))
	for _, proj := range goProjects {
		fmt.Printf("   - %s (%s)\n", proj.Name, proj.Root)
	}
	fmt.Println()

	if s.dryRun {
		fmt.Println("🏃 DRY RUN - No changes will be made")
		return report, nil
	}

	// Step 1: Generate root BUILD.bazel with gazelle target
	fmt.Println("📝 Step 1: Generating root BUILD.bazel...")
	if err := s.generateRootBuildFile(goProjects); err != nil {
		return report, fmt.Errorf("failed to generate root BUILD.bazel: %w", err)
	}
	fmt.Println("✅ Root BUILD.bazel generated")
	fmt.Println()

	// Step 2: Generate go.work and run go work sync
	fmt.Println("📝 Step 2: Syncing go.work...")
	if err := s.syncGoWork(goProjects); err != nil {
		return report, fmt.Errorf("failed to sync go.work: %w", err)
	}
	fmt.Println("✅ go.work synced")
	fmt.Println()

	// Step 2b: Ensure MODULE.bazel has OCI support
	fmt.Println("📝 Step 2b: Ensuring OCI support in MODULE.bazel...")
	if err := s.ensureOciSupport(); err != nil {
		return report, fmt.Errorf("failed to ensure OCI support: %w", err)
	}
	fmt.Println("✅ OCI support ensured")
	fmt.Println()

	// Step 3: Create empty BUILD files in service directories
	// (Required for bzlmod to evaluate go.work references)
	fmt.Println("📝 Step 3: Creating BUILD files in service directories...")
	for _, proj := range goProjects {
		buildPath := filepath.Join(s.workspaceRoot, proj.Root, "BUILD.bazel")
		if _, err := os.Stat(buildPath); os.IsNotExist(err) {
			if err := os.WriteFile(buildPath, []byte("# Managed by gazelle\n"), 0644); err != nil {
				return report, fmt.Errorf("failed to create BUILD file for %s: %w", proj.Name, err)
			}
			fmt.Printf("   Created %s/BUILD.bazel\n", proj.Root)
		}
	}
	fmt.Println("✅ BUILD files created")
	fmt.Println()

	// Step 4: Run gazelle to populate BUILD.bazel files
	fmt.Println("📝 Step 4: Generating BUILD.bazel files...")
	if err := s.runGazelle(); err != nil {
		return report, fmt.Errorf("failed to run gazelle: %w", err)
	}
	fmt.Println("✅ BUILD.bazel files generated")
	fmt.Println()

	// Step 4b: Add container image targets for services
	fmt.Println("📝 Step 4b: Adding container image targets for services...")
	if err := s.ensureServiceImageTargets(); err != nil {
		return report, fmt.Errorf("failed to add container image targets: %w", err)
	}
	fmt.Println("✅ Container image targets ready")
	fmt.Println()

	// Step 5: Run bazel mod tidy (reads go.work via go_deps.from_file)
	fmt.Println("📝 Step 5: Running bazel mod tidy...")
	if err := s.runBazelModTidy(); err != nil {
		return report, fmt.Errorf("failed to run bazel mod tidy: %w", err)
	}
	fmt.Println("✅ Dependencies resolved from go.work")
	fmt.Println()

	// Step 6: Validate workspace
	fmt.Println("🔍 Step 6: Validating workspace...")
	if err := s.validateWorkspace(); err != nil {
		fmt.Printf("⚠️  Warning: %v\n", err)
		report.Errors = append(report.Errors, err)
	} else {
		fmt.Println("✅ Workspace validated")
	}
	fmt.Println()

	// Final summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ Sync complete!")
	if len(report.Errors) > 0 {
		fmt.Printf("⚠️  Completed with %d warning(s)\n", len(report.Errors))
	}
	fmt.Println("Ready for: forge build, forge test, forge deploy")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return report, nil
}

// Validate checks workspace integrity without making changes.
func (s *Syncer) Validate() error {
	// Check forge.json exists and is valid
	if s.config == nil {
		return fmt.Errorf("forge.json not found or invalid")
	}

	// Check MODULE.bazel exists
	modulePath := filepath.Join(s.workspaceRoot, "MODULE.bazel")
	if _, err := os.Stat(modulePath); os.IsNotExist(err) {
		return fmt.Errorf("MODULE.bazel not found")
	}

	// TODO: Validate MODULE.bazel has correct rules for detected languages
	// TODO: Validate BUILD files exist for all projects
	// TODO: Check for orphaned BUILD files

	return nil
}

// detectLanguages scans forge.json to determine which languages are used.
func (s *Syncer) detectLanguages() []string {
	languageMap := make(map[string]bool)

	for _, project := range s.config.Projects {
		if project.Language != "" {
			languageMap[project.Language] = true
		}
	}

	languages := make([]string, 0, len(languageMap))
	for lang := range languageMap {
		languages = append(languages, lang)
	}

	return languages
}

// deleteAllBuildFiles removes all Bazel files from the workspace.
func (s *Syncer) deleteAllBuildFiles(report *SyncReport) error {
	// Find all BUILD.bazel and MODULE.bazel files
	var filesToDelete []string

	err := filepath.WalkDir(s.workspaceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip bazel output directories and hidden directories
		if d.IsDir() {
			name := d.Name()
			if name == "bazel-bin" || name == "bazel-out" || name == "bazel-testlogs" ||
				name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			if len(name) > 0 && name[0] == '.' && name != "." {
				return filepath.SkipDir
			}
		}

		// Collect BUILD.bazel files
		if !d.IsDir() && d.Name() == "BUILD.bazel" {
			filesToDelete = append(filesToDelete, path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Add MODULE.bazel
	modulePath := filepath.Join(s.workspaceRoot, "MODULE.bazel")
	if _, err := os.Stat(modulePath); err == nil {
		filesToDelete = append(filesToDelete, modulePath)
	}

	// Delete files
	if !s.dryRun {
		for _, file := range filesToDelete {
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("failed to delete %s: %w", file, err)
			}
		}
	}

	report.DeletedFiles = filesToDelete
	return nil
}

// syncBuildFiles regenerates all BUILD.bazel files.
func (s *Syncer) syncBuildFiles(report *SyncReport) error {
	languages := s.detectLanguages()

	// Regenerate Go BUILD files
	if contains(languages, "go") {
		fmt.Println("🔧 Regenerating Go BUILD files...")
		if err := s.syncGoBuildFiles(report); err != nil {
			return err
		}
	}

	// Regenerate NestJS/Angular BUILD files
	if contains(languages, "nestjs") || contains(languages, "angular") || contains(languages, "react") {
		fmt.Println("🔧 Regenerating JavaScript BUILD files...")
		if err := s.syncJSBuildFiles(report); err != nil {
			return err
		}
	}

	return nil
}

// GoProject represents a Go project with a go.mod file
type GoProject struct {
	Name string
	Root string
}

// getGoProjects returns all Go projects from forge.json that have go.mod files
func (s *Syncer) getGoProjects() []GoProject {
	var projects []GoProject

	for name, project := range s.config.Projects {
		if project.Language != "go" {
			continue
		}

		// Check if go.mod exists
		goModPath := filepath.Join(s.workspaceRoot, project.Root, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			projects = append(projects, GoProject{
				Name: name,
				Root: project.Root,
			})
		}
	}

	return projects
}

// runGazelle executes bazel run //:gazelle to generate BUILD.bazel files
func (s *Syncer) runGazelle() error {
	cmd := exec.Command("bazel", "run", "//:gazelle")
	cmd.Dir = s.workspaceRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Use go.work if it exists for proper module resolution
	goWorkPath := filepath.Join(s.workspaceRoot, "go.work")
	if _, err := os.Stat(goWorkPath); err == nil {
		cmd.Env = append(os.Environ(), fmt.Sprintf("GOWORK=%s", goWorkPath))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gazelle execution failed: %w", err)
	}

	return nil
}

// validateWorkspace runs quick validation checks on the workspace
func (s *Syncer) validateWorkspace() error {
	// Check if we can query the workspace
	cmd := exec.Command("bazel", "query", "//...", "--noshow_progress")
	cmd.Dir = s.workspaceRoot
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("bazel query failed: %w\nOutput: %s", err, string(output))
	}

	// Count targets
	targets := strings.Split(strings.TrimSpace(string(output)), "\n")
	fmt.Printf("   Found %d Bazel target(s)\n", len(targets))

	return nil
}

// getWorkspaceModulesForGazelle returns all workspace module import paths
func (s *Syncer) getWorkspaceModulesForGazelle() ([]string, error) {
	goWorkPath := filepath.Join(s.workspaceRoot, "go.work")
	content, err := os.ReadFile(goWorkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.work: %w", err)
	}

	var modules []string
	lines := strings.Split(string(content), "\n")
	inUseBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "use (") {
			inUseBlock = true
			continue
		}

		if inUseBlock && line == ")" {
			inUseBlock = false
			continue
		}

		var modulePath string
		if inUseBlock {
			modulePath = strings.Trim(line, `"`)
		} else if strings.HasPrefix(line, "use ") {
			modulePath = strings.TrimPrefix(line, "use ")
			modulePath = strings.Trim(modulePath, `"`)
		}

		if modulePath != "" && modulePath != "." {
			goModPath := filepath.Join(s.workspaceRoot, modulePath, "go.mod")
			goModContent, err := os.ReadFile(goModPath)
			if err != nil {
				continue
			}

			for _, goModLine := range strings.Split(string(goModContent), "\n") {
				if strings.HasPrefix(goModLine, "module ") {
					importPath := strings.TrimSpace(strings.TrimPrefix(goModLine, "module "))
					modules = append(modules, importPath)
					break
				}
			}
		}
	}

	return modules, nil
}

// generateRootBuildFile creates the root BUILD.bazel with gazelle target and resolve directives.
func (s *Syncer) generateRootBuildFile(goProjects []GoProject) error {
	buildFile := filepath.Join(s.workspaceRoot, "BUILD.bazel")

	// Detect the Go module prefix from the first service's go.mod
	// This handles cases like "github.com/owner/repo" vs just "repo-name"
	modulePrefix := s.config.Workspace.Name // fallback
	if len(goProjects) > 0 {
		goModPath := filepath.Join(s.workspaceRoot, goProjects[0].Root, "go.mod")
		content, err := os.ReadFile(goModPath)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "module ") {
					modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module"))
					// Remove the service-specific suffix to get the base module path
					// e.g., "github.com/dosanma1/mmo-game/backend/services/auth" -> "github.com/dosanma1/mmo-game"
					if idx := strings.Index(modulePath, "/"+goProjects[0].Root); idx != -1 {
						modulePrefix = modulePath[:idx]
					} else {
						modulePrefix = modulePath
					}
					break
				}
			}
		}
	}

	// Scan for proto directories and add them as additional projects
	protoProjects := []GoProject{}
	for _, proj := range goProjects {
		projPath := filepath.Join(s.workspaceRoot, proj.Root)
		filepath.Walk(projPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return nil
			}
			// Check if directory contains .proto files
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil
			}
			hasProto := false
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".proto") {
					hasProto = true
					break
				}
			}
			if hasProto {
				relPath, _ := filepath.Rel(s.workspaceRoot, path)
				protoProjects = append(protoProjects, GoProject{
					Name: filepath.Base(path),
					Root: relPath,
				})
			}
			return nil
		})
	}

	// Combine base projects and proto projects
	allProjects := append(goProjects, protoProjects...)

	// Template data
	data := struct {
		ModulePrefix      string
		Projects          []GoProject
		GazelleDirectives []string
	}{
		ModulePrefix:      modulePrefix,
		Projects:          allProjects,
		GazelleDirectives: s.config.Workspace.GazelleDirectives,
	}

	// Render template
	content, err := s.engine.RenderTemplate("bazel/root-build.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render BUILD.bazel template: %w", err)
	}

	if err := os.WriteFile(buildFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write BUILD.bazel: %w", err)
	}

	fmt.Printf("   Added gazelle target with prefix %s and %d resolve directives\n", modulePrefix, len(allProjects))
	return nil
}

// updateGoDeps runs gazelle update-repos for each Go project to populate MODULE.bazel.
func (s *Syncer) updateGoDeps(goProjects []GoProject) error {
	// First, clean up old use_repo to avoid conflicts
	if err := s.cleanUseRepo(); err != nil {
		fmt.Printf("⚠️  Warning: failed to clean use_repo: %v\n", err)
	}

	for i, proj := range goProjects {
		goModPath := filepath.Join(proj.Root, "go.mod")
		fmt.Printf("   [%d/%d] Updating from %s...\n", i+1, len(goProjects), goModPath)

		cmd := exec.Command("bazel", "run", "//:gazelle", "--",
			"update-repos",
			"-from_file="+goModPath,
			"-prune",
		)
		cmd.Dir = s.workspaceRoot
		cmd.Env = append(os.Environ(), "GOWORK="+filepath.Join(s.workspaceRoot, "go.work"))

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("⚠️  Warning: gazelle update-repos failed for %s: %v\n", proj.Name, err)
			if len(output) > 0 {
				fmt.Printf("   Output: %s\n", string(output))
			}
			// Continue with other projects
			continue
		}

		fmt.Printf("      ✓ %s\n", proj.Name)
	}

	return nil
}

// cleanUseRepo removes use_repo lines from MODULE.bazel to avoid conflicts
func (s *Syncer) cleanUseRepo() error {
	modulePath := filepath.Join(s.workspaceRoot, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	inUseRepo := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Start of use_repo block
		if strings.HasPrefix(trimmed, "use_repo(go_deps,") {
			inUseRepo = true
			continue
		}

		// Inside use_repo block
		if inUseRepo {
			// End of use_repo block
			if strings.HasSuffix(trimmed, ")") {
				inUseRepo = false
				continue
			}
			// Skip lines inside use_repo
			continue
		}

		newLines = append(newLines, line)
	}

	newContent := strings.Join(newLines, "\n")
	return os.WriteFile(modulePath, []byte(newContent), 0644)
}

// syncGoWork creates go.work and runs go work sync
func (s *Syncer) syncGoWork(goProjects []GoProject) error {
	goWorkPath := filepath.Join(s.workspaceRoot, "go.work")

	// Create go.work content
	content := "go 1.24.0\n\n"
	for _, proj := range goProjects {
		content += fmt.Sprintf("use ./%s\n", proj.Root)
	}

	if err := os.WriteFile(goWorkPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write go.work: %w", err)
	}

	fmt.Printf("   Created go.work with %d modules\n", len(goProjects))

	// Run go work sync to update go.mod files
	cmd := exec.Command("go", "work", "sync")
	cmd.Dir = s.workspaceRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run go work sync: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("   Ran go work sync")
	return nil
}

// updateModuleDeps extracts all dependencies from go.work and adds them to MODULE.bazel
func (s *Syncer) updateModuleDeps() error {
	goWorkPath := filepath.Join(s.workspaceRoot, "go.work")

	// Get all modules and their dependencies using go list
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = s.workspaceRoot
	cmd.Env = append(os.Environ(), "GOWORK="+goWorkPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list Go modules: %w\nOutput: %s", err, string(output))
	}

	// Parse JSON output
	type Module struct {
		Path     string
		Version  string
		Main     bool
		Indirect bool
		GoMod    string
	}

	var modules []Module
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for {
		var mod Module
		if err := decoder.Decode(&mod); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("failed to parse module JSON: %w", err)
		}

		// Skip main modules (our workspace modules)
		if mod.Main {
			continue
		}

		// Skip modules without versions (local or replaced)
		if mod.Version == "" {
			continue
		}

		// Skip workspace modules
		if strings.HasPrefix(mod.Path, s.config.Workspace.Name) {
			continue
		}

		modules = append(modules, mod)
	}

	fmt.Printf("   Found %d external dependencies\n", len(modules))

	// Collect sums from all go.sum files in workspace
	sums := make(map[string]string) // "path@version" -> sum
	goWorkContent, _ := os.ReadFile(goWorkPath)
	for _, line := range strings.Split(string(goWorkContent), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "use ") {
			modPath := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "use "))
			modPath = strings.Trim(modPath, "./")
			sumPath := filepath.Join(s.workspaceRoot, modPath, "go.sum")
			if sumContent, err := os.ReadFile(sumPath); err == nil {
				for _, sumLine := range strings.Split(string(sumContent), "\n") {
					parts := strings.Fields(sumLine)
					if len(parts) >= 3 {
						// go.sum format: "module version hash"
						// Skip the /go.mod line, we want the module itself
						if !strings.HasSuffix(parts[0], "/go.mod") {
							key := parts[0] + "@" + parts[1]
							sums[key] = parts[2]
						}
					}
				}
			}
		}
	}

	fmt.Printf("   Collected %d sums from go.sum files\n", len(sums))

	// Read current MODULE.bazel
	modulePath := filepath.Join(s.workspaceRoot, "MODULE.bazel")
	content, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	skipSection := false
	inserted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip existing go_deps.module() calls
		if strings.HasPrefix(trimmed, "go_deps.module(") {
			skipSection = true
			continue
		}

		if skipSection {
			if trimmed == ")" {
				skipSection = false
			}
			continue
		}

		newLines = append(newLines, line)

		// After go_deps extension line, insert all module calls
		if !inserted && strings.HasPrefix(trimmed, "go_deps = use_extension") {
			// Add all module dependencies
			addedCount := 0
			for _, mod := range modules {
				key := mod.Path + "@" + mod.Version
				sum, hasSum := sums[key]
				if !hasSum {
					// Skip modules without sums - they won't work in bzlmod anyway
					continue
				}

				newLines = append(newLines, fmt.Sprintf("go_deps.module("))
				newLines = append(newLines, fmt.Sprintf("    path = \"%s\",", mod.Path))
				newLines = append(newLines, fmt.Sprintf("    sum = \"%s\",", sum))
				newLines = append(newLines, fmt.Sprintf("    version = \"%s\",", mod.Version))
				newLines = append(newLines, ")")
				addedCount++
			}
			fmt.Printf("   Added %d go_deps.module() calls\n", addedCount)
			inserted = true
		}
	}

	// Write updated MODULE.bazel
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(modulePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	return nil
}
