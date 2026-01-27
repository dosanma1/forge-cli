package deployer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/generator"
	"github.com/dosanma1/forge-cli/internal/ui"
	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// SwitcherOptions contains options for switching deployers
type SwitcherOptions struct {
	Config         *workspace.Config
	ProjectName    string
	Project        *workspace.Project
	TargetDeployer string
	DeployerConfig map[string]string
	Force          bool
	WorkspaceRoot  string
}

// Switcher handles switching deployment targets for a project
type Switcher struct {
	opts *SwitcherOptions
}

// NewSwitcher creates a new deployer switcher
func NewSwitcher(opts *SwitcherOptions) *Switcher {
	return &Switcher{opts: opts}
}

// Switch executes the deployer switch process
func (s *Switcher) Switch(ctx context.Context, prompter *ui.Prompter) error {
	fmt.Printf("\n🔄 Switching deployer for '%s'...\n\n", s.opts.ProjectName)

	// Step 1: Detect and remove old deployment files
	if err := s.removeOldDeploymentFiles(prompter); err != nil {
		return fmt.Errorf("failed to remove old deployment files: %w", err)
	}

	// Step 2: Update forge.json configuration
	if err := s.updateForgeConfig(); err != nil {
		return fmt.Errorf("failed to update forge.json: %w", err)
	}

	// Step 3: Generate new deployment files
	if err := s.generateDeploymentFiles(); err != nil {
		return fmt.Errorf("failed to generate deployment files: %w", err)
	}

	// Step 4: Update GitHub workflows based on active deployers
	if err := s.updateGitHubWorkflows(); err != nil {
		return fmt.Errorf("failed to update GitHub workflows: %w", err)
	}

	return nil
}

// removeOldDeploymentFiles removes the old deployment folder
func (s *Switcher) removeOldDeploymentFiles(prompter *ui.Prompter) error {
	// Get current deployer and configPath
	if s.opts.Project.Architect == nil || s.opts.Project.Architect.Deploy == nil {
		fmt.Println("⚠️  No existing deployment configuration found")
		return nil
	}

	currentDeployer := s.opts.Project.Architect.Deploy.Deployer
	if currentDeployer == "" {
		fmt.Println("⚠️  No current deployer configured")
		return nil
	}

	// Extract deployer name from @forge/<name>:deploy
	currentDeployerName := extractDeployerName(currentDeployer)
	if currentDeployerName == s.opts.TargetDeployer {
		fmt.Printf("✓ Project already using '%s' deployer\n", s.opts.TargetDeployer)
		return nil
	}

	// Get configPath from current deploy options
	var oldConfigPath string
	if options, ok := s.opts.Project.Architect.Deploy.Options["configPath"].(string); ok && options != "" {
		oldConfigPath = options
	} else {
		// Fallback to default pattern
		oldConfigPath = fmt.Sprintf("deploy/%s", currentDeployerName)
	}

	// Resolve absolute path
	projectRoot := filepath.Join(s.opts.WorkspaceRoot, s.opts.Project.Root)
	oldDeployPath := filepath.Join(projectRoot, oldConfigPath)

	// Check if deployment folder exists
	if _, err := os.Stat(oldDeployPath); os.IsNotExist(err) {
		fmt.Printf("⚠️  Old deployment folder not found: %s\n", oldConfigPath)
		return nil
	}

	fmt.Printf("📁 Found old deployment folder: %s\n", oldConfigPath)

	// Prompt for confirmation unless --force is set
	if !s.opts.Force {
		confirm, err := prompter.AskConfirm(
			fmt.Sprintf("Delete old deployment folder '%s'?", oldConfigPath),
			true,
		)
		if err != nil {
			return err
		}
		if !confirm {
			return fmt.Errorf("deployment switch cancelled by user")
		}
	}

	// Remove old deployment folder
	fmt.Printf("🗑️  Removing old deployment folder: %s\n", oldConfigPath)
	if err := os.RemoveAll(oldDeployPath); err != nil {
		return fmt.Errorf("failed to remove old deployment folder: %w", err)
	}

	fmt.Printf("✓ Removed old deployment configuration\n\n")
	return nil
}

// updateForgeConfig updates forge.json with the new deployer configuration
func (s *Switcher) updateForgeConfig() error {
	fmt.Println("📝 Updating forge.json...")

	project := s.opts.Project

	// Ensure architect structure exists
	if project.Architect == nil {
		project.Architect = &workspace.Architect{}
	}
	if project.Architect.Deploy == nil {
		project.Architect.Deploy = &workspace.ArchitectTarget{}
	}

	// Update deployer
	project.Architect.Deploy.Deployer = fmt.Sprintf("@forge/%s:deploy", s.opts.TargetDeployer)

	// Update options
	options := make(map[string]interface{})
	for k, v := range s.opts.DeployerConfig {
		options[k] = v
	}
	project.Architect.Deploy.Options = options

	// Update configurations (environment-specific overrides)
	configurations := s.getDefaultConfigurations()
	project.Architect.Deploy.Configurations = configurations
	project.Architect.Deploy.DefaultConfiguration = "production"

	// Update metadata
	if project.Metadata == nil {
		project.Metadata = make(map[string]interface{})
	}
	if project.Metadata["deployment"] == nil {
		project.Metadata["deployment"] = make(map[string]interface{})
	}
	deploymentMeta := project.Metadata["deployment"].(map[string]interface{})
	deploymentMeta["target"] = s.opts.TargetDeployer
	project.Metadata["deployment"] = deploymentMeta

	// Update project in config
	s.opts.Config.Projects[s.opts.ProjectName] = *project

	// Save forge.json
	if err := s.opts.Config.SaveToDir(s.opts.WorkspaceRoot); err != nil {
		return err
	}

	fmt.Println("✓ Updated forge.json")
	return nil
}

