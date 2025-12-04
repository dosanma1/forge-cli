package cmd

import (
	"context"
	"fmt"

	"github.com/dosanma1/forge-cli/internal/generator"
	"github.com/spf13/cobra"
)

var (
	newGitHubOrg      string
	newDockerRegistry string
	newGCPProjectID   string
	newK8sNamespace   string
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
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Create generator
	gen := generator.NewWorkspaceGenerator()

	// Prepare options
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      name,
		Data: map[string]interface{}{
			"github_org":      newGitHubOrg,
			"docker_registry": newDockerRegistry,
			"gcp_project_id":  newGCPProjectID,
			"k8s_namespace":   newK8sNamespace,
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
