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
	if opts.Data != nil {
		if githubOrg, ok := opts.Data["github_org"].(string); ok && githubOrg != "" {
			config.Workspace.GitHub = &workspace.GitHubConfig{Org: githubOrg}
		}
		if dockerRegistry, ok := opts.Data["docker_registry"].(string); ok && dockerRegistry != "" {
			config.Workspace.Docker = &workspace.DockerConfig{Registry: dockerRegistry}
		}
		if gcpProjectID, ok := opts.Data["gcp_project_id"].(string); ok && gcpProjectID != "" {
			config.Workspace.GCP = &workspace.GCPConfig{ProjectID: gcpProjectID}
		}
		if k8sNamespace, ok := opts.Data["k8s_namespace"].(string); ok && k8sNamespace != "" {
			config.Workspace.Kubernetes = &workspace.KubernetesConfig{Namespace: k8sNamespace}
		}
	}

	// Save forge.json
	if err := config.SaveToDir(workspaceDir); err != nil {
		return fmt.Errorf("failed to save workspace config: %w", err)
	}

	// Create directory structure
	directories := []string{
		filepath.Join(workspaceDir, "backend/services"),
		filepath.Join(workspaceDir, "frontend/projects"),
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

	fmt.Printf("✓ Workspace created successfully at: %s\n", workspaceDir)
	fmt.Printf("✓ Run 'cd %s' to enter the workspace\n", workspaceName)
	fmt.Printf("✓ Run 'forge generate service <name>' to create your first service\n")

	return nil
}
