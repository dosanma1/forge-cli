package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/builder"
	"github.com/dosanma1/forge-cli/internal/deployer"
	"github.com/dosanma1/forge-cli/internal/skaffold"
	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	deployEnv       string
	deployVerbose   bool
	deployTail      bool
	deploySkipBuild bool
	deployPlatform  string
)

var deployCmd = &cobra.Command{
	Use:   "deploy [service...]",
	Short: "Deploy services using Skaffold",
	Long: `Deploy one or more services to the specified environment using Skaffold.

The deploy command uses Skaffold's API to orchestrate deployments based on
your forge.json configuration. Each configuration (local, development, production)
becomes a Skaffold profile with environment-specific settings.

Examples:
  forge deploy                           # Deploy all services using default config
  forge deploy --env=production          # Deploy all to production
  forge deploy api-server --env=local    # Deploy specific service locally
  forge deploy --skip-build              # Deploy without rebuilding images
  forge deploy --tail                    # Stream logs after deployment`,
	RunE: runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVarP(&deployEnv, "env", "e", "", "Environment/profile to deploy (local, development, production)")
	deployCmd.Flags().BoolVarP(&deployVerbose, "verbose", "v", false, "Show verbose output")
	deployCmd.Flags().BoolVarP(&deployTail, "tail", "t", false, "Stream logs after deployment")
	deployCmd.Flags().BoolVar(&deploySkipBuild, "skip-build", false, "Skip build phase")
	deployCmd.Flags().StringVar(&deployPlatform, "platform", "", "Target platform for builds (empty = native platform)")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Using Skaffold-first deployment architecture")
	ctx := context.Background()

	// Get workspace root
	workspaceRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load forge.json (with validation)
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load forge.json: %w", err)
	}

	// Determine which projects to deploy
	projectNames := args
	if len(projectNames) == 0 {
		// Deploy all projects
		for name := range config.Projects {
			projectNames = append(projectNames, name)
		}
	}

	// Validate that all specified projects exist
	for _, projectName := range projectNames {
		if _, exists := config.Projects[projectName]; !exists {
			return fmt.Errorf("project %q not found in forge.json", projectName)
		}
	}

	// Determine configuration/environment
	deployConfig := deployEnv
	if deployConfig == "" {
		deployConfig = "production"
		if deployVerbose {
			fmt.Printf("ℹ️  Using default configuration: %s\n", deployConfig)
		}
	}

	// Partition projects into Skaffold-compatible vs direct deployment
	skaffoldProjects := []string{}
	directProjects := []string{}

	for _, projectName := range projectNames {
		project := config.Projects[projectName]

		if project.Architect == nil || project.Architect.Deploy == nil || project.Architect.Build == nil {
			return fmt.Errorf("project %s has incomplete architect configuration", projectName)
		}

		deployerName := project.Architect.Deploy.Deployer
		builderName := project.Architect.Build.Builder

		// Check if this combination can use Skaffold
		if deployer.CanUseSkaffold(deployerName, builderName) {
			skaffoldProjects = append(skaffoldProjects, projectName)
		} else {
			directProjects = append(directProjects, projectName)
		}
	}

	// Deploy Skaffold-compatible projects first (batch orchestration)
	if len(skaffoldProjects) > 0 {
		if deployVerbose {
			fmt.Printf("🔧 Deploying with Skaffold orchestration: %s\n", strings.Join(skaffoldProjects, ", "))
		}

		// Generate Skaffold configuration
		skaffoldConfig, err := skaffold.GenerateConfig(config, skaffoldProjects, workspaceRoot, deployPlatform)
		if err != nil {
			return fmt.Errorf("failed to generate Skaffold config: %w", err)
		}

		// Create Skaffold executor
		executor := skaffold.NewExecutor(skaffoldConfig, workspaceRoot)

		// Deploy using Skaffold (builds + deploys)
		deployOpts := skaffold.DeployOptions{
			Profile:   deployConfig,
			SkipBuild: deploySkipBuild,
			Verbose:   deployVerbose,
			Tail:      deployTail,
		}

		if err := executor.Deploy(ctx, deployOpts); err != nil {
			return fmt.Errorf("❌ Skaffold deploy failed: %w", err)
		}
	}

	// Deploy direct projects sequentially (build then deploy each)
	if len(directProjects) > 0 {
		if deployVerbose {
			fmt.Printf("🔧 Deploying with direct deployers: %s\n", strings.Join(directProjects, ", "))
		}

		for _, projectName := range directProjects {
			project := config.Projects[projectName]

			if deployVerbose {
				fmt.Printf("\n📦 Deploying %s (configuration: %s)\n", projectName, deployConfig)
			}

			// Step 1: Build the project (unless skip-build is set)
			var artifact *builder.BuildArtifact
			if !deploySkipBuild {
				// Get builder
				builderName := project.Architect.Build.Builder
				projectBuilder, err := builder.GetBuilder(builderName)
				if err != nil {
					return fmt.Errorf("failed to get builder for %s: %w", projectName, err)
				}

				// Get project absolute path
				projectAbsPath := filepath.Join(workspaceRoot, project.Root)

				// Get build options and configuration options
				buildOpts := project.Architect.Build.Options
				var configOpts map[string]interface{}
				if project.Architect.Build.Configurations != nil {
					if cfg, ok := project.Architect.Build.Configurations[deployConfig]; ok {
						if typedCfg, ok := cfg.(map[string]interface{}); ok {
							configOpts = typedCfg
						}
					}
				}

				// Build the project
				opts := &builder.BuildOptions{
					ProjectRoot:          projectAbsPath,
					Configuration:        deployConfig,
					Options:              buildOpts,
					ConfigurationOptions: configOpts,
					Verbose:              deployVerbose,
					Platform:             deployPlatform,
					WorkspaceRoot:        workspaceRoot,
				}

				if deployVerbose {
					fmt.Printf("🔨 Building %s with %s\n", projectName, builderName)
				}

				artifact, err = projectBuilder.Build(ctx, opts)
				if err != nil {
					return fmt.Errorf("❌ Build failed for %s: %w", projectName, err)
				}

				if deployVerbose {
					fmt.Printf("✅ Built %s: %s\n", projectName, artifact.Type)
				}
			}

			// Step 2: Deploy using the deployer
			deployerName := project.Architect.Deploy.Deployer
			projectDeployer, err := deployer.GetDeployer(deployerName)
			if err != nil {
				return fmt.Errorf("failed to get deployer for %s: %w", projectName, err)
			}

			// Get deployment options
			deployOpts := project.Architect.Deploy.Options
			if deployOpts == nil {
				deployOpts = make(map[string]interface{})
			}

			// Merge configuration-specific options
			if project.Architect.Deploy.Configurations != nil {
				if cfg, ok := project.Architect.Deploy.Configurations[deployConfig].(map[string]interface{}); ok {
					for k, v := range cfg {
						deployOpts[k] = v
					}
				}
			}

			// Deploy
			if deployVerbose {
				fmt.Printf("🚀 Deploying %s with %s\n", projectName, deployerName)
			}

			deployOptions := &deployer.DeployOptions{
				Project:       projectName,
				Artifact:      artifact,
				Builder:       project.Architect.Build.Builder,
				Configuration: deployConfig,
				Options:       deployOpts,
				Verbose:       deployVerbose,
				WorkspaceRoot: workspaceRoot,
				ProjectRoot:   filepath.Join(workspaceRoot, project.Root),
			}

			if err := projectDeployer.Deploy(ctx, deployOptions); err != nil {
				return fmt.Errorf("❌ Deploy failed for %s: %w", projectName, err)
			}

			if deployVerbose {
				fmt.Printf("✅ Deployed %s successfully\n", projectName)
			}
		}
	}

	fmt.Printf("\n✅ All deployments completed successfully!\n")
	return nil
}
