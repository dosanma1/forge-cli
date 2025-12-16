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

	// Load workspace config (without project validation during workspace creation)
	config, err := workspace.LoadConfigWithoutProjectValidation(workspaceRoot)
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

	if err := g.runNestJSCLI(servicesDir, config, []string{
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

	// Get deployer from opts.Data or default to helm
	deployerTarget := "helm"
	if opts.Data != nil {
		if deployer, ok := opts.Data["deployer"].(string); ok && deployer != "" {
			deployerTarget = deployer
		}
	}

	// Create deploy directory for selected deployer only
	deployDir := filepath.Join(serviceDir, "deploy", deployerTarget)
	if err := os.MkdirAll(deployDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", deployDir, err)
	}

	// Generate Forge-specific files from templates
	data := map[string]interface{}{
		"ServiceName":   serviceName,
		"Registry":      registry,
		"WorkspaceName": workspaceName,
		"ServicesPath":  servicesPath,
	}

	// Base files that are always generated
	forgeFiles := map[string]string{
		"BUILD.bazel":                     "BUILD.bazel.tmpl",
		"Dockerfile":                      "Dockerfile.tmpl",
		"src/health/health.controller.ts": "src/health/health.controller.ts.tmpl",
	}

	// Add deployer-specific files
	switch deployerTarget {
	case "helm":
		forgeFiles["deploy/helm/values.yaml"] = "deploy/helm/values.yaml.tmpl"
	case "cloudrun":
		forgeFiles["deploy/cloudrun/service.yaml"] = "deploy/cloudrun/service.yaml.tmpl"
	}

	for outputPath, templatePath := range forgeFiles {
		fullPath := filepath.Join(serviceDir, outputPath)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", outputPath, err)
		}

		// Read template from embedded filesystem
		templateContent, err := template.TemplatesFS.ReadFile("templates/nestjs/" + templatePath)
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
		ProjectType: "service",
		Language:    "nestjs",
		Root:        filepath.Join(servicesPath, serviceName),
		Tags:        []string{"backend", "nestjs", "service"},
		Architect: &workspace.Architect{
			Build: &workspace.ArchitectTarget{
				Builder: "@forge/bazel:build",
				Options: map[string]interface{}{
					"target":      ":image_tarball.tar",
					"nodeVersion": "22.0.0",
					"registry":    registry,
					"dockerfile":  "Dockerfile",
				},
				Configurations: map[string]interface{}{
					"development": map[string]interface{}{},
					"local":       map[string]interface{}{},
					"production": map[string]interface{}{
						"optimization": true,
						"registry":     registry,
					},
				},
				DefaultConfiguration: "production",
			},
			Serve: &workspace.ArchitectTarget{
				Builder: "@forge/nestjs:serve",
				Options: map[string]interface{}{
					"port": 3000,
				},
			},
			Deploy: &workspace.ArchitectTarget{
				Deployer: fmt.Sprintf("@forge/%s:deploy", deployerTarget),
				Options: map[string]interface{}{
					"configPath": fmt.Sprintf("deploy/%s", deployerTarget),
					"healthPath": "/health",
					"namespace":  "default",
					"port":       3000,
				},
				Configurations: map[string]interface{}{
					"development": map[string]interface{}{
						"namespace": "dev",
					},
					"local": map[string]interface{}{
						"namespace": "default",
					},
					"production": map[string]interface{}{
						"namespace": "prod",
					},
				},
				DefaultConfiguration: "production",
			},
		},
		Metadata: map[string]interface{}{
			"deployment": map[string]interface{}{
				"target": deployerTarget,
			},
		},
	}

	config.Projects[serviceName] = project

	if err := config.SaveToDir(workspaceRoot); err != nil {
		return fmt.Errorf("failed to save workspace config: %w", err)
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

// runNestJSCLI executes NestJS CLI commands
func (g *NestJSServiceGenerator) runNestJSCLI(workDir string, config *workspace.Config, args []string) error {
	nestjsVersion := "10.4.9" // default
	if config.Workspace.ToolVersions != nil && config.Workspace.ToolVersions.NestJS != "" {
		nestjsVersion = config.Workspace.ToolVersions.NestJS
	}
	return g.runCommand(workDir, "npx", append([]string{fmt.Sprintf("@nestjs/cli@%s", nestjsVersion)}, args...)...)
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
	lines := strings.Split(content, "\n")

	// Find the last import statement
	lastImportIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "import ") {
			lastImportIdx = i
		}
	}

	// Add imports after the last import statement
	if lastImportIdx != -1 {
		newImports := []string{}

		// Add TerminusModule import if not present
		if !strings.Contains(content, "@nestjs/terminus") {
			newImports = append(newImports, "import { TerminusModule } from '@nestjs/terminus';")
		}

		// Add HealthController import if not present
		if !strings.Contains(content, "./health/health.controller") {
			newImports = append(newImports, "import { HealthController } from './health/health.controller';")
		}

		if len(newImports) > 0 {
			lines = append(lines[:lastImportIdx+1], append(newImports, lines[lastImportIdx+1:]...)...)
			content = strings.Join(lines, "\n")
		}
	}

	// Add TerminusModule to imports array if not already there
	// Check if TerminusModule is imported but not in the imports array
	if strings.Contains(content, "import { TerminusModule }") && !strings.Contains(content, "imports: [TerminusModule") {
		// Check if imports array is empty or has items
		if strings.Contains(content, "imports: []") {
			content = strings.Replace(content, "imports: []", "imports: [TerminusModule]", 1)
		} else {
			content = strings.Replace(content, "imports: [", "imports: [TerminusModule, ", 1)
		}
	}

	// Add HealthController to controllers array if not already there
	if strings.Contains(content, "controllers: [") && strings.Contains(content, "HealthController") && !strings.Contains(content, "controllers: [HealthController") && !strings.Contains(content, "controllers: [AppController, HealthController") {
		content = strings.Replace(content, "controllers: [AppController", "controllers: [AppController, HealthController", 1)
	}

	// Write updated app.module.ts
	if err := os.WriteFile(appModulePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write app.module.ts: %w", err)
	}

	return nil
}
