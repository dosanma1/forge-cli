package cmd

import (
	"context"
	"fmt"
	"os"

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

	fmt.Println(ui.TitleStyle.Render("🚀 Forge Workspace Setup"))
	fmt.Println()

	// Collect services
	type serviceSpec struct {
		Name string
		Type string
	}
	var services []serviceSpec

	for {
		fmt.Println(ui.SubtitleStyle.Render("🔧 Backend Service"))
		createService, err := ui.AskConfirm("Create a backend service?", true)
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}

		if !createService {
			break
		}

		_, serviceType, err := ui.AskSelect("Select service language:", []string{"Go", "NestJS"})
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}

		serviceName, err := ui.AskText("Service name (e.g., api-server):", "api-server")
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}

		services = append(services, serviceSpec{
			Name: serviceName,
			Type: serviceType,
		})

		if len(services) == 1 {
			addMore, err := ui.AskConfirm("Add another service?", false)
			if err != nil {
				return fmt.Errorf("cancelled: %w", err)
			}
			if !addMore {
				break
			}
		}
	}

	// Collect frontends
	type frontendSpec struct {
		Name       string
		Type       string
		Deployment string
	}
	var frontends []frontendSpec

	for {
		fmt.Println(ui.SubtitleStyle.Render("🎨 Frontend Application"))
		createFrontend, err := ui.AskConfirm("Create a frontend application?", false)
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}

		if !createFrontend {
			break
		}

		_, frontendType, err := ui.AskSelect("Select frontend framework:", []string{"Angular", "Next.js"})
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}

		frontendName, err := ui.AskText("Application name (e.g., web-app):", "web-app")
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}

		_, deployment, err := ui.AskSelect("Deployment target:", []string{"Firebase", "CloudRun", "GKE"})
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}

		frontends = append(frontends, frontendSpec{
			Name:       frontendName,
			Type:       frontendType,
			Deployment: deployment,
		})

		if len(frontends) == 1 {
			addMore, err := ui.AskConfirm("Add another frontend?", false)
			if err != nil {
				return fmt.Errorf("cancelled: %w", err)
			}
			if !addMore {
				break
			}
		}
	}

	// Show summary
	fmt.Println()
	fmt.Println(ui.TitleStyle.Render("📦 Workspace Summary"))
	fmt.Println()
	fmt.Printf("Workspace: %s\n", ui.SuccessStyle.Render(name))

	if len(services) > 0 {
		fmt.Println("\n" + ui.SubtitleStyle.Render("Services:"))
		for _, svc := range services {
			fmt.Printf("  • %s (%s)\n", svc.Name, svc.Type)
		}
	}

	if len(frontends) > 0 {
		fmt.Println("\n" + ui.SubtitleStyle.Render("Frontends:"))
		for _, app := range frontends {
			fmt.Printf("  • %s (%s → %s)\n", app.Name, app.Type, app.Deployment)
		}
	}

	fmt.Println()
	proceed, err := ui.AskConfirm("Proceed with generation?", true)
	if err != nil || !proceed {
		fmt.Println(ui.ErrorStyle.Render("✗ Cancelled"))
		os.Exit(0)
	}

	// Create generator
	gen := generator.NewWorkspaceGenerator()

	// Convert services and frontends to []interface{} for generator
	var servicesData []interface{}
	for _, svc := range services {
		servicesData = append(servicesData, map[string]interface{}{
			"Name": svc.Name,
			"Type": svc.Type,
		})
	}

	var frontendsData []interface{}
	for _, app := range frontends {
		frontendsData = append(frontendsData, map[string]interface{}{
			"Name":       app.Name,
			"Type":       app.Type,
			"Deployment": app.Deployment,
		})
	}

	// Prepare base options
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      name,
		Data: map[string]interface{}{
			"github_org":      newGitHubOrg,
			"docker_registry": newDockerRegistry,
			"gcp_project_id":  newGCPProjectID,
			"k8s_namespace":   newK8sNamespace,
			"gke_region":      newGKERegion,
			"gke_cluster":     newGKECluster,
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

	fmt.Println()
	fmt.Println(ui.SuccessStyle.Render("✓ Workspace created successfully!"))
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", name)
	fmt.Printf("  forge build\n")

	return nil
}
