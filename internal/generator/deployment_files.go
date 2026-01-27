package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// DeploymentFileGenerator generates deployment configuration files
type DeploymentFileGenerator struct {
	project     *workspace.Project
	projectName string
	config      *workspace.Config
	engine      *template.Engine
}

// NewDeploymentFileGenerator creates a new deployment file generator
func NewDeploymentFileGenerator(project *workspace.Project, projectName string, config *workspace.Config) *DeploymentFileGenerator {
	return &DeploymentFileGenerator{
		project:     project,
		projectName: projectName,
		config:      config,
		engine:      template.NewEngine(),
	}
}

// GenerateHelmValues generates Helm values files
func (g *DeploymentFileGenerator) GenerateHelmValues(deployPath string, config map[string]string) error {
	data := g.prepareTemplateData(config)

	helmTemplates := map[string]string{
		"values.yaml":      "service/deploy/helm/values.yaml.tmpl",
		"values-dev.yaml":  "service/deploy/helm/values-dev.yaml.tmpl",
		"values-prod.yaml": "service/deploy/helm/values-prod.yaml.tmpl",
		"README.md":        "service/deploy/helm/README.md.tmpl",
	}

	for filename, templatePath := range helmTemplates {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(deployPath, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// GenerateFirebaseConfig generates Firebase configuration files
func (g *DeploymentFileGenerator) GenerateFirebaseConfig(deployPath string, config map[string]string) error {
	data := g.prepareTemplateData(config)

	firebaseTemplates := map[string]string{
		"firebase.json": "frontend/deploy/firebase/firebase.json.tmpl",
		".firebaserc":   "frontend/deploy/firebase/.firebaserc.tmpl",
		"README.md":     "frontend/deploy/firebase/README.md.tmpl",
	}

	for filename, templatePath := range firebaseTemplates {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(deployPath, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// GenerateCloudRunConfig generates Cloud Run configuration files
func (g *DeploymentFileGenerator) GenerateCloudRunConfig(deployPath string, config map[string]string) error {
	data := g.prepareTemplateData(config)

	cloudRunTemplates := map[string]string{
		"service.yaml": "service/deploy/cloudrun/service.yaml.tmpl",
		"README.md":    "service/deploy/cloudrun/README.md.tmpl",
	}

	// For Angular projects, also generate Dockerfile and nginx.conf
	if g.project.Language == "angular" {
		cloudRunTemplates["Dockerfile"] = "frontend/deploy/cloudrun/Dockerfile.tmpl"
		cloudRunTemplates["nginx.conf"] = "frontend/deploy/cloudrun/nginx.conf.tmpl"
	}

	for filename, templatePath := range cloudRunTemplates {
		content, err := g.engine.RenderTemplate(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to render %s: %w", filename, err)
		}

		filePath := filepath.Join(deployPath, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// prepareTemplateData prepares data for template rendering
func (g *DeploymentFileGenerator) prepareTemplateData(config map[string]string) map[string]interface{} {
	data := map[string]interface{}{
		"ServiceName":       g.projectName,
		"ProjectName":       g.config.Workspace.Name,
		"ServiceNamePascal": template.Pascalize(g.projectName),
		"ServiceNameCamel":  template.Camelize(g.projectName),
		"Language":          g.project.Language,
		"ProjectType":       g.project.ProjectType,
	}

	// Add deployer-specific config
	for k, v := range config {
		data[k] = v
	}

	// Add workspace-level config
	if g.config.Workspace.GitHub != nil {
		data["GitHubOrg"] = g.config.Workspace.GitHub.Org
	}
	if g.config.Workspace.Docker != nil {
		data["Registry"] = g.config.Workspace.Docker.Registry
	}
	if g.config.Workspace.GCP != nil {
		data["GCPProjectID"] = g.config.Workspace.GCP.ProjectID
	}

	return data
}
