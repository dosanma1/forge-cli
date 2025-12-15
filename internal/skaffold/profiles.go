package skaffold

import (
	"fmt"

	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/dosanma1/forge-cli/internal/workspace"
)

// GenerateProfiles creates Skaffold profiles from forge.json configurations.
// Each configuration key becomes a profile with that exact name.
// Profiles inherit from base config and override registry, namespaces, and build args.
func GenerateProfiles(config *workspace.Config, projectNames []string, platform string) ([]latest.Profile, error) {
	profiles := []latest.Profile{}

	// Collect all unique configuration keys across all selected projects
	configKeys := collectConfigurationKeys(config, projectNames)

	// Create a profile for each configuration
	for _, configKey := range configKeys {
		profile := createProfile(config, projectNames, configKey, platform)
		// Skip profiles with no artifacts (e.g., when all projects use @forge/angular:build)
		if len(profile.Pipeline.Build.Artifacts) > 0 {
			profiles = append(profiles, profile)
		}
	}

	return profiles, nil
}

// collectConfigurationKeys collects all unique configuration keys from the selected projects.
// Since we validate that build and deploy configs match, we only need to check build configs.
func collectConfigurationKeys(config *workspace.Config, projectNames []string) []string {
	keysMap := make(map[string]bool)

	for _, projectName := range projectNames {
		project, exists := config.Projects[projectName]
		if !exists {
			continue
		}

		if project.Architect == nil || project.Architect.Build == nil {
			continue
		}

		if project.Architect.Build.Configurations != nil {
			for key := range project.Architect.Build.Configurations {
				keysMap[key] = true
			}
		}
	}

	// Convert map to slice
	keys := make([]string, 0, len(keysMap))
	for key := range keysMap {
		keys = append(keys, key)
	}

	return keys
}

// createProfile creates a single Skaffold profile for a configuration key.
func createProfile(config *workspace.Config, projectNames []string, configKey string, platform string) latest.Profile {
	profile := latest.Profile{
		Name: configKey,
		Pipeline: latest.Pipeline{
			Build: latest.BuildConfig{
				Artifacts: []*latest.Artifact{},
			},
			Deploy: latest.DeployConfig{},
		},
	}

	// Get platform-specific Bazel args
	bazelArgs := GetBazelPlatformArgs(platform)

	// Add configuration-specific build args
	bazelArgs = append(bazelArgs, fmt.Sprintf("--define=ENV=%s", configKey))

	// Create artifacts with configuration-specific settings
	for _, projectName := range projectNames {
		project, exists := config.Projects[projectName]
		if !exists {
			continue
		}

		if project.Architect == nil || project.Architect.Build == nil {
			continue
		}

		// Get base options
		baseOptions := project.Architect.Build.Options
		if baseOptions == nil {
			baseOptions = make(map[string]interface{})
		}

		// Get configuration-specific options
		configOptions := make(map[string]interface{})
		if project.Architect.Build.Configurations != nil {
			if cfg, ok := project.Architect.Build.Configurations[configKey].(map[string]interface{}); ok {
				configOptions = cfg
			}
		}

		// Merge options: config-specific overrides base
		mergedOptions := mergeOptions(baseOptions, configOptions)

		// Get registry (config-specific or base)
		registry := GetRegistryFromOptions(mergedOptions, "gcr.io/default-project")

		// Note: We don't add --config=race or --config=debug here
		// Environment-specific profiles (local, development, production) should
		// define their own Bazel configs in .bazelrc if needed

		// Create artifact only for Bazel builds
		// Skip @forge/angular:build as it produces static files, not container images
		builder := project.Architect.Build.Builder
		if builder == "@forge/bazel:build" {
			artifact := CreateBazelArtifact(projectName, project, registry, bazelArgs)
			profile.Pipeline.Build.Artifacts = append(profile.Pipeline.Build.Artifacts, artifact)
		}

		// Add deploy configuration overrides
		if project.Architect.Deploy != nil {
			deployOptions := project.Architect.Deploy.Options
			if deployOptions == nil {
				deployOptions = make(map[string]interface{})
			}

			// Get configuration-specific deploy options
			deployConfigOptions := make(map[string]interface{})
			if project.Architect.Deploy.Configurations != nil {
				if cfg, ok := project.Architect.Deploy.Configurations[configKey].(map[string]interface{}); ok {
					deployConfigOptions = cfg
				}
			}

			// Merge deploy options
			mergedDeployOptions := mergeOptions(deployOptions, deployConfigOptions)

			// Apply namespace override for Helm
			if project.Architect.Deploy.Deployer == "@forge/helm:deploy" {
				if profile.Pipeline.Deploy.LegacyHelmDeploy == nil {
					profile.Pipeline.Deploy.LegacyHelmDeploy = &latest.LegacyHelmDeploy{
						Releases: []latest.HelmRelease{},
					}
				}

				// Update namespace for this project's release
				namespace := getStringOption(mergedDeployOptions, "namespace", "default")

				// Find and update the release for this project
				for i, release := range profile.Pipeline.Deploy.LegacyHelmDeploy.Releases {
					if release.Name == projectName {
						profile.Pipeline.Deploy.LegacyHelmDeploy.Releases[i].Namespace = namespace
						break
					}
				}
			}
		}
	}

	return profile
}

// mergeOptions merges two option maps, with override values taking precedence.
func mergeOptions(base, override map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// Copy base options
	for k, v := range base {
		merged[k] = v
	}

	// Override with config-specific options
	for k, v := range override {
		merged[k] = v
	}

	return merged
}
