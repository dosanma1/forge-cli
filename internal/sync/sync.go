package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/internal/workspace"
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

// Sync performs a full workspace synchronization.
func (s *Syncer) Sync() (*SyncReport, error) {
	report := &SyncReport{
		DeletedFiles: []string{},
		CreatedFiles: []string{},
		Errors:       []error{},
	}

	// Detect languages from forge.json
	languages := s.detectLanguages()

	fmt.Printf("🔍 Detected languages: %v\n", languages)

	// Step 1: Delete existing Bazel files
	if err := s.deleteAllBuildFiles(report); err != nil {
		return report, fmt.Errorf("failed to delete files: %w", err)
	}

	// Step 2: Regenerate MODULE.bazel
	if err := s.syncModuleBazel(languages, report); err != nil {
		return report, fmt.Errorf("failed to sync MODULE.bazel: %w", err)
	}

	// Step 3: Regenerate BUILD.bazel files
	if err := s.syncBuildFiles(report); err != nil {
		return report, fmt.Errorf("failed to sync BUILD files: %w", err)
	}

	// Step 4: Run bazel mod tidy to populate use_repo declarations
	if contains(languages, "go") && !s.dryRun {
		if err := s.runBazelModTidy(); err != nil {
			fmt.Printf("⚠️  bazel mod tidy failed: %v\n", err)
			// Don't fail the sync, continue to gazelle
		}
		
		// Step 4.5: Fix missing indirect dependencies (like blank import drivers)
		if err := s.fixModuleBazelDependencies(); err != nil {
			fmt.Printf("⚠️  Failed to fix MODULE.bazel dependencies: %v\n", err)
		}
	}

	// Step 5: Run gazelle to resolve Go dependencies
	if contains(languages, "go") && !s.dryRun {
		fmt.Println("🔄 Running gazelle to resolve dependencies...")
		if err := s.runGazelle(); err != nil {
			fmt.Printf("⚠️  Gazelle execution failed: %v\n", err)
			// Don't fail the sync, gazelle might not be available yet
		} else {
			fmt.Println("✅ Gazelle completed successfully")
		}
	}

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

// runGazelle executes bazel run //:gazelle to resolve Go dependencies
func (s *Syncer) runGazelle() error {
	// Get workspace modules to pass as -known_import flags
	modules, err := s.getWorkspaceModulesForGazelle()
	if err != nil {
		fmt.Printf("⚠️  Failed to get workspace modules: %v\n", err)
	}

	args := []string{"run", "//:gazelle", "--", "-go_repository_mode"}

	// Add -known_import flag for each workspace module to prevent external lookups
	for _, mod := range modules {
		args = append(args, "-known_import="+mod)
	}

	cmd := exec.Command("bazel", args...)
	cmd.Dir = s.workspaceRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gazelle execution failed: %w", err)
	}

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
