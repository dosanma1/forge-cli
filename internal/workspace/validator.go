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
		if err := v.validateProject(&project); err != nil {
			return fmt.Errorf("project %q: %w", name, err)
		}

		if name != project.Name {
			return fmt.Errorf("project key %q does not match project name %q", name, project.Name)
		}
	}

	return nil
}

// validateProject validates a single project.
func (v *Validator) validateProject(project *Project) error {
	if project.Name == "" {
		return fmt.Errorf("project name is required")
	}

	if err := ValidateName(project.Name); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}

	if project.Root == "" {
		return fmt.Errorf("project root is required")
	}

	if !isValidProjectType(project.Type) {
		return fmt.Errorf("invalid project type: %s", project.Type)
	}

	return nil
}

// ValidateName validates a name follows kebab-case convention.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("name must be kebab-case (lowercase letters, numbers, and hyphens only, starting with a letter)")
	}
	return nil
}

// isValidProjectType checks if a project type is valid.
func isValidProjectType(pt ProjectType) bool {
	switch pt {
	case ProjectTypeGoService, ProjectTypeNestJSService, ProjectTypeAngularApp, ProjectTypeReactApp, ProjectTypeVueApp, ProjectTypeSharedLib:
		return true
	default:
		return false
	}
}
