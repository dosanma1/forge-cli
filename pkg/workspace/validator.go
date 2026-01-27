package workspace

import (
	"fmt"
	"regexp"
)

var (
	// namePattern matches valid kebab-case names.
	namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// Validator validates workspace configurations.
type Validator struct{}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the entire configuration.
func (v *Validator) Validate(config *Config) error {
	if err := v.validateWorkspace(&config.Workspace); err != nil {
		return fmt.Errorf("workspace validation failed: %w", err)
	}

	if err := v.validateProjects(config.Projects); err != nil {
		return fmt.Errorf("projects validation failed: %w", err)
	}

	return nil
}

// validateWorkspace validates workspace metadata.
func (v *Validator) validateWorkspace(ws *WorkspaceMetadata) error {
	if ws.Name == "" {
		return fmt.Errorf("workspace name is required")
	}

	if err := ValidateName(ws.Name); err != nil {
		return fmt.Errorf("invalid workspace name: %w", err)
	}

	if ws.ForgeVersion == "" {
		return fmt.Errorf("forge version is required")
	}

	return nil
}

// validateProjects validates all projects.
func (v *Validator) validateProjects(projects map[string]Project) error {
	for name, project := range projects {
		if err := v.validateProject(name, &project); err != nil {
			return fmt.Errorf("project %q: %w", name, err)
		}
	}

	return nil
}

// validateProject validates a single project.
func (v *Validator) validateProject(name string, project *Project) error {
	if err := ValidateName(name); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}

	if project.Root == "" {
		return fmt.Errorf("project root is required")
	}

	if project.ProjectType == "" {
		return fmt.Errorf("projectType is required")
	}

	if !isValidProjectType(project.ProjectType) {
		return fmt.Errorf("invalid project type: %s", project.ProjectType)
	}

	if project.Language == "" {
		return fmt.Errorf("language is required")
	}

	if !isValidLanguage(project.Language) {
		return fmt.Errorf("invalid language: %s", project.Language)
	}

	// Validate Angular builder with K8s/CloudRun deployer requires Dockerfile
	if err := v.validateAngularKubernetesDeployment(name, project); err != nil {
		return err
	}

	return nil
}

// validateAngularKubernetesDeployment validates that Angular projects deploying
// to Kubernetes or CloudRun have a Dockerfile to containerize the build output.
func (v *Validator) validateAngularKubernetesDeployment(name string, project *Project) error {
	// Check if project uses Angular builder
	if project.Architect == nil || project.Architect.Build == nil {
		return nil
	}

	builder := project.Architect.Build.Builder
	if builder != "@forge/angular:build" {
		return nil
	}

	// Check if deploying to Helm or CloudRun
	if project.Architect.Deploy == nil {
		return nil
	}

	deployer := project.Architect.Deploy.Deployer
	if deployer != "@forge/helm:deploy" && deployer != "@forge/cloudrun:deploy" {
		return nil
	}

	// Angular project deploying to K8s/CloudRun must have a Dockerfile
	// We don't actually check filesystem here, just document the requirement
	// The actual error will occur at deploy time if missing
	return fmt.Errorf(
		"project %q uses Angular builder (@forge/angular:build) with Kubernetes/CloudRun deployer (%s). "+
			"Create a Dockerfile at %s/Dockerfile to containerize the Angular build output (e.g., using Nginx to serve static files)",
		name, deployer, project.Root,
	)
}

// ValidateName validates a name follows kebab-case convention.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("name must be kebab-case (lowercase letters, numbers, and hyphens only, starting with a letter)")
	}
	return nil
}

// isValidProjectType checks if a project type is valid.
func isValidProjectType(pt string) bool {
	switch pt {
	case "application", "service", "library":
		return true
	default:
		return false
	}
}

// isValidLanguage checks if a language is valid.
func isValidLanguage(lang string) bool {
	switch lang {
	case "go", "nestjs", "angular", "react", "vue":
		return true
	default:
		return false
	}
}
