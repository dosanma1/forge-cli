package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dosanma1/forge-cli/internal/generator"
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
	Use:   "new <name>",
	Short: "Create a new Forge workspace",
	Long: `Create a new Forge workspace with the specified name.

Examples:
  forge new my-project
  forge new my-project --github-org=mycompany
  forge new my-project --docker-registry=gcr.io/mycompany
  forge new my-project --gcp-project=my-gcp-project`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	rootCmd.AddCommand(newCmd)

	newCmd.Flags().StringVar(&newGitHubOrg, "github-org", "", "GitHub organization (e.g., mycompany)")
	newCmd.Flags().StringVar(&newDockerRegistry, "docker-registry", "", "Docker registry (e.g., gcr.io/mycompany)")
	newCmd.Flags().StringVar(&newGCPProjectID, "gcp-project", "", "GCP project ID")
	newCmd.Flags().StringVar(&newK8sNamespace, "k8s-namespace", "", "Kubernetes namespace")
	newCmd.Flags().StringVar(&newGKERegion, "gke-region", "us-central1", "GKE cluster region")
	newCmd.Flags().StringVar(&newGKECluster, "gke-cluster", "", "GKE cluster name (defaults to <workspace>-cluster)")
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]

	reader := bufio.NewReader(os.Stdin)

	// Ask if user wants to create a backend service
	fmt.Println("🔧 Backend Setup")
	fmt.Print("Do you want to create a Go backend service? (Y/n): ")

	backendResponse, _ := reader.ReadString('\n')
	backendResponse = strings.TrimSpace(strings.ToLower(backendResponse))

	createBackend := backendResponse == "" || backendResponse == "y" || backendResponse == "yes"
	var backendServiceName string

	if createBackend {
		fmt.Print("Enter service name (e.g., api-server): ")
		serviceNameInput, _ := reader.ReadString('\n')
		backendServiceName = strings.TrimSpace(serviceNameInput)

		if backendServiceName == "" {
			backendServiceName = "api-server" // default name
			fmt.Printf("Using default name: %s\n", backendServiceName)
		}
	}

	// Ask if user wants to create a frontend app
	fmt.Println("\n🎨 Frontend Setup")
	fmt.Print("Do you want to create a frontend application? (y/N): ")

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	createFrontend := response == "y" || response == "yes"
	var frontendAppName string

	if createFrontend {
		fmt.Print("Enter frontend application name (e.g., web-app): ")
		appNameInput, _ := reader.ReadString('\n')
		frontendAppName = strings.TrimSpace(appNameInput)

		if frontendAppName == "" {
			frontendAppName = "web-app" // default name
			fmt.Printf("Using default name: %s\n", frontendAppName)
		}
	}

	// Create generator
	gen := generator.NewWorkspaceGenerator()

	// Prepare options
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      name,
		Data: map[string]interface{}{
			"github_org":           newGitHubOrg,
			"docker_registry":      newDockerRegistry,
			"gcp_project_id":       newGCPProjectID,
			"k8s_namespace":        newK8sNamespace,
			"gke_region":           newGKERegion,
			"gke_cluster":          newGKECluster,
			"create_backend":       createBackend,
			"backend_service_name": backendServiceName,
			"create_frontend":      createFrontend,
			"frontend_app_name":    frontendAppName,
		},
		DryRun: false,
	}

	// Generate workspace
	ctx := context.Background()
	if err := gen.Generate(ctx, opts); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	return nil
}
