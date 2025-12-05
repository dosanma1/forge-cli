package generator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/internal/workspace"
)

// FrontendGenerator generates a new Angular application.
type FrontendGenerator struct {
	engine *template.Engine
}

// NewFrontendGenerator creates a new frontend generator.
func NewFrontendGenerator() *FrontendGenerator {
	return &FrontendGenerator{
		engine: template.NewEngine(),
	}
}

// Name returns the generator name.
func (g *FrontendGenerator) Name() string {
	return "frontend"
}

// Description returns the generator description.
func (g *FrontendGenerator) Description() string {
	return "Generate a new Angular frontend application"
}

// Generate creates a new Angular application.
func (g *FrontendGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	appName := opts.Name
	if appName == "" {
		return fmt.Errorf("application name is required")
	}

	// Validate name
	if err := workspace.ValidateName(appName); err != nil {
		return fmt.Errorf("invalid application name: %w", err)
	}

	// Load workspace config
	config, err := workspace.LoadConfig(opts.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	frontendDir := filepath.Join(opts.OutputDir, "frontend")

	if opts.DryRun {
		fmt.Printf("Would create Angular application: %s\n", appName)
		return nil
	}

	// Check if this is the first Angular app (need to initialize workspace)
	angularJsonPath := filepath.Join(frontendDir, "angular.json")
	isFirstApp := true
	if _, err := os.Stat(angularJsonPath); err == nil {
		isFirstApp = false
	}

	if isFirstApp {
		// Initialize Angular workspace using ng new
		fmt.Println("🔧 Initializing Angular workspace...")

		// Use ng new to create the workspace with proper Angular CLI setup
		// Flags match monorepo-starter configuration
		if err := g.runAngularCLI(opts.OutputDir, []string{
			"new", "frontend",
			"--directory=frontend",
			"--create-application=false", // Don't create default app
			"--routing=true",
			"--style=css",
			"--skip-git=true",
			"--package-manager=npm",
		}); err != nil {
			return fmt.Errorf("failed to initialize Angular workspace: %w", err)
		}

		// Update angular.json with schematics defaults
		if err := g.updateAngularJsonSchematics(frontendDir); err != nil {
			return fmt.Errorf("failed to update angular.json: %w", err)
		}

		// Initialize Tailwind CSS
		fmt.Println("🎨 Installing Tailwind CSS...")
		if err := g.runNpmCommand(frontendDir, []string{"install", "tailwindcss", "@tailwindcss/postcss", "postcss", "--force"}); err != nil {
			return fmt.Errorf("failed to install Tailwind: %w", err)
		}

		// Create .postcssrc.json from template
		postcssContent, err := g.engine.RenderTemplate("frontend/.postcssrc.json.tmpl", map[string]interface{}{})
		if err != nil {
			return fmt.Errorf("failed to render .postcssrc.json: %w", err)
		}
		postcssPath := filepath.Join(frontendDir, ".postcssrc.json")
		if err := os.WriteFile(postcssPath, []byte(postcssContent), 0644); err != nil {
			return fmt.Errorf("failed to create .postcssrc.json: %w", err)
		}

		// Create .npmrc from template for Bazel + pnpm compatibility
		npmrcContent, err := g.engine.RenderTemplate("frontend/.npmrc.tmpl", map[string]interface{}{})
		if err != nil {
			return fmt.Errorf("failed to render .npmrc: %w", err)
		}
		npmrcPath := filepath.Join(frontendDir, ".npmrc")
		if err := os.WriteFile(npmrcPath, []byte(npmrcContent), 0644); err != nil {
			return fmt.Errorf("failed to create .npmrc: %w", err)
		}
	}

	// Generate application using ng generate application
	fmt.Printf("📦 Generating Angular application: %s\n", appName)

	if err := g.runAngularCLI(frontendDir, []string{
		"generate", "application", appName,
		"--routing=true",
		"--style=css",
		"--skip-tests=false",
		"--standalone=true", // Use standalone components (Angular 19+)
	}); err != nil {
		return fmt.Errorf("failed to generate application: %w", err)
	}

	// Update app's styles.css with Tailwind import
	appDir := filepath.Join(frontendDir, "projects", appName)
	appStylesPath := filepath.Join(appDir, "src", "styles.css")

	stylesContent, err := g.engine.RenderTemplate("frontend/styles.css.tmpl", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("failed to render styles.css: %w", err)
	}

	if err := os.WriteFile(appStylesPath, []byte(stylesContent), 0644); err != nil {
		return fmt.Errorf("failed to update app styles.css: %w", err)
	}

	// Add project to workspace config
	project := &workspace.Project{
		Name: appName,
		Type: workspace.ProjectTypeAngularApp,
		Root: fmt.Sprintf("frontend/projects/%s", appName),
		Tags: []string{"frontend", "angular"},
	}

	if err := config.AddProject(project); err != nil {
		return fmt.Errorf("failed to add project to config: %w", err)
	}

	if err := config.SaveToDir(opts.OutputDir); err != nil {
		return fmt.Errorf("failed to save workspace config: %w", err)
	}

	fmt.Printf("✓ Angular application %q created successfully\n", appName)
	fmt.Printf("✓ Location: %s\n", appDir)
	fmt.Printf("✓ Run 'cd frontend && ng serve %s' to start the development server\n", appName)
	fmt.Printf("✓ Open http://localhost:4200 in your browser\n")

	return nil
}

// runAngularCLI executes Angular CLI commands
func (g *FrontendGenerator) runAngularCLI(workDir string, args []string) error {
	return g.runCommand(workDir, "npx", append([]string{"@angular/cli@latest"}, args...)...)
}

// runNpmCommand executes npm commands
func (g *FrontendGenerator) runNpmCommand(workDir string, args []string) error {
	return g.runCommand(workDir, "npm", args...)
}

// runNpxCommand executes npx commands
func (g *FrontendGenerator) runNpxCommand(workDir string, args []string) error {
	return g.runCommand(workDir, "npx", args...)
}

// runCommand executes a shell command
func (g *FrontendGenerator) runCommand(workDir, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Set environment variables to make Angular CLI non-interactive
	cmd.Env = append(os.Environ(),
		"NG_CLI_ANALYTICS=false", // Disable analytics prompts
		"CI=true",                // Treat as CI environment (non-interactive)
	)

	fmt.Printf("  Running: %s %v\n", command, args)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// updateAngularJsonSchematics updates angular.json with default schematics
func (g *FrontendGenerator) updateAngularJsonSchematics(frontendDir string) error {
	angularJsonPath := filepath.Join(frontendDir, "angular.json")

	// Read angular.json
	data, err := os.ReadFile(angularJsonPath)
	if err != nil {
		return fmt.Errorf("failed to read angular.json: %w", err)
	}

	// Parse JSON manually (simple string replacement approach)
	// We'll add schematics section after "version"
	schematicsConfig := `
  "schematics": {
    "@schematics/angular:component": {
      "style": "css",
      "standalone": true
    },
    "@schematics/angular:directive": {
      "standalone": true
    },
    "@schematics/angular:pipe": {
      "standalone": true
    },
    "@schematics/angular:guard": {
      "typeSeparator": "."
    },
    "@schematics/angular:interceptor": {
      "typeSeparator": "."
    },
    "@schematics/angular:resolver": {
      "typeSeparator": "."
    },
    "@schematics/angular:service": {
      "typeSeparator": "."
    }
  },`

	// Find the position after "version": "1" line
	content := string(data)
	versionLine := `"version": 1,`
	if idx := strings.Index(content, versionLine); idx != -1 {
		// Insert schematics config after version line
		newContent := content[:idx+len(versionLine)] + schematicsConfig + content[idx+len(versionLine):]
		if err := os.WriteFile(angularJsonPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write angular.json: %w", err)
		}
		fmt.Println("  ✓ Added Angular schematics defaults")
	}

	return nil
}
