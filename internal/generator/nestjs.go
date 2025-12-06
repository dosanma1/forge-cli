package generator

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/internal/workspace"
)

//go:embed templates/nestjs/BUILD.bazel.tmpl
//go:embed templates/nestjs/skaffold.yaml.tmpl
//go:embed templates/nestjs/Dockerfile.tmpl
//go:embed templates/nestjs/src/health/health.controller.ts.tmpl
//go:embed templates/nestjs/deploy/helm/values.yaml.tmpl
//go:embed templates/nestjs/deploy/cloudrun/service.yaml.tmpl
var nestjsTemplates embed.FS

// NestJSServiceGenerator generates a new NestJS microservice.
type NestJSServiceGenerator struct {
	engine *template.Engine
}

// NewNestJSServiceGenerator creates a new NestJS service generator.
func NewNestJSServiceGenerator() *NestJSServiceGenerator {
	return &NestJSServiceGenerator{
		engine: template.NewEngine(),
	}
}

// Name returns the generator name.
func (g *NestJSServiceGenerator) Name() string {
	return "nestjs-service"
}

// Description returns the generator description.
func (g *NestJSServiceGenerator) Description() string {
	return "Generate a new NestJS microservice"
}

// Generate creates a new NestJS service.
func (g *NestJSServiceGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	serviceName := opts.Name
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}

	// Check prerequisites
	if err := CheckNodeJS(); err != nil {
		return err
	}

	if err := CheckNPM(); err != nil {
		return err
	}

	// Validate name
	if err := workspace.ValidateName(serviceName); err != nil {
		return fmt.Errorf("invalid service name: %w", err)
	}

	// Get workspace root
	workspaceRoot := opts.OutputDir
	if workspaceRoot == "" {
		var err error
		workspaceRoot, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Load workspace config
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Determine service path using workspace.paths or default
	servicesPath := "backend/services"
	if config.Workspace.Paths != nil && config.Workspace.Paths.Services != "" {
		servicesPath = config.Workspace.Paths.Services
	}

	servicesDir := filepath.Join(workspaceRoot, servicesPath)
	serviceDir := filepath.Join(servicesDir, serviceName)

	// Check if service already exists
	if _, err := os.Stat(serviceDir); err == nil {
		return fmt.Errorf("service %s already exists at %s", serviceName, serviceDir)
	}

	if opts.DryRun {
		fmt.Printf("Would create NestJS service: %s at %s\n", serviceName, serviceDir)
		return nil
	}

	// Ensure services directory exists
	if err := os.MkdirAll(servicesDir, 0755); err != nil {
		return fmt.Errorf("failed to create services directory: %w", err)
	}

	// Generate NestJS project using Nest CLI
	fmt.Printf("🚀 Generating NestJS project: %s\n", serviceName)

	if err := g.runNestCLI(servicesDir, []string{
		"new", serviceName,
		"--package-manager", "npm",
		"--skip-git",
		"--strict",
	}); err != nil {
		return fmt.Errorf("failed to generate NestJS project: %w", err)
	}

	// Determine registry
	registry := "gcr.io/your-project"
	if opts.Data != nil {
		if r, ok := opts.Data["registry"].(string); ok && r != "" {
			registry = r
		}
	}
	if config.Workspace.Docker != nil && config.Workspace.Docker.Registry != "" {
		registry = config.Workspace.Docker.Registry
	}

	// Get workspace name for templates
	workspaceName := config.Workspace.Name
	if workspaceName == "" {
		workspaceName = "workspace"
	}

	// Install additional dependencies
	fmt.Println("📦 Installing additional dependencies...")
	if err := g.runNpmCommand(serviceDir, []string{"install", "@nestjs/terminus", "--save"}); err != nil {
		return fmt.Errorf("failed to install @nestjs/terminus: %w", err)
	}

	// Generate health controller using Nest CLI
	fmt.Println("🏥 Generating health check controller...")
	if err := g.runNestCLI(serviceDir, []string{"generate", "controller", "health", "--no-spec", "--flat"}); err != nil {
		return fmt.Errorf("failed to generate health controller: %w", err)
	}

	// Create deploy directories
	deployDirs := []string{
		filepath.Join(serviceDir, "deploy", "helm"),
		filepath.Join(serviceDir, "deploy", "cloudrun"),
	}
	for _, dir := range deployDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Generate Forge-specific files from templates
	data := map[string]interface{}{
		"ServiceName":   serviceName,
		"Registry":      registry,
		"WorkspaceName": workspaceName,
		"ServicesPath":  servicesPath,
	}

	forgeFiles := map[string]string{
		"BUILD.bazel":                     "templates/nestjs/BUILD.bazel.tmpl",
		"skaffold.yaml":                   "templates/nestjs/skaffold.yaml.tmpl",
		"Dockerfile":                      "templates/nestjs/Dockerfile.tmpl",
		"src/health/health.controller.ts": "templates/nestjs/src/health/health.controller.ts.tmpl",
		"deploy/helm/values.yaml":         "templates/nestjs/deploy/helm/values.yaml.tmpl",
		"deploy/cloudrun/service.yaml":    "templates/nestjs/deploy/cloudrun/service.yaml.tmpl",
	}

	for outputPath, templatePath := range forgeFiles {
		fullPath := filepath.Join(serviceDir, outputPath)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", outputPath, err)
		}

		// Read template from embedded filesystem
		templateContent, err := nestjsTemplates.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templatePath, err)
		}

		rendered, err := g.engine.Render(string(templateContent), data)
		if err != nil {
			return fmt.Errorf("failed to render template for %s: %w", outputPath, err)
		}

		if err := os.WriteFile(fullPath, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", outputPath, err)
		}
	}

	// Update app.module.ts to import TerminusModule and HealthController
	fmt.Println("🔧 Configuring health check module...")
	if err := g.updateAppModule(serviceDir); err != nil {
		return fmt.Errorf("failed to update app.module.ts: %w", err)
	}

	// Register service in forge.json
	project := workspace.Project{
		Name: serviceName,
		Type: workspace.ProjectTypeNestJSService,
		Root: filepath.Join(servicesPath, serviceName),
		Tags: []string{"backend", "nestjs", "service"},
		Build: &workspace.ProjectBuildConfig{
			NodeVersion: "22.0.0",
			Registry:    registry,
			Dockerfile:  "Dockerfile",
		},
		Deploy: &workspace.ProjectDeployConfig{
			Targets:    []string{"helm", "cloudrun"},
			ConfigPath: "deploy",
			Helm: &workspace.ProjectDeployHelm{
				Port:       3000,
				HealthPath: "/health",
			},
			CloudRun: &workspace.ProjectDeployCloudRun{
				Port:         3000,
				CPU:          "1",
				Memory:       "512Mi",
				Concurrency:  80,
				MinInstances: 0,
				MaxInstances: 10,
				Timeout:      "300s",
				HealthPath:   "/health",
			},
		},
		Local: &workspace.ProjectLocalConfig{
			CloudRun: &workspace.ProjectLocalCloudRun{
				Port: 3000,
				Env: map[string]string{
					"NODE_ENV": "development",
				},
			},
			GKE: &workspace.ProjectLocalGKE{
				Port: 3000,
				Env: map[string]string{
					"NODE_ENV": "development",
				},
			},
		},
	}

	config.Projects[serviceName] = project

	if err := config.SaveToDir(workspaceRoot); err != nil {
		return fmt.Errorf("failed to save workspace config: %w", err)
	}

	// Update root skaffold.yaml
	if err := updateRootSkaffold(workspaceRoot, servicesPath, serviceName); err != nil {
		return fmt.Errorf("failed to update root skaffold.yaml: %w", err)
	}

	fmt.Printf("\n✓ Created NestJS service: %s\n", serviceName)
	fmt.Printf("  Location: %s\n", serviceDir)
	fmt.Printf("  Registry: %s\n", registry)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. cd %s\n", filepath.Join(servicesPath, serviceName))
	fmt.Printf("  2. npm install\n")
	fmt.Printf("  3. npm run start:dev\n")
	fmt.Printf("  4. forge deploy --env=local\n")

	return nil
}

