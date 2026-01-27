package skaffold

import (
	"fmt"

	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// GenerateConfig creates a unified Skaffold configuration from forge.json.
// It generates a base config with artifacts and deployers for the specified projects,
// then creates profiles for each configuration key.
func GenerateConfig(config *workspace.Config, projectNames []string, workspaceRoot string, platform string) (*latest.SkaffoldConfig, error) {
	// Validate inputs
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if len(projectNames) == 0 {
		return nil, fmt.Errorf("at least one project name is required")
	}

	// Start with default config
	skaffoldConfig := GetDefaultConfig()

	// Set metadata name from workspace
	skaffoldConfig.Metadata.Name = config.Workspace.Name

	// Get default registry from workspace config
	defaultRegistry := "gcr.io/default-project"
	if config.Workspace.Docker != nil && config.Workspace.Docker.Registry != "" {
		defaultRegistry = config.Workspace.Docker.Registry
	}

	// Create base artifacts for all selected projects
	// Use default platform (linux/amd64) for base config
	baseArtifacts := CreateBazelArtifacts(config.Projects, projectNames, defaultRegistry, "linux/amd64")
	skaffoldConfig.Pipeline.Build.Artifacts = baseArtifacts

	// Create multi-deployer configuration
	deployConfig := CreateMultiDeployer(config.Projects, projectNames, workspaceRoot)
	skaffoldConfig.Pipeline.Deploy = latest.DeployConfig{
		DeployType: *deployConfig,
	}

	// Generate profiles from configurations
	profiles, err := GenerateProfiles(config, projectNames, workspaceRoot, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to generate profiles: %w", err)
	}
	skaffoldConfig.Profiles = profiles

	return skaffoldConfig, nil
}

// GetDefaultProfile returns the default profile name from the config.
// Uses the first project's defaultConfiguration.
func GetDefaultProfile(config *workspace.Config, projectNames []string) string {
	// Use first project's default
	if len(projectNames) > 0 {
		firstProject := config.Projects[projectNames[0]]
		if firstProject.Architect != nil && firstProject.Architect.Build != nil {
			if firstProject.Architect.Build.DefaultConfiguration != "" {
				return firstProject.Architect.Build.DefaultConfiguration
			}
		}
	}

	// Ultimate fallback
	return "production"
}

// ValidateProfile checks if a profile exists in the configuration.
func ValidateProfile(config *workspace.Config, projectNames []string, profileName string) error {
	// Collect all valid configuration keys
	validKeys := collectConfigurationKeys(config, projectNames)

	// Check if profile exists
	for _, key := range validKeys {
		if key == profileName {
			return nil
		}
	}

	return fmt.Errorf("profile %q does not exist. Available profiles: %v", profileName, validKeys)
}
