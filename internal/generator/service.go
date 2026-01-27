package generator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// ServiceGenerator generates a new Go microservice.
type ServiceGenerator struct {
	engine *template.Engine
}

// NewServiceGenerator creates a new service generator.
func NewServiceGenerator() *ServiceGenerator {
	return &ServiceGenerator{
		engine: template.NewEngine(),
	}
}

// Name returns the generator name.
func (g *ServiceGenerator) Name() string {
	return "service"
}

// Description returns the generator description.
func (g *ServiceGenerator) Description() string {
	return "Generate a new Go microservice with Forge patterns"
}

// Generate creates a new service.
func (g *ServiceGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	serviceName := opts.Name
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}

	// Validate name
	if err := workspace.ValidateName(serviceName); err != nil {
		return fmt.Errorf("invalid service name: %w", err)
	}

	// Load workspace config (without project validation during workspace creation)
	config, err := workspace.LoadConfigWithoutProjectValidation(opts.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Check if service already exists
	if config.GetProject(serviceName) != nil {
		return fmt.Errorf("project %q already exists", serviceName)
	}

	// Determine service path using workspace.paths or default
	servicesPath := "backend/services"
	if config.Workspace.Paths != nil && config.Workspace.Paths.Services != "" {
		servicesPath = config.Workspace.Paths.Services
	}

	serviceDir := filepath.Join(opts.OutputDir, servicesPath, serviceName)

	if opts.DryRun {
		fmt.Printf("Would create service: %s\n", serviceDir)
		return nil
	}

	// Create service directory
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create service directory: %w", err)
	}

	// Prepare template data
	githubOrg := "github.com/yourorg"
	if config.Workspace.GitHub != nil {
		githubOrg = fmt.Sprintf("github.com/%s", config.Workspace.GitHub.Org)
	}

	dockerRegistry := "gcr.io/your-project"
	if config.Workspace.Docker != nil {
		dockerRegistry = config.Workspace.Docker.Registry
	}

	data := map[string]interface{}{
		"ServiceName":       serviceName,
		"ServiceNamePascal": template.Pascalize(serviceName),
		"ServiceNameCamel":  template.Camelize(serviceName),
		"ModulePath":        fmt.Sprintf("%s/%s/backend/services/%s", githubOrg, config.Workspace.Name, serviceName),
		"WorkspaceName":     config.Workspace.Name,
		"GitHubOrg":         config.Workspace.GitHub.Org, // Just the org name without github.com/
		"Registry":          dockerRegistry,
		"ProjectName":       config.Workspace.Name,
	}

	// Generate directory structure
	dirs := []string{
		"cmd/server",
		"cmd/migrator",
		"internal",
		"pkg/api",
		"pkg/model",
		"pkg/proto",
		"test",
		"deploy/helm",
		"deploy/cloudrun",
	}

	for _, dir := range dirs {
		dirPath := filepath.Join(serviceDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Generate root files
	rootTemplates := map[string]string{
		"go.mod":      "service/go.mod.tmpl",
		"BUILD.bazel": "service/BUILD.bazel.tmpl",
		"README.md":   "service/README.md.tmpl",
		"Dockerfile":  "service/Dockerfile.tmpl",
	}

	for filename, templatePath := range rootTemplates {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(serviceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Generate cmd/server files
	cmdServerTemplates := map[string]string{
		"cmd/server/main.go":       "service/cmd/server/main.go.tmpl",
		"cmd/server/BUILD.bazel":   "service/cmd/server/BUILD.bazel.tmpl",
		"cmd/migrator/doc.go":      "service/cmd/migrator/doc.go.tmpl",
		"cmd/migrator/BUILD.bazel": "service/cmd/migrator/BUILD.bazel.tmpl",
	}

	for filename, templatePath := range cmdServerTemplates {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(serviceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Generate package files (internal, pkg/*)
	pkgTemplates := map[string]string{
		"internal/doc.go":            "service/internal/doc.go.tmpl",
		"internal/BUILD.bazel":       "service/internal/BUILD.bazel.tmpl",
		"internal/entity.go":         "service/internal/entity.go.tmpl",
		"internal/transport_rest.go": "service/internal/transport_rest.go.tmpl",
		"internal/module.go":         "service/internal/module.go.tmpl",
		"pkg/api/doc.go":             "service/pkg/api/doc.go.tmpl",
		"pkg/api/BUILD.bazel":        "service/pkg/api/BUILD.bazel.tmpl",
		"pkg/model/doc.go":           "service/pkg/model/doc.go.tmpl",
		"pkg/model/BUILD.bazel":      "service/pkg/model/BUILD.bazel.tmpl",
		"pkg/proto/doc.go":           "service/pkg/proto/doc.go.tmpl",
		"pkg/proto/BUILD.bazel":      "service/pkg/proto/BUILD.bazel.tmpl",
	}

	data["EntityNamePascal"] = data["ServiceNamePascal"] // Default entity name = Service Name
	data["EntityNameCamel"] = data["ServiceNameCamel"]

	for filename, templatePath := range pkgTemplates {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(serviceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Generate test and deploy README files
	readmeTemplates := map[string]string{
		"test/README.md":   "service/test/README.md.tmpl",
		"deploy/README.md": "service/deploy/README.md.tmpl",
	}

	for filename, templatePath := range readmeTemplates {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(serviceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Get deployer from opts.Data or default to helm
	deployerTarget := "helm"
	if opts.Data != nil {
		if deployer, ok := opts.Data["deployer"].(string); ok && deployer != "" {
			deployerTarget = deployer
		}
	}

	// Generate deployment files based on selected deployer
	switch deployerTarget {
	case "helm":
		// Generate Helm values files
		helmTemplates := map[string]string{
			"deploy/helm/values.yaml":      "service/deploy/helm/values.yaml.tmpl",
			"deploy/helm/values-dev.yaml":  "service/deploy/helm/values-dev.yaml.tmpl",
			"deploy/helm/values-prod.yaml": "service/deploy/helm/values-prod.yaml.tmpl",
		}

		for filename, templatePath := range helmTemplates {
			content, err := g.engine.RenderTemplate(templatePath, data)
			if err != nil {
				return fmt.Errorf("failed to render %s: %w", filename, err)
			}

			filePath := filepath.Join(serviceDir, filename)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}

	case "cloudrun":
		// Generate Cloud Run deployment file
		cloudRunTemplate := map[string]string{
			"deploy/cloudrun/service.yaml": "service/deploy/cloudrun/service.yaml.tmpl",
		}

		for filename, templatePath := range cloudRunTemplate {
			content, err := g.engine.RenderTemplate(templatePath, data)
			if err != nil {
				return fmt.Errorf("failed to render %s: %w", filename, err)
			}

			filePath := filepath.Join(serviceDir, filename)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}
	}

	// Add project to workspace config with new architect pattern
	project := &workspace.Project{
		ProjectType: "service",
		Language:    "go",
		Root:        filepath.Join(servicesPath, serviceName),
		Tags:        []string{"backend", "service"},
		Architect: &workspace.Architect{
			Build: &workspace.ArchitectTarget{
				Builder: "@forge/bazel:build",
				Options: map[string]interface{}{
					"target":     "/...",
					"goVersion":  config.Workspace.ToolVersions.Go,
					"registry":   dockerRegistry,
					"dockerfile": "Dockerfile",
				},
				Configurations: map[string]interface{}{
					"production": map[string]interface{}{
						"optimization": true,
						"registry":     dockerRegistry,
					},
					"development": map[string]interface{}{},
					"local": map[string]interface{}{
						"race": true,
					},
				},
				DefaultConfiguration: "production",
			},
			Deploy: &workspace.ArchitectTarget{
				Deployer: fmt.Sprintf("@forge/%s:deploy", deployerTarget),
				Options: map[string]interface{}{
					"configPath": fmt.Sprintf("deploy/%s", deployerTarget),
					"namespace":  "default",
					"port":       8080,
					"healthPath": "/health",
				},
				Configurations: map[string]interface{}{
					"production": map[string]interface{}{
						"namespace": "prod",
					},
					"development": map[string]interface{}{
						"namespace": "dev",
					},
					"local": map[string]interface{}{
						"namespace": "default",
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

	if err := config.AddProject(serviceName, project); err != nil {
		return fmt.Errorf("failed to add project to config: %w", err)
	}

	if err := config.SaveToDir(opts.OutputDir); err != nil {
		return fmt.Errorf("failed to save workspace config: %w", err)
	}

	// Run go mod tidy automatically
	fmt.Printf("📦 Running go mod tidy for %s...\n", serviceName)
	if err := g.runGoModTidy(serviceDir); err != nil {
		// Warn but don't fail - user can run manually
		fmt.Printf("⚠️  Warning: go mod tidy failed: %v\n", err)
		fmt.Printf("   Run 'cd %s && go mod tidy' manually\n", serviceDir)
	} else {
		fmt.Println("✓ Dependencies synchronized")
	}

	// Update MODULE.bazel to include this service's go.mod
	if err := g.updateModuleBazel(opts.OutputDir, config); err != nil {
		return fmt.Errorf("failed to update MODULE.bazel: %w", err)
	}

	// Update go.work to include this service
	if err := g.updateGoWork(opts.OutputDir, config); err != nil {
		return fmt.Errorf("failed to update go.work: %w", err)
	}

	fmt.Printf("✓ Service %q created successfully\n", serviceName)
	fmt.Printf("✓ Location: %s\n", serviceDir)
	fmt.Printf("✓ Run 'cd %s && go mod tidy' to install dependencies\n", serviceDir)
	fmt.Printf("✓ Run 'forge build %s' to build the service\n", serviceName)
	fmt.Printf("✓ Run 'forge test %s' to run tests\n", serviceName)
	fmt.Printf("✓ Run 'forge run %s' to start the service\n", serviceName)

	return nil
}

// updateModuleBazel updates MODULE.bazel to include the new service's go.mod
func (g *ServiceGenerator) updateModuleBazel(workspaceDir string, config *workspace.Config) error {
	// Collect all services
	var services []map[string]interface{}
	for name, project := range config.Projects {
		if project.Language == "go" {
			services = append(services, map[string]interface{}{
				"Name": name,
			})
		}
	}

	// Check if frontend exists
	hasFrontend := false
	for _, project := range config.Projects {
		if project.Language == "angular" {
			hasFrontend = true
			break
		}
	}

	data := map[string]interface{}{
		"ProjectName": config.Workspace.Name,
		"Version":     "0.1.0",
		"GoVersion":   config.Workspace.ToolVersions.Go,
		"NodeVersion": "20.18.1",
		"HasFrontend": hasFrontend,
		"Services":    services,
	}

	content, err := g.engine.RenderTemplate("bazel/MODULE.bazel.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render MODULE.bazel: %w", err)
	}

	modulePath := filepath.Join(workspaceDir, "MODULE.bazel")
	if err := os.WriteFile(modulePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write MODULE.bazel: %w", err)
	}

	return nil
}

// updateGoWork updates go.work to include the new service
func (g *ServiceGenerator) updateGoWork(workspaceDir string, config *workspace.Config) error {
	// Collect all services
	var services []map[string]interface{}
	for name, project := range config.Projects {
		if project.Language == "go" {
			services = append(services, map[string]interface{}{
				"Name": name,
			})
		}
	}

	data := map[string]interface{}{
		"GoVersion": config.Workspace.ToolVersions.Go,
		"Services":  services,
	}

	content, err := g.engine.RenderTemplate("bazel/go.work.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render go.work: %w", err)
	}

	goWorkPath := filepath.Join(workspaceDir, "go.work")
	if err := os.WriteFile(goWorkPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write go.work: %w", err)
	}

	return nil
}

// runGoModTidy runs go mod tidy in the specified directory
func (g *ServiceGenerator) runGoModTidy(serviceDir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = serviceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
