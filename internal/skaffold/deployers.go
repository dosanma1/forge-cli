package skaffold

import (
	"fmt"
	"path/filepath"

	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// CreateMultiDeployer creates a Skaffold deploy configuration supporting multiple deployers.
// Supports Helm, Firebase (Cloud Run), and mixed deployments in a single config.
func CreateMultiDeployer(projects map[string]workspace.Project, projectNames []string, workspaceRoot string) *latest.DeployType {
	deploy := &latest.DeployType{}

	helmReleases := []latest.HelmRelease{}
	hasCloudRun := false

	for _, projectName := range projectNames {
		project, exists := projects[projectName]
		if !exists {
			continue
		}

		if project.Architect == nil || project.Architect.Deploy == nil {
			continue
		}

		deployTarget := project.Architect.Deploy
		deployer := deployTarget.Deployer

		switch deployer {
		case "@forge/helm:deploy":
			// Check if project has multiple instances
			if instances, ok := deployTarget.Options["instances"].([]interface{}); ok && len(instances) > 0 {
				// Deploy multiple instances of the same service
				for _, inst := range instances {
					instanceName := fmt.Sprintf("%v", inst)
					release := createHelmReleaseForInstance(projectName, instanceName, project, deployTarget, workspaceRoot)
					helmReleases = append(helmReleases, release)
				}
			} else {
				// Single instance deployment
				release := createHelmRelease(projectName, project, deployTarget, workspaceRoot)
				helmReleases = append(helmReleases, release)
			}

		case "@forge/cloudrun:deploy":
			hasCloudRun = true
			// CloudRun deployments will be handled at profile level
			// as they require different setups per environment

		case "@forge/firebase:deploy":
			// Firebase deployments are handled outside Skaffold via Firebase CLI
			// in cmd/deploy.go, so we skip them here
		}
	}

	// Set up Helm deployer if we have Helm releases
	if len(helmReleases) > 0 {
		deploy.LegacyHelmDeploy = &latest.LegacyHelmDeploy{
			Releases: helmReleases,
		}
	}

	// Note: CloudRun/Firebase setup is minimal here, profiles will override
	if hasCloudRun {
		deploy.CloudRunDeploy = &latest.CloudRunDeploy{
			ProjectID: "", // Will be set by profile
			Region:    "", // Will be set by profile
		}
	}

	return deploy
}

// createHelmRelease creates a Helm release configuration for a project.
func createHelmRelease(projectName string, project workspace.Project, deployTarget *workspace.ArchitectTarget, workspaceRoot string) latest.HelmRelease {
	options := deployTarget.Options
	if options == nil {
		options = make(map[string]interface{})
	}

	// Get namespace from options
	namespace := "default"
	if ns, ok := options["namespace"].(string); ok && ns != "" {
		namespace = ns
	}

	// Determine chart based on language
	var remoteChart string
	switch project.Language {
	case "go":
		remoteChart = "https://github.com/dosanma1/forge/raw/main/templates/infra/helm/go-service"
	case "nestjs":
		remoteChart = "https://github.com/dosanma1/forge/raw/main/templates/infra/helm/nestjs-service"
	default:
		// Default to go-service chart for backward compatibility
		remoteChart = "https://github.com/dosanma1/forge/raw/main/templates/infra/helm/go-service"
	}

	// Check if user provided a custom chart path (for advanced use cases)
	chartPath := remoteChart
	if path, ok := options["chartPath"].(string); ok && path != "" {
		// If chartPath is provided, treat it as a local path relative to project root
		chartPath = filepath.Join(project.Root, path)
	} else if path, ok := options["configPath"].(string); ok && path != "" {
		// Legacy support for configPath
		chartPath = filepath.Join(project.Root, path)
	}

	// Build values to override
	valuesMap := map[string]string{
		"nameOverride":     projectName,
		"fullnameOverride": projectName,
	}

	// Add port if specified
	if port, ok := options["port"].(int); ok {
		valuesMap["service.port"] = fmt.Sprintf("%d", port)
	}

	// Add health check path if specified
	if healthPath, ok := options["healthPath"].(string); ok && healthPath != "" {
		valuesMap["healthCheck.path"] = healthPath
	}
	
	// DO NOT set image.repository here - Skaffold will automatically inject the image
	// based on the artifact's ImageName matching the pattern in the deployment template

	// Build base values files list (if using local chart)
	var valuesFiles []string
	useHelmSecrets := false
	if path, ok := options["chartPath"].(string); ok && path != "" {
		// Add base values.yaml
		baseValuesPath := filepath.Join(project.Root, path, "values.yaml")
		valuesFiles = append(valuesFiles, baseValuesPath)
	} else if path, ok := options["configPath"].(string); ok && path != "" {
		// Legacy support
		baseValuesPath := filepath.Join(project.Root, path, "values.yaml")
		valuesFiles = append(valuesFiles, baseValuesPath)
	}

	release := latest.HelmRelease{
		Name:                  projectName,
		ChartPath:             chartPath,
		ValuesFiles:           valuesFiles,
		Namespace:             namespace,
		CreateNamespace:       &[]bool{true}[0],
		Wait:                  false, // avoid blocking deploy on pods becoming Ready; readiness is handled separately
		RecreatePods:          false,
		SkipBuildDependencies: false, // Build chart dependencies so bundled Postgres can be pulled when enabled
		SetValueTemplates:     valuesMap,
		UseHelmSecrets:        useHelmSecrets,
	}

	return release
}

// getStringOption safely extracts a string value from options map.
func getStringOption(options map[string]interface{}, key string, defaultValue string) string {
	if value, ok := options[key].(string); ok && value != "" {
		return value
	}
	return defaultValue
}

// GetRegistryForProject extracts the registry from project's build options.
func GetRegistryForProject(project workspace.Project, options map[string]interface{}) string {
	// Check deploy options first
	if registry, ok := options["registry"].(string); ok && registry != "" {
		return registry
	}
	
	// Check build options
	if project.Architect != nil && project.Architect.Build != nil && project.Architect.Build.Options != nil {
		if registry, ok := project.Architect.Build.Options["registry"].(string); ok && registry != "" {
			return registry
		}
	}
	
	// Default fallback
	return "gcr.io/default-project"
}

// createHelmReleaseForInstance creates a Helm release for a specific instance of a service.
// Instance name is used for the release name and to load instance-specific values.
func createHelmReleaseForInstance(projectName, instanceName string, project workspace.Project, deployTarget *workspace.ArchitectTarget, workspaceRoot string) latest.HelmRelease {
	options := deployTarget.Options
	if options == nil {
		options = make(map[string]interface{})
	}

	// Get namespace from options
	namespace := "default"
	if ns, ok := options["namespace"].(string); ok && ns != "" {
		namespace = ns
	}

	// Determine chart path (prefer local if specified)
	var chartPath string
	if path, ok := options["chartPath"].(string); ok && path != "" {
		chartPath = filepath.Join(project.Root, path)
	} else if path, ok := options["configPath"].(string); ok && path != "" {
		// Legacy support
		chartPath = filepath.Join(project.Root, path)
	} else {
		// Fallback to remote chart
		switch project.Language {
		case "go":
			chartPath = "https://github.com/dosanma1/forge/raw/main/templates/infra/helm/go-service"
		case "nestjs":
			chartPath = "https://github.com/dosanma1/forge/raw/main/templates/infra/helm/nestjs-service"
		default:
			chartPath = "https://github.com/dosanma1/forge/raw/main/templates/infra/helm/go-service"
		}
	}

	// Build values to override
	valuesMap := map[string]string{
		"nameOverride":     instanceName,
		"fullnameOverride": instanceName,
	}

	// Add port if specified
	if port, ok := options["port"].(int); ok {
		valuesMap["service.port"] = fmt.Sprintf("%d", port)
	}

	// Add health check path if specified
	if healthPath, ok := options["healthPath"].(string); ok && healthPath != "" {
		valuesMap["healthCheck.path"] = healthPath
	}
	
	// DO NOT set image.repository here - Skaffold will automatically inject the image
	// based on the artifact's ImageName matching the pattern in the deployment template

	// Build values files list
	valuesFiles := []string{}
	useHelmSecrets := false

	// Try to load instance-specific values file
	// Pattern: deploy/helm/values-{instanceName}.yaml
	if path, ok := options["chartPath"].(string); ok && path != "" {
		instanceValuesPath := filepath.Join(project.Root, path, fmt.Sprintf("values-%s.yaml", instanceName))
		valuesFiles = append(valuesFiles, instanceValuesPath)
	} else if path, ok := options["configPath"].(string); ok && path != "" {
		instanceValuesPath := filepath.Join(project.Root, path, fmt.Sprintf("values-%s.yaml", instanceName))
		valuesFiles = append(valuesFiles, instanceValuesPath)
	}

	release := latest.HelmRelease{
		Name:                  fmt.Sprintf("%s-%s", projectName, instanceName),
		ChartPath:             chartPath,
		ValuesFiles:           valuesFiles,
		Namespace:             namespace,
		CreateNamespace:       &[]bool{true}[0],
		Wait:                  false, // avoid blocking deploy on pods becoming Ready; readiness is handled separately
		RecreatePods:          false,
		SkipBuildDependencies: false, // Build chart dependencies so bundled services (e.g., Postgres) are pulled
		SetValueTemplates:     valuesMap,
		UseHelmSecrets:        useHelmSecrets,
	}

	return release
}
