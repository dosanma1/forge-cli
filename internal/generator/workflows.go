package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// WorkflowGenerator generates and updates GitHub Actions workflows
type WorkflowGenerator struct {
	config        *workspace.Config
	workspaceRoot string
	engine        *template.Engine
}

// NewWorkflowGenerator creates a new workflow generator
func NewWorkflowGenerator(config *workspace.Config, workspaceRoot string) *WorkflowGenerator {
	return &WorkflowGenerator{
		config:        config,
		workspaceRoot: workspaceRoot,
		engine:        template.NewEngine(),
	}
}

// UpdateWorkflows updates GitHub Actions workflows based on active deployers
func (g *WorkflowGenerator) UpdateWorkflows() error {
	// Scan all projects to collect active deployers
	activeDeployers := g.collectActiveDeployers()

	workflowsDir := filepath.Join(g.workspaceRoot, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflows directory: %w", err)
	}

	// Always generate ci.yml
	if err := g.generateWorkflow("ci.yml", "github/workflows/ci.yml.tmpl", nil); err != nil {
		return err
	}

	// Generate deployer-specific workflows only if they're used
	deployerWorkflows := map[string]string{
		"helm":     "deploy-gke.yml",
		"firebase": "deploy-firebase.yml",
		"cloudrun": "deploy-cloudrun.yml",
	}

	for deployer, workflowFile := range deployerWorkflows {
		workflowPath := filepath.Join(workflowsDir, workflowFile)

		if activeDeployers[deployer] {
			// Generate workflow if deployer is active
			templatePath := fmt.Sprintf("github/workflows/%s.tmpl", workflowFile)
			if err := g.generateWorkflow(workflowFile, templatePath, nil); err != nil {
				return err
			}
			fmt.Printf("  ✓ Generated %s (deployer in use)\n", workflowFile)
		} else {
			// Remove workflow if deployer is not used
			if _, err := os.Stat(workflowPath); err == nil {
				if err := os.Remove(workflowPath); err != nil {
					return fmt.Errorf("failed to remove %s: %w", workflowFile, err)
				}
				fmt.Printf("  ✓ Removed %s (deployer not in use)\n", workflowFile)
			}
		}
	}

	return nil
}

// collectActiveDeployers scans all projects and returns a set of active deployers
func (g *WorkflowGenerator) collectActiveDeployers() map[string]bool {
	deployers := make(map[string]bool)

	for _, project := range g.config.Projects {
		if project.Architect != nil && project.Architect.Deploy != nil {
			deployerName := extractDeployerName(project.Architect.Deploy.Deployer)
			if deployerName != "" {
				deployers[deployerName] = true
			}
		}
	}

	return deployers
}

// generateWorkflow generates a single workflow file
func (g *WorkflowGenerator) generateWorkflow(filename, templatePath string, data map[string]interface{}) error {
	if data == nil {
		data = make(map[string]interface{})
	}

	// Add workspace-level data
	if g.config.Workspace.GitHub != nil {
		data["GitHubOrg"] = g.config.Workspace.GitHub.Org
	}

	content, err := g.engine.RenderTemplate(templatePath, data)
	if err != nil {
		return fmt.Errorf("failed to render %s: %w", filename, err)
	}

	workflowPath := filepath.Join(g.workspaceRoot, ".github", "workflows", filename)
	if err := os.WriteFile(workflowPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}

	return nil
}

// extractDeployerName extracts the deployer name from a deployer string like "@forge/helm:deploy"
func extractDeployerName(deployer string) string {
	// Parse @forge/<name>:deploy
	if len(deployer) > 7 && deployer[:7] == "@forge/" {
		end := len(deployer)
		if colonIdx := strings.LastIndex(deployer, ":"); colonIdx != -1 {
			end = colonIdx
		}
		return deployer[7:end]
	}
	return deployer
}
