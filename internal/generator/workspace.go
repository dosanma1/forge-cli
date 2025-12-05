package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/internal/workspace"
)

// WorkspaceGenerator generates a new Forge workspace.
type WorkspaceGenerator struct {
	engine *template.Engine
}

// NewWorkspaceGenerator creates a new workspace generator.
func NewWorkspaceGenerator() *WorkspaceGenerator {
	return &WorkspaceGenerator{
		engine: template.NewEngine(),
	}
}

// Name returns the generator name.
func (g *WorkspaceGenerator) Name() string {
	return "workspace"
}

// Description returns the generator description.
func (g *WorkspaceGenerator) Description() string {
	return "Generate a new Forge workspace with initial structure"
}

// Generate creates a new workspace.
func (g *WorkspaceGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	workspaceName := opts.Name
	if workspaceName == "" {
		return fmt.Errorf("workspace name is required")
	}

	// Validate name
	if err := workspace.ValidateName(workspaceName); err != nil {
		return fmt.Errorf("invalid workspace name: %w", err)
	}

	workspaceDir := filepath.Join(opts.OutputDir, workspaceName)

	// Check if directory already exists
	if _, err := os.Stat(workspaceDir); err == nil {
		return fmt.Errorf("directory %s already exists", workspaceDir)
	}

	if opts.DryRun {
		fmt.Printf("Would create workspace: %s\n", workspaceDir)
		return nil
	}

	// Create workspace directory
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create workspace configuration
	config := workspace.NewConfig(workspaceName)

	// Extract optional metadata from Data
	var dockerRegistry, gcpProjectID, k8sNamespace string
	if opts.Data != nil {
		if githubOrg, ok := opts.Data["github_org"].(string); ok && githubOrg != "" {
			config.Workspace.GitHub = &workspace.GitHubConfig{Org: githubOrg}
		}
		if registry, ok := opts.Data["docker_registry"].(string); ok && registry != "" {
			dockerRegistry = registry
			config.Workspace.Docker = &workspace.DockerConfig{Registry: registry}
		}
		if projectID, ok := opts.Data["gcp_project_id"].(string); ok && projectID != "" {
			gcpProjectID = projectID
			config.Workspace.GCP = &workspace.GCPConfig{ProjectID: projectID}
		}
		if namespace, ok := opts.Data["k8s_namespace"].(string); ok && namespace != "" {
			k8sNamespace = namespace
			config.Workspace.Kubernetes = &workspace.KubernetesConfig{Namespace: namespace}
		}
	}

	// Set default registry if not provided
	if dockerRegistry == "" {
		if gcpProjectID != "" {
			dockerRegistry = fmt.Sprintf("gcr.io/%s", gcpProjectID)
		} else {
			dockerRegistry = fmt.Sprintf("gcr.io/%s", workspaceName)
		}
	}

	// Set default namespace
	if k8sNamespace == "" {
		k8sNamespace = "default"
	}

	// Initialize build configuration
	config.Build = &workspace.BuildConfig{
		GoVersion:   "1.23",
		NodeVersion: "20.18.1",
		Registry:    dockerRegistry,
		Cache: &workspace.CacheConfig{
			RemoteURL: "", // User can configure this
		},
		Parallel: &workspace.ParallelConfig{
			Workers: 0, // Auto-detect
		},
	}

	// Initialize infrastructure configuration
	config.Infrastructure = &workspace.InfrastructureConfig{
		Kubernetes: &workspace.KubernetesInfra{
			Cluster:   fmt.Sprintf("kind-%s", workspaceName),
			Namespace: k8sNamespace,
		},
		CloudRun: &workspace.CloudRunInfra{
			Region:  "us-central1",
			Project: gcpProjectID,
		},
	}

	// Initialize default environments
	config.Environments = map[string]workspace.EnvironmentConfig{
		"local": {
			Name:        "local",
			Description: "Local development (kind/minikube)",
			Cluster:     fmt.Sprintf("kind-%s", workspaceName),
			Namespace:   "default",
			Profile:     "",
		},
		"dev": {
			Name:        "dev",
			Description: "Development environment",
			Cluster:     "your-dev-cluster",
			Namespace:   "dev",
			Profile:     "dev",
			Registry:    dockerRegistry,
		},
		"staging": {
			Name:        "staging",
			Description: "Staging environment",
			Cluster:     "your-staging-cluster",
			Namespace:   "staging",
			Profile:     "staging",
			Registry:    dockerRegistry,
		},
		"prod": {
			Name:        "prod",
			Description: "Production environment",
			Cluster:     "your-prod-cluster",
			Namespace:   "production",
			Profile:     "prod",
			Registry:    dockerRegistry,
			Region:      "us-central1",
		},
	}

	// Save forge.json
	if err := config.SaveToDir(workspaceDir); err != nil {
		return fmt.Errorf("failed to save workspace config: %w", err)
	}

	// Create directory structure (without frontend for now)
	directories := []string{
		filepath.Join(workspaceDir, "backend/services"),
		filepath.Join(workspaceDir, "infra/helm"),
		filepath.Join(workspaceDir, "infra/cloudrun"),
		filepath.Join(workspaceDir, "shared"),
		filepath.Join(workspaceDir, "docs"),
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create README.md
	readmeContent := fmt.Sprintf(`# %s

A Forge workspace for building production-ready microservices.

## Getting Started

### Prerequisites

- Go 1.23+
- Node.js 20+
- Bazel 7+
- Docker

### Project Structure

`+"```"+`
%s/
├── forge.json          # Workspace configuration
├── backend/            # Backend services
│   └── services/       # Microservices
├── frontend/           # Frontend applications
│   └── projects/       # Angular projects
├── infra/              # Infrastructure
│   ├── helm/           # Kubernetes Helm charts
│   └── cloudrun/       # Cloud Run configurations
├── shared/             # Shared libraries
└── docs/               # Documentation
`+"```"+`

### Commands

`+"```bash"+`
# Generate a new Go service
forge generate service <service-name>

# Generate a new Angular application
forge generate frontend <app-name>

# Add a handler to a service
forge add handler <service-name> <endpoint>

# Add middleware to a service
forge add middleware <service-name> <middleware-type>
`+"```"+`

## Documentation

See [docs/](./docs/) for detailed documentation.
`, workspaceName, workspaceName)

	readmePath := filepath.Join(workspaceDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	// Create .gitignore
	gitignoreContent := `# Bazel
bazel-*

# Go
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
go.work.sum

# Node
node_modules/
dist/
.angular/

# IDEs
.vscode/
.idea/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Env files
.env
.env.local
*.local
`

	gitignorePath := filepath.Join(workspaceDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	// Track created services and frontend for Bazel config
	var createdServices []string
	hasFrontend := false

	// Initial Bazel configuration (will be updated after services are created)
	// Pass the github org from the config we just created
	githubOrg := "myorg"
	if config.Workspace.GitHub != nil {
		githubOrg = config.Workspace.GitHub.Org
	}
	if err := g.generateBazelFilesWithOrg(workspaceDir, workspaceName, hasFrontend, createdServices, githubOrg); err != nil {
		return fmt.Errorf("failed to generate Bazel files: %w", err)
	}

	// Note: forge.json is now the single source of truth (already created above)
	// No need for separate .forge.yaml file

	// Generate GitHub Actions workflows
	if err := g.generateGitHubWorkflows(workspaceDir); err != nil {
		return fmt.Errorf("failed to generate GitHub workflows: %w", err)
	}

	// Generate infrastructure templates (needs workspace config first)
	if err := g.generateInfrastructure(workspaceDir); err != nil {
		return fmt.Errorf("failed to generate infrastructure: %w", err)
	}

	// Generate backend service if requested
	if opts.Data != nil {
		if createBackend, ok := opts.Data["create_backend"].(bool); ok && createBackend {
			backendServiceName := "api-server" // default
			if serviceName, ok := opts.Data["backend_service_name"].(string); ok && serviceName != "" {
				backendServiceName = serviceName
			}

			fmt.Printf("\n🚀 Generating backend service: %s\n", backendServiceName)

			serviceGen := NewServiceGenerator()
			serviceOpts := GeneratorOptions{
				OutputDir: workspaceDir,
				Name:      backendServiceName,
				DryRun:    false,
			}

			if err := serviceGen.Generate(ctx, serviceOpts); err != nil {
				return fmt.Errorf("failed to generate backend service: %w", err)
			}

			createdServices = append(createdServices, backendServiceName)
		}
	}

	// Generate frontend if requested
	if opts.Data != nil {
		if createFrontend, ok := opts.Data["create_frontend"].(bool); ok && createFrontend {
			frontendAppName := "web-app" // default
			if appName, ok := opts.Data["frontend_app_name"].(string); ok && appName != "" {
				frontendAppName = appName
			}

			fmt.Printf("\n🎨 Generating Angular frontend application: %s\n", frontendAppName)

			hasFrontend = true

			frontendGen := NewFrontendGenerator()
			frontendOpts := GeneratorOptions{
				OutputDir: workspaceDir,
				Name:      frontendAppName,
				DryRun:    false,
			}

			if err := frontendGen.Generate(ctx, frontendOpts); err != nil {
				return fmt.Errorf("failed to generate frontend: %w", err)
			}
		}
	}

	// Update go.work to include generated services
	if len(createdServices) > 0 {
		if err := g.updateGoWork(workspaceDir, createdServices); err != nil {
			return fmt.Errorf("failed to update go.work: %w", err)
		}
	}

	// Regenerate MODULE.bazel with services and frontend info
	if len(createdServices) > 0 || hasFrontend {
		if err := g.generateBazelFilesWithOrg(workspaceDir, workspaceName, hasFrontend, createdServices, githubOrg); err != nil {
			return fmt.Errorf("failed to regenerate Bazel files: %w", err)
		}
	}

	fmt.Printf("\n✓ Workspace created successfully at: %s\n", workspaceDir)
	fmt.Printf("✓ Run 'cd %s' to enter the workspace\n", workspaceName)
	fmt.Printf("✓ Run 'forge setup' to install Bazel\n")

	// Show relevant next steps based on what was created
	if opts.Data != nil {
		if createBackend, ok := opts.Data["create_backend"].(bool); ok && createBackend {
			if serviceName, ok := opts.Data["backend_service_name"].(string); ok && serviceName != "" {
				fmt.Printf("✓ Backend service '%s' created\n", serviceName)
			}
		}
		if createFrontend, ok := opts.Data["create_frontend"].(bool); ok && createFrontend {
			if appName, ok := opts.Data["frontend_app_name"].(string); ok && appName != "" {
				fmt.Printf("✓ Frontend application '%s' created\n", appName)
				fmt.Printf("✓ Run 'cd frontend && ng serve %s' to start your frontend\n", appName)
			}
		} else {
			fmt.Printf("✓ Run 'forge generate frontend <name>' to create a frontend app\n")
		}
		if createBackend, ok := opts.Data["create_backend"].(bool); !ok || !createBackend {
			fmt.Printf("✓ Run 'forge generate service <name>' to create a backend service\n")
		}
	}

	return nil
}

// generateBazelFiles creates Bazel configuration files
func (g *WorkspaceGenerator) generateBazelFilesWithOrg(workspaceDir, workspaceName string, hasFrontend bool, services []string, githubOrg string) error {
	files := map[string]string{
		"MODULE.bazel":  "bazel/MODULE.bazel.tmpl",
		"BUILD.bazel":   "bazel/BUILD.bazel.tmpl",
		".bazelrc":      "bazel/.bazelrc.tmpl",
		".bazelignore":  "bazel/.bazelignore.tmpl",
		".bazelversion": "bazel/.bazelversion.tmpl",
	}

	// Convert services to map format for template
	var servicesData []map[string]interface{}
	for _, name := range services {
		servicesData = append(servicesData, map[string]interface{}{
			"Name": name,
		})
	}

	data := map[string]interface{}{
		"ProjectName": workspaceName,
		"Version":     "0.1.0",
		"GoVersion":   "1.23",
		"NodeVersion": "20.18.1",
		"HasFrontend": hasFrontend,
		"Services":    servicesData,
		"GitHubOrg":   githubOrg,
	}

	for filename, templatePath := range files {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(workspaceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// generateForgeConfig creates .forge.yaml
func (g *WorkspaceGenerator) generateForgeConfig(workspaceDir, workspaceName string) error {
	data := map[string]interface{}{
		"ProjectName":    workspaceName,
		"GoVersion":      "1.23",
		"NodeVersion":    "20.18.1",
		"DockerRegistry": fmt.Sprintf("gcr.io/%s", workspaceName),
		"GCPProjectID":   workspaceName,
		"K8sNamespace":   "default",
	}

	content, err := g.engine.RenderTemplate(".forge.yaml.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render .forge.yaml: %w", err)
	}

	filePath := filepath.Join(workspaceDir, ".forge.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write .forge.yaml: %w", err)
	}

	return nil
}

// generateGitHubWorkflows creates GitHub Actions workflow files
func (g *WorkspaceGenerator) generateGitHubWorkflows(workspaceDir string) error {
	workflowsDir := filepath.Join(workspaceDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflows directory: %w", err)
	}

	workflows := map[string]string{
		"ci.yml":              "github/workflows/ci.yml.tmpl",
		"deploy-k8s.yml":      "github/workflows/deploy-k8s.yml.tmpl",
		"deploy-cloudrun.yml": "github/workflows/deploy-cloudrun.yml.tmpl",
	}

	data := map[string]interface{}{
		"GitHubOrg": "myorg", // Default, user can update manually
	}

	for filename, templatePath := range workflows {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(workflowsDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// generateInfrastructure creates infrastructure configuration files
func (g *WorkspaceGenerator) generateInfrastructure(workspaceDir string) error {
	infraDir := filepath.Join(workspaceDir, "infra")

	// Load workspace config to get project name
	config, err := workspace.LoadConfig(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	projectName := config.Workspace.Name
	registry := "gcr.io/your-project"
	if config.Workspace.Docker != nil {
		registry = config.Workspace.Docker.Registry
	}

	// Create kind-config.yaml
	kindData := map[string]interface{}{
		"ProjectName": projectName,
	}
	kindContent, err := g.engine.RenderTemplate("infra/kind-config.yaml.tmpl", kindData)
	if err != nil {
		return fmt.Errorf("failed to render kind-config.yaml: %w", err)
	}

	kindPath := filepath.Join(infraDir, "kind-config.yaml")
	if err := os.WriteFile(kindPath, []byte(kindContent), 0644); err != nil {
		return fmt.Errorf("failed to write kind-config.yaml: %w", err)
	}

	// Create skaffold.yaml
	skaffoldData := map[string]interface{}{
		"ProjectName":   projectName,
		"Services":      []map[string]interface{}{}, // Empty initially
		"HasAPIGateway": false,
		"Registry":      registry,
	}
	skaffoldContent, err := g.engine.RenderTemplate("skaffold.yaml.tmpl", skaffoldData)
	if err != nil {
		return fmt.Errorf("failed to render skaffold.yaml: %w", err)
	}

	skaffoldPath := filepath.Join(workspaceDir, "skaffold.yaml")
	if err := os.WriteFile(skaffoldPath, []byte(skaffoldContent), 0644); err != nil {
		return fmt.Errorf("failed to write skaffold.yaml: %w", err)
	}

	// Create helm directory with README
	helmDir := filepath.Join(infraDir, "helm")
	if err := os.MkdirAll(helmDir, 0755); err != nil {
		return fmt.Errorf("failed to create helm directory: %w", err)
	}

	helmReadmeData := map[string]interface{}{
		"ProjectName": projectName,
	}
	helmReadmeContent, err := g.engine.RenderTemplate("infra/helm/README.md.tmpl", helmReadmeData)
	if err != nil {
		return fmt.Errorf("failed to render helm README: %w", err)
	}

	helmReadmePath := filepath.Join(helmDir, "README.md")
	if err := os.WriteFile(helmReadmePath, []byte(helmReadmeContent), 0644); err != nil {
		return fmt.Errorf("failed to write helm README: %w", err)
	}

	// Create helm/service directory structure with generic chart
	helmServiceDir := filepath.Join(helmDir, "service")
	if err := os.MkdirAll(helmServiceDir, 0755); err != nil {
		return fmt.Errorf("failed to create helm/service directory: %w", err)
	}

	helmTemplatesDir := filepath.Join(helmServiceDir, "templates")
	if err := os.MkdirAll(helmTemplatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create helm/service/templates directory: %w", err)
	}

	// Generate Chart.yaml
	chartData := map[string]interface{}{
		"ProjectName": projectName,
	}
	chartContent, err := g.engine.RenderTemplate("infra/helm/service/Chart.yaml.tmpl", chartData)
	if err != nil {
		return fmt.Errorf("failed to render Chart.yaml: %w", err)
	}
	chartPath := filepath.Join(helmServiceDir, "Chart.yaml")
	if err := os.WriteFile(chartPath, []byte(chartContent), 0644); err != nil {
		return fmt.Errorf("failed to write Chart.yaml: %w", err)
	}

	// Generate values.yaml
	valuesContent, err := g.engine.RenderTemplate("infra/helm/service/values.yaml.tmpl", chartData)
	if err != nil {
		return fmt.Errorf("failed to render values.yaml: %w", err)
	}
	valuesPath := filepath.Join(helmServiceDir, "values.yaml")
	if err := os.WriteFile(valuesPath, []byte(valuesContent), 0644); err != nil {
		return fmt.Errorf("failed to write values.yaml: %w", err)
	}

	// Copy Helm template files (these are standard Helm templates, not Go templates)
	helmTemplateFiles := []string{
		"_helpers.tpl",
		"NOTES.txt",
		"configmap.yaml",
		"deployment.yaml",
		"hpa.yaml",
		"ingress.yaml",
		"pdb.yaml",
		"secret.yaml",
		"service.yaml",
		"serviceaccount.yaml",
	}

	for _, filename := range helmTemplateFiles {
		templatePath := fmt.Sprintf("infra/helm/service/templates/%s", filename)
		content, err := g.engine.ReadEmbeddedFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filename, err)
		}
		filePath := filepath.Join(helmTemplatesDir, filename)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Create cloudrun directory with README
	cloudrunDir := filepath.Join(infraDir, "cloudrun")
	if err := os.MkdirAll(cloudrunDir, 0755); err != nil {
		return fmt.Errorf("failed to create cloudrun directory: %w", err)
	}

	cloudrunReadmeData := map[string]interface{}{
		"ProjectName": projectName,
	}
	cloudrunReadmeContent, err := g.engine.RenderTemplate("infra/cloudrun/README.md.tmpl", cloudrunReadmeData)
	if err != nil {
		return fmt.Errorf("failed to render cloudrun README: %w", err)
	}

	cloudrunReadmePath := filepath.Join(cloudrunDir, "README.md")
	if err := os.WriteFile(cloudrunReadmePath, []byte(cloudrunReadmeContent), 0644); err != nil {
		return fmt.Errorf("failed to write cloudrun README: %w", err)
	}

	// Create api-gateway Helm chart
	if err := g.generateAPIGateway(workspaceDir, projectName); err != nil {
		return fmt.Errorf("failed to generate API gateway: %w", err)
	}

	return nil
}

// generateAPIGateway creates the API gateway Helm chart infrastructure
func (g *WorkspaceGenerator) generateAPIGateway(workspaceDir, projectName string) error {
	apiGatewayDir := filepath.Join(workspaceDir, "infra", "api-gateway")

	// Create directory structure
	dirs := []string{
		apiGatewayDir,
		filepath.Join(apiGatewayDir, "templates"),
		filepath.Join(apiGatewayDir, "envs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	data := map[string]interface{}{
		"WorkspaceName": projectName,
		"Timestamp":     "2025-01-01T00:00:00Z", // Use current timestamp in production
	}

	// Generate root files
	rootFiles := map[string]string{
		"Chart.yaml":    "infra/api-gateway/Chart.yaml.tmpl",
		"Chart.lock":    "infra/api-gateway/Chart.lock.tmpl",
		"values.yaml":   "infra/api-gateway/values.yaml.tmpl",
		"README.md":     "infra/api-gateway/README.md.tmpl",
		"skaffold.yaml": "infra/api-gateway/skaffold.yaml.tmpl",
	}

	for filename, templatePath := range rootFiles {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(apiGatewayDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Generate templates
	templateFiles := map[string]string{
		"templates/_helpers.tpl":     "infra/api-gateway/templates/_helpers.tpl.tmpl",
		"templates/ingress.yaml":     "infra/api-gateway/templates/ingress.yaml.tmpl",
		"templates/cert-issuer.yaml": "infra/api-gateway/templates/cert-issuer.yaml.tmpl",
		"templates/certificate.yaml": "infra/api-gateway/templates/certificate.yaml.tmpl",
	}

	for filename, templatePath := range templateFiles {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(apiGatewayDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Generate environment files
	envFiles := map[string]string{
		"envs/local.yaml": "infra/api-gateway/envs/local.yaml.tmpl",
	}

	for filename, templatePath := range envFiles {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(apiGatewayDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// updateGoWork updates go.work to include generated services
func (g *WorkspaceGenerator) updateGoWork(workspaceDir string, serviceNames []string) error {
	// Prepare service data for template
	var services []map[string]interface{}
	for _, name := range serviceNames {
		services = append(services, map[string]interface{}{
			"Name": name,
		})
	}

	data := map[string]interface{}{
		"GoVersion": "1.23",
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
