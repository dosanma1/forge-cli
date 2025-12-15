package skaffold

import (
	"fmt"
	"path/filepath"

	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/dosanma1/forge-cli/internal/workspace"
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
			release := createHelmRelease(projectName, project, deployTarget, workspaceRoot)
			helmReleases = append(helmReleases, release)

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
	if path, ok := options["configPath"].(string); ok && path != "" {
		// If configPath is provided, treat it as a local path relative to project root
		chartPath = filepath.Join(project.Root, path)
	}

	// Build values to override
	valuesMap := map[string]string{
		"image.repository": fmt.Sprintf("{{.IMAGE_REPO_%s}}", projectName),
		"image.tag":        fmt.Sprintf("{{.IMAGE_TAG_%s}}@{{.IMAGE_DIGEST_%s}}", projectName, projectName),
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

	release := latest.HelmRelease{
		Name:                  projectName,
		ChartPath:             chartPath,
		Namespace:             namespace,
		CreateNamespace:       &[]bool{true}[0],
		Wait:                  true,
		RecreatePods:          false,
		SkipBuildDependencies: false,
		SetValueTemplates:     valuesMap,
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
