package generator

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/internal/workspace"
)

//go:embed templates/nestjs/*
//go:embed templates/nestjs/**/*
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

	serviceDir := filepath.Join(workspaceRoot, servicesPath, serviceName)

	// Check if service already exists
	if _, err := os.Stat(serviceDir); err == nil {
		return fmt.Errorf("service %s already exists at %s", serviceName, serviceDir)
	}

	if opts.DryRun {
		fmt.Printf("Would create NestJS service: %s at %s\n", serviceName, serviceDir)
		return nil
	}

	// Create service directory structure
	dirs := []string{
		serviceDir,
		filepath.Join(serviceDir, "src"),
		filepath.Join(serviceDir, "src", "health"),
		filepath.Join(serviceDir, "test"),
		filepath.Join(serviceDir, "deploy", "helm"),
		filepath.Join(serviceDir, "deploy", "cloudrun"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
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

	// Generate files
	data := map[string]interface{}{
		"ServiceName":   serviceName,
		"Registry":      registry,
		"WorkspaceName": workspaceName,
		"ServicesPath":  servicesPath,
	}

	files := map[string]string{
		"BUILD.bazel":                     "templates/nestjs/BUILD.bazel.tmpl",
		"skaffold.yaml":                   "templates/nestjs/skaffold.yaml.tmpl",
		"package.json":                    "templates/nestjs/package.json.tmpl",
		"tsconfig.json":                   "templates/nestjs/tsconfig.json.tmpl",
		"nest-cli.json":                   "templates/nestjs/nest-cli.json.tmpl",
		".eslintrc.js":                    "templates/nestjs/.eslintrc.js.tmpl",
		".prettierrc":                     "templates/nestjs/.prettierrc.tmpl",
		"Dockerfile":                      "templates/nestjs/Dockerfile.tmpl",
		"README.md":                       "templates/nestjs/README.md.tmpl",
		"src/main.ts":                     "templates/nestjs/src/main.ts.tmpl",
		"src/app.module.ts":               "templates/nestjs/src/app.module.ts.tmpl",
		"src/app.controller.ts":           "templates/nestjs/src/app.controller.ts.tmpl",
		"src/app.service.ts":              "templates/nestjs/src/app.service.ts.tmpl",
		"src/health/health.controller.ts": "templates/nestjs/src/health/health.controller.ts.tmpl",
		"test/app.e2e-spec.ts":            "templates/nestjs/test/app.e2e-spec.ts.tmpl",
		"test/jest-e2e.json":              "templates/nestjs/test/jest-e2e.json.tmpl",
		"deploy/helm/values.yaml":         "templates/nestjs/deploy/helm/values.yaml.tmpl",
		"deploy/cloudrun/service.yaml":    "templates/nestjs/deploy/cloudrun/service.yaml.tmpl",
	}

	for outputPath, templatePath := range files {
		fullPath := filepath.Join(serviceDir, outputPath)
		
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

	fmt.Printf("✓ Created NestJS service: %s\n", serviceName)
	fmt.Printf("  Location: %s\n", serviceDir)
	fmt.Printf("  Registry: %s\n", registry)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. cd %s\n", filepath.Join(servicesPath, serviceName))
	fmt.Printf("  2. npm install\n")
	fmt.Printf("  3. npm run start:dev\n")
	fmt.Printf("  4. forge deploy --env=local\n")

	return nil
}