// runNestCLI executes Nest CLI commands
func (g *NestJSServiceGenerator) runNestCLI(workDir string, args []string) error {
	return g.runCommand(workDir, "npx", append([]string{"@nestjs/cli@latest"}, args...)...)
}

// runNpmCommand executes npm commands
func (g *NestJSServiceGenerator) runNpmCommand(workDir string, args []string) error {
	return g.runCommand(workDir, "npm", args...)
}

// runCommand executes a shell command
func (g *NestJSServiceGenerator) runCommand(workDir, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Set environment variables to make CLI non-interactive
	cmd.Env = append(os.Environ(),
		"CI=true", // Treat as CI environment (non-interactive)
	)

	fmt.Printf("  Running: %s %s\n", command, strings.Join(args, " "))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// updateAppModule updates app.module.ts to import TerminusModule and HealthController
func (g *NestJSServiceGenerator) updateAppModule(serviceDir string) error {
	appModulePath := filepath.Join(serviceDir, "src", "app.module.ts")

	// Read app.module.ts
	data, err := os.ReadFile(appModulePath)
	if err != nil {
		return fmt.Errorf("failed to read app.module.ts: %w", err)
	}

	content := string(data)

	// Add TerminusModule import
	if !strings.Contains(content, "@nestjs/terminus") {
		// Find the last import statement
		importLines := strings.Split(content, "\n")
		lastImportIdx := -1
		for i, line := range importLines {
			if strings.HasPrefix(strings.TrimSpace(line), "import ") {
				lastImportIdx = i
			}
		}

		if lastImportIdx != -1 {
			// Insert TerminusModule import after last import
			terminusImport := "import { TerminusModule } from '@nestjs/terminus';"
			importLines = append(importLines[:lastImportIdx+1], append([]string{terminusImport}, importLines[lastImportIdx+1:]...)...)
			content = strings.Join(importLines, "\n")
		}
	}

	// Add HealthController import
	if !strings.Contains(content, "./health/health.controller") {
		// Find the last import statement again
		importLines := strings.Split(content, "\n")
		lastImportIdx := -1
		for i, line := range importLines {
			if strings.HasPrefix(strings.TrimSpace(line), "import ") {
				lastImportIdx = i
			}
		}

		if lastImportIdx != -1 {
			// Insert HealthController import after last import
			healthImport := "import { HealthController } from './health/health.controller';"
			importLines = append(importLines[:lastImportIdx+1], append([]string{healthImport}, importLines[lastImportIdx+1:]...)...)
			content = strings.Join(importLines, "\n")
		}
	}

	// Add TerminusModule to imports array
	if !strings.Contains(content, "TerminusModule") {
		content = strings.Replace(content, "imports: [", "imports: [TerminusModule, ", 1)
	}

	// Add HealthController to controllers array
	if !strings.Contains(content, "HealthController") {
		content = strings.Replace(content, "controllers: [", "controllers: [HealthController, ", 1)
	}

	// Write updated app.module.ts
	if err := os.WriteFile(appModulePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write app.module.ts: %w", err)
	}

	return nil
}
