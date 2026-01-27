package cmd

import (
	"context"
	"fmt"

	"github.com/dosanma1/forge-cli/internal/deployer"
	"github.com/dosanma1/forge-cli/internal/ui"
	"github.com/dosanma1/forge-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var (
	switchForce      bool
	switchConfigPath string
	switchConfig     map[string]string
)

var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch project configurations",
	Long: `Switch project configurations like deployment targets.

Examples:
  forge switch deployer web-app helm
  forge switch deployer dashboard firebase --config projectId=my-project
  forge switch deployer web-app helm --config-path=infra/helm --force`,
}

var switchDeployerCmd = &cobra.Command{
	Use:   "deployer [project] [deployer]",
	Short: "Switch deployment target for a project",
	Long: `Switch the deployment target for a project.

Available deployers:
  - helm: Deploy to Kubernetes using Helm charts
  - firebase: Deploy to Firebase Hosting (Angular only)
  - cloudrun: Deploy to Google Cloud Run

The command will:
1. Prompt for deployer-specific configuration (unless --config is provided)
2. Update forge.json with the new deployer configuration
3. Remove old deployment files from the previous deployer
4. Generate new deployment files for the target deployer
5. Update GitHub Actions workflows based on active deployers`,
	Example: `  # Interactive mode
  forge switch deployer web-app helm

  # Non-interactive mode with configuration
  forge switch deployer web-app helm --config namespace=prod,port=8080 --force

  # Custom deployment path
  forge switch deployer dashboard firebase --config-path deploy/hosting

  # CloudRun deployment
  forge switch deployer api-service cloudrun --config region=us-central1`,
	Args: cobra.ExactArgs(2),
	RunE: runSwitchDeployer,
}

func init() {
	rootCmd.AddCommand(switchCmd)
	switchCmd.AddCommand(switchDeployerCmd)

	switchDeployerCmd.Flags().BoolVar(&switchForce, "force", false, "Skip all confirmation prompts")
	switchDeployerCmd.Flags().StringVar(&switchConfigPath, "config-path", "", "Custom deployment folder path (default: deploy/<deployer>)")
	switchDeployerCmd.Flags().StringToStringVar(&switchConfig, "config", nil, "Deployer-specific configuration (key=value pairs)")
}

func runSwitchDeployer(cmd *cobra.Command, args []string) error {
	projectName := args[0]
	deployerName := args[1]

	// Validate deployer name
	validDeployers := []string{"helm", "firebase", "cloudrun"}
	if !contains(validDeployers, deployerName) {
		return fmt.Errorf("invalid deployer '%s'. Valid options: %v", deployerName, validDeployers)
	}

	// Load forge.json
	config, err := workspace.LoadConfig(".")
	if err != nil {
		return fmt.Errorf("failed to load forge.json: %w", err)
	}

	// Validate project exists
	project := config.GetProject(projectName)
	if project == nil {
		return fmt.Errorf("project '%s' not found in forge.json", projectName)
	}

	// Validate deployer compatibility with project language
	if err := validateDeployerCompatibility(project.Language, deployerName); err != nil {
		return err
	}

	// Create prompter for interactive mode
	prompter, err := ui.NewPrompter()
	if err != nil {
		return fmt.Errorf("failed to create prompter: %w", err)
	}

	// Get configuration - either from flags or interactive prompts
	deployerConfig := switchConfig
	if len(deployerConfig) == 0 {
		deployerConfig, err = promptForDeployerConfig(prompter, deployerName, project.Language)
		if err != nil {
			return fmt.Errorf("failed to get deployer configuration: %w", err)
		}
	}

	// Determine config path
	configPath := switchConfigPath
	if configPath == "" {
		configPath = fmt.Sprintf("deploy/%s", deployerName)
	}
	deployerConfig["configPath"] = configPath

	// Create deployer switcher
	switcher := deployer.NewSwitcher(&deployer.SwitcherOptions{
		Config:         config,
		ProjectName:    projectName,
		Project:        project,
		TargetDeployer: deployerName,
		DeployerConfig: deployerConfig,
		Force:          switchForce,
		WorkspaceRoot:  ".",
	})

	// Execute switch
	ctx := context.Background()
	if err := switcher.Switch(ctx, prompter); err != nil {
		return err
	}

	fmt.Printf("\n✓ Successfully switched '%s' to use '%s' deployer\n", projectName, deployerName)
	fmt.Printf("✓ Deployment configuration: %s/%s\n", project.Root, configPath)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  $ forge build %s\n", projectName)
	fmt.Printf("  $ forge deploy %s\n", projectName)

	return nil
}

// validateDeployerCompatibility checks if the deployer is compatible with the project language
func validateDeployerCompatibility(language, deployer string) error {
	// Firebase is only for Angular projects
	if deployer == "firebase" && language != "angular" {
		return fmt.Errorf("firebase deployer is only compatible with Angular projects, found: %s", language)
	}

	// All deployers support Go and NestJS
	// Helm and CloudRun support Angular
	return nil
}

// promptForDeployerConfig prompts the user for deployer-specific configuration
func promptForDeployerConfig(prompter *ui.Prompter, deployerName, language string) (map[string]string, error) {
	config := make(map[string]string)

	switch deployerName {
	case "helm":
		// Prompt for Helm configuration
		namespace, err := prompter.AskText("Kubernetes namespace", "default")
		if err != nil {
			return nil, err
		}
		config["namespace"] = namespace

		port, err := prompter.AskText("Service port", getDefaultPort(language))
		if err != nil {
			return nil, err
		}
		config["port"] = port

		healthPath, err := prompter.AskText("Health check path", "/health")
		if err != nil {
			return nil, err
		}
		config["healthPath"] = healthPath

	case "firebase":
		// Prompt for Firebase configuration
		projectId, err := prompter.AskText("Firebase project ID", "")
		if err != nil {
			return nil, err
		}
		config["projectId"] = projectId

		site, err := prompter.AskText("Firebase hosting site (optional)", "")
		if err != nil {
			return nil, err
		}
		if site != "" {
			config["site"] = site
		}

	case "cloudrun":
		// Prompt for Cloud Run configuration
		region, err := prompter.AskText("Cloud Run region", "us-central1")
		if err != nil {
			return nil, err
		}
		config["region"] = region

		serviceName, err := prompter.AskText("Service name", "")
		if err != nil {
			return nil, err
		}
		config["service"] = serviceName

		memory, err := prompter.AskText("Memory limit", "512Mi")
		if err != nil {
			return nil, err
		}
		config["memory"] = memory

		cpu, err := prompter.AskText("CPU limit", "1")
		if err != nil {
			return nil, err
		}
		config["cpu"] = cpu
	}

	return config, nil
}

// getDefaultPort returns the default port for a given language
func getDefaultPort(language string) string {
	switch language {
	case "angular":
		return "4200"
	case "nestjs":
		return "3000"
	case "go":
		return "8080"
	default:
		return "8080"
	}
}