// getDefaultConfigurations returns default environment configurations for the deployer
func (s *Switcher) getDefaultConfigurations() map[string]interface{} {
	configs := map[string]interface{}{
		"production":  make(map[string]interface{}),
		"development": make(map[string]interface{}),
		"local":       make(map[string]interface{}),
	}

	switch s.opts.TargetDeployer {
	case "helm":
		configs["production"] = map[string]interface{}{
			"namespace": "prod",
		}
		configs["development"] = map[string]interface{}{
			"namespace": "dev",
		}
		configs["local"] = map[string]interface{}{
			"namespace": "default",
		}

	case "firebase":
		if projectId, ok := s.opts.DeployerConfig["projectId"]; ok {
			configs["production"] = map[string]interface{}{
				"projectId": projectId,
			}
			configs["development"] = map[string]interface{}{
				"projectId": fmt.Sprintf("%s-dev", projectId),
			}
			configs["local"] = map[string]interface{}{
				"projectId": fmt.Sprintf("%s-dev", projectId),
			}
		}

	case "cloudrun":
		if region, ok := s.opts.DeployerConfig["region"]; ok {
			configs["production"] = map[string]interface{}{
				"region": region,
			}
			configs["development"] = map[string]interface{}{
				"region": region,
			}
			configs["local"] = map[string]interface{}{
				"region": region,
			}
		}
	}

	return configs
}

// generateDeploymentFiles generates new deployment configuration files
func (s *Switcher) generateDeploymentFiles() error {
	fmt.Printf("\n📦 Generating %s deployment files...\n", s.opts.TargetDeployer)

	projectRoot := filepath.Join(s.opts.WorkspaceRoot, s.opts.Project.Root)
	configPath := s.opts.DeployerConfig["configPath"]
	deployPath := filepath.Join(projectRoot, configPath)

	// Create deployment directory
	if err := os.MkdirAll(deployPath, 0755); err != nil {
		return fmt.Errorf("failed to create deployment directory: %w", err)
	}

	// Generate deployer-specific files
	switch s.opts.TargetDeployer {
	case "helm":
		return s.generateHelmFiles(projectRoot, deployPath)
	case "firebase":
		return s.generateFirebaseFiles(projectRoot, deployPath)
	case "cloudrun":
		return s.generateCloudRunFiles(projectRoot, deployPath)
	default:
		return fmt.Errorf("unsupported deployer: %s", s.opts.TargetDeployer)
	}
}

// generateHelmFiles generates Helm deployment files
func (s *Switcher) generateHelmFiles(projectRoot, deployPath string) error {
	deployGen := generator.NewDeploymentFileGenerator(s.opts.Project, s.opts.ProjectName, s.opts.Config)

	// Generate Helm values files
	if err := deployGen.GenerateHelmValues(deployPath, s.opts.DeployerConfig); err != nil {
		return err
	}

	fmt.Println("  ✓ values.yaml")
	fmt.Println("  ✓ values-dev.yaml")
	fmt.Println("  ✓ values-prod.yaml")
	fmt.Println("  ✓ README.md")
	fmt.Println("✓ Helm deployment files generated")

	return nil
}

// generateFirebaseFiles generates Firebase deployment files
func (s *Switcher) generateFirebaseFiles(projectRoot, deployPath string) error {
	deployGen := generator.NewDeploymentFileGenerator(s.opts.Project, s.opts.ProjectName, s.opts.Config)

	// Generate Firebase configuration files
	if err := deployGen.GenerateFirebaseConfig(deployPath, s.opts.DeployerConfig); err != nil {
		return err
	}

	fmt.Println("  ✓ firebase.json")
	fmt.Println("  ✓ .firebaserc")
	fmt.Println("  ✓ README.md")
	fmt.Println("✓ Firebase deployment files generated")

	return nil
}

// generateCloudRunFiles generates Cloud Run deployment files
func (s *Switcher) generateCloudRunFiles(projectRoot, deployPath string) error {
	deployGen := generator.NewDeploymentFileGenerator(s.opts.Project, s.opts.ProjectName, s.opts.Config)

	// Generate Cloud Run service definition
	if err := deployGen.GenerateCloudRunConfig(deployPath, s.opts.DeployerConfig); err != nil {
		return err
	}

	fmt.Println("  ✓ service.yaml")
	fmt.Println("  ✓ README.md")
	fmt.Println("✓ Cloud Run deployment files generated")

	return nil
}

// updateGitHubWorkflows updates GitHub Actions workflows based on active deployers
func (s *Switcher) updateGitHubWorkflows() error {
	fmt.Println("\n🔧 Updating GitHub Actions workflows...")

	workflowGen := generator.NewWorkflowGenerator(s.opts.Config, s.opts.WorkspaceRoot)
	if err := workflowGen.UpdateWorkflows(); err != nil {
		return err
	}

	fmt.Println("✓ GitHub workflows updated")
	return nil
}

// extractDeployerName extracts the deployer name from a deployer string like "@forge/helm:deploy"
func extractDeployerName(deployer string) string {
	// Parse @forge/<name>:deploy
	if len(deployer) > 7 && deployer[:7] == "@forge/" {
		end := len(deployer)
		if colonIdx := findLastColon(deployer); colonIdx != -1 {
			end = colonIdx
		}
		return deployer[7:end]
	}
	return deployer
}

// findLastColon finds the index of the last colon in a string
func findLastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
