package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dosanma1/forge-cli/internal/generator"
	"github.com/dosanma1/forge-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	newGitHubOrg      string
	newDockerRegistry string
	newGCPProjectID   string
	newK8sNamespace   string
	newGKERegion      string
	newGKECluster     string
)

var newCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new Forge workspace",
	Long: `Create a new Forge workspace with the specified name.

Examples:
  forge new
  forge new my-project
  forge new my-project --github-org=mycompany
  forge new my-project --docker-registry=gcr.io/mycompany
  forge new my-project --gcp-project=my-gcp-project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runNew,
}

func init() {
	rootCmd.AddCommand(newCmd)

	newCmd.Flags().StringVar(&newGitHubOrg, "github-org", "", "Organization/username (e.g., mycompany)")
	newCmd.Flags().StringVar(&newDockerRegistry, "docker-registry", "", "Docker registry (e.g., gcr.io/mycompany)")
	newCmd.Flags().StringVar(&newGCPProjectID, "gcp-project", "", "GCP project ID")
	newCmd.Flags().StringVar(&newK8sNamespace, "k8s-namespace", "", "Kubernetes namespace")
	newCmd.Flags().StringVar(&newGKERegion, "gke-region", "us-central1", "GKE cluster region")
	newCmd.Flags().StringVar(&newGKECluster, "gke-cluster", "", "GKE cluster name (defaults to <workspace>-cluster)")
}

func runNew(cmd *cobra.Command, args []string) error {
	// Create prompter
	prompter, err := ui.NewPrompter()
	if err != nil {
		return fmt.Errorf("failed to create prompter: %w", err)
	}

	// Get name from args or prompt for it
	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		name, err = prompter.AskText("What name would you like to use for the workspace?", "")
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}
	}

	// Collect initial values from flags
	githubOrg := newGitHubOrg

	// If github-org not provided, try to get it from git config
	if githubOrg == "" {
		if org, err := getOrgFromGit(); err == nil && org != "" {
			githubOrg = org
		}
	}

	// If still not set, prompt for it
	if githubOrg == "" {
		githubOrg, err = prompter.AskText("Organization/username (e.g., mycompany, myuser)", "")
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}
	}

	dockerRegistry := newDockerRegistry
	gcpProjectId := newGCPProjectID
	k8sNamespace := newK8sNamespace
	gkeRegion := newGKERegion
	gkeCluster := newGKECluster

	// Build services list
	var servicesData []interface{}

	// Ask for services in a loop
	for {
		addService, err := prompter.AskConfirm("Would you like to add a backend service?", len(servicesData) == 0)
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}

		if !addService {
			break
		}

		serviceName, err := prompter.AskText("What name would you like to use for the service?", "api-server")
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}

		serviceType, err := prompter.AskSelect("Which backend framework would you like to use?", []string{"Go", "NestJS"})
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}

		// Prompt for deployer selection
		deployerChoice, err := prompter.AskSelect("Which deployment target would you like to use?", []string{"Helm (Kubernetes)", "CloudRun"})
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}

		// Map display names to internal names
		var deployer string
		switch deployerChoice {
		case "Helm (Kubernetes)":
			deployer = "helm"
		case "CloudRun":
			deployer = "cloudrun"
		default:
			deployer = "helm"
		}

		// Prompt for deployer-specific configuration
		deployerConfig := make(map[string]string)
		switch deployer {
		case "helm":
			namespace, err := prompter.AskText("Kubernetes namespace", "default")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["namespace"] = namespace

			port, err := prompter.AskText("Service port", "8080")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["port"] = port

			healthPath, err := prompter.AskText("Health check path", "/health")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["healthPath"] = healthPath

		case "cloudrun":
			region, err := prompter.AskText("Cloud Run region", "us-central1")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["region"] = region

			memory, err := prompter.AskText("Memory limit", "512Mi")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["memory"] = memory
		}

		service := map[string]interface{}{
			"Name":           serviceName,
			"Type":           serviceType,
			"Deployer":       deployer,
			"DeployerConfig": deployerConfig,
		}
		servicesData = append(servicesData, service)
	}

	// Build frontends list
	var frontendsData []interface{}

	// Ask for apps in a loop
	for {
		addApp, err := prompter.AskConfirm("Would you like to add a frontend application?", len(frontendsData) == 0 && len(servicesData) > 0)
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}

		if !addApp {
			break
		}

		appName, err := prompter.AskText("What name would you like to use for the application?", "web-app")
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}

		appType, err := prompter.AskSelect("Which frontend framework would you like to use?", []string{"Angular", "Next.js"})
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}

		deployerChoice, err := prompter.AskSelect("Which deployment target would you like to use?", []string{"Firebase", "Helm (Kubernetes)", "CloudRun"})
		if err != nil {
			fmt.Println("Workspace creation cancelled.")
			return nil
		}

		// Map display names to internal names
		var deployer string
		switch deployerChoice {
		case "Firebase":
			deployer = "firebase"
		case "Helm (Kubernetes)":
			deployer = "helm"
		case "CloudRun":
			deployer = "cloudrun"
		default:
			deployer = "firebase"
		}

		// Prompt for deployer-specific configuration
		deployerConfig := make(map[string]string)
		switch deployer {
		case "firebase":
			projectId, err := prompter.AskText("Firebase project ID", "")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["projectId"] = projectId

			site, err := prompter.AskText("Firebase hosting site (optional)", "")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			if site != "" {
				deployerConfig["site"] = site
			}

		case "helm":
			namespace, err := prompter.AskText("Kubernetes namespace", "default")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["namespace"] = namespace

			port, err := prompter.AskText("Service port", "4200")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["port"] = port

		case "cloudrun":
			region, err := prompter.AskText("Cloud Run region", "us-central1")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["region"] = region

			memory, err := prompter.AskText("Memory limit", "512Mi")
			if err != nil {
				fmt.Println("Workspace creation cancelled.")
				return nil
			}
			deployerConfig["memory"] = memory
		}

		frontend := map[string]interface{}{
			"Name":           appName,
			"Type":           appType,
			"Deployment":     deployer,
			"DeployerConfig": deployerConfig,
		}
		frontendsData = append(frontendsData, frontend)
	}

	// Validate we have at least one service or frontend
	if len(servicesData) == 0 && len(frontendsData) == 0 {
		fmt.Println("At least one backend service or frontend application is required.")
		return nil
	}

	// Show summary
	fmt.Println("\nWorkspace Configuration:")
	fmt.Printf("  Name: %s\n", name)

	if len(servicesData) > 0 {
		fmt.Println("  Backend Services:")
		for _, svc := range servicesData {
			svcMap := svc.(map[string]interface{})
			fmt.Printf("    - %s (%s, %s)\n", svcMap["Name"], svcMap["Type"], svcMap["Deployer"])
		}
	}

	if len(frontendsData) > 0 {
		fmt.Println("  Frontend Applications:")
		for _, app := range frontendsData {
			appMap := app.(map[string]interface{})
			fmt.Printf("    - %s (%s, %s)\n", appMap["Name"], appMap["Type"], appMap["Deployment"])
		}
	}

	fmt.Println()
	proceed, err := prompter.AskConfirm("Would you like to proceed?", true)
	if err != nil || !proceed {
		fmt.Println("Workspace creation cancelled.")
		return nil
	}

	// Create generator
	fmt.Println("CREATE Creating workspace...")
	gen := generator.NewWorkspaceGenerator()

	// Prepare base options
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      name,
		Data: map[string]interface{}{
			"github_org":      githubOrg,
			"docker_registry": dockerRegistry,
			"gcp_project_id":  gcpProjectId,
			"k8s_namespace":   k8sNamespace,
			"gke_region":      gkeRegion,
			"gke_cluster":     gkeCluster,
			"services":        servicesData,
			"frontends":       frontendsData,
		},
		DryRun: false,
	}

	// Generate workspace
	ctx := context.Background()
	if err := gen.Generate(ctx, opts); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	fmt.Printf("CREATE %s\n", name)
	fmt.Println("✔ Workspace created successfully.")
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  $ cd %s\n", name)
	fmt.Printf("  $ forge build\n")

	return nil
}

// getOrgFromGit tries to get the organization/username from git config.
// This could be for GitHub, GitLab, Bitbucket, or any git hosting provider.
func getOrgFromGit() (string, error) {
	// Try github.user first (common convention)
	cmd := exec.Command("git", "config", "--get", "github.user")
	if output, err := cmd.Output(); err == nil {
		org := strings.TrimSpace(string(output))
		if org != "" && !strings.Contains(org, " ") {
			return org, nil
		}
	}

	// Try gitlab.user
	cmd = exec.Command("git", "config", "--get", "gitlab.user")
	if output, err := cmd.Output(); err == nil {
		org := strings.TrimSpace(string(output))
		if org != "" && !strings.Contains(org, " ") {
			return org, nil
		}
	}

	// Fallback to user.name, but only if it's username-like (no spaces)
	cmd = exec.Command("git", "config", "--get", "user.name")
	if output, err := cmd.Output(); err == nil {
		username := strings.TrimSpace(string(output))
		// Only return if it doesn't contain spaces (likely a username, not a full name)
		if username != "" && !strings.Contains(username, " ") {
			return username, nil
		}
	}

	return "", fmt.Errorf("no git config found")
}
