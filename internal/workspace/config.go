// Package workspace provides workspace configuration management.
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ConfigFileName = "forge.json"

// Config represents the workspace configuration.
type Config struct {
	Schema         string             `json:"$schema,omitempty"`
	Version        string             `json:"version"`
	Workspace      WorkspaceMetadata  `json:"workspace"`
	NewProjectRoot string             `json:"newProjectRoot,omitempty"`
	Projects       map[string]Project `json:"projects"`
}

// Architect contains build, serve, deploy, and test targets
type Architect struct {
	Build  *ArchitectTarget `json:"build,omitempty"`
	Serve  *ArchitectTarget `json:"serve,omitempty"`
	Deploy *ArchitectTarget `json:"deploy,omitempty"`
	Test   *ArchitectTarget `json:"test,omitempty"`
}

// ArchitectTarget represents a build/serve/deploy/test target
type ArchitectTarget struct {
	Builder              string                 `json:"builder,omitempty"`
	Deployer             string                 `json:"deployer,omitempty"`
	Options              map[string]interface{} `json:"options,omitempty"`
	Configurations       map[string]interface{} `json:"configurations,omitempty"`
	DefaultConfiguration string                 `json:"defaultConfiguration,omitempty"`
}

// WorkspaceMetadata contains workspace-level metadata.
type WorkspaceMetadata struct {
	Name         string             `json:"name"`
	ForgeVersion string             `json:"forgeVersion"`
	ToolVersions *ToolVersions      `json:"toolVersions,omitempty"`
	Paths        *WorkspacePaths    `json:"paths,omitempty"`
	Defaults     *WorkspaceDefaults `json:"defaults,omitempty"`
	GitHub       *GitHubConfig      `json:"github,omitempty"`
	Docker       *DockerConfig      `json:"docker,omitempty"`
	GCP          *GCPConfig         `json:"gcp,omitempty"`
	Kubernetes   *KubernetesConfig  `json:"kubernetes,omitempty"`
	GazelleDirectives []string      `json:"gazelleDirectives,omitempty"`
}

// WorkspaceDefaults contains workspace-level defaults for projects
type WorkspaceDefaults struct {
	BuildEnvironment         string            `json:"buildEnvironment,omitempty"`         // Default: "local"
	AngularEnvironmentMapper map[string]string `json:"angularEnvironmentMapper,omitempty"` // Maps forge env to Angular config
}

// ToolVersions contains locked versions of framework tools.
type ToolVersions struct {
	Angular string `json:"angular,omitempty"` // Angular CLI and framework version
	Go      string `json:"go,omitempty"`      // Go SDK version
	NestJS  string `json:"nestjs,omitempty"`  // NestJS CLI and core version
	Node    string `json:"node,omitempty"`    // Node.js version
	Bazel   string `json:"bazel,omitempty"`   // Bazel build tool version
}

// WorkspacePaths contains workspace directory structure configuration.
type WorkspacePaths struct {
	Services       string `json:"services,omitempty"`
	FrontendApps   string `json:"frontendApps,omitempty"`
	Infrastructure string `json:"infrastructure,omitempty"`
	Shared         string `json:"shared,omitempty"`
	Docs           string `json:"docs,omitempty"`
}

// GitHubConfig contains GitHub-related configuration.
type GitHubConfig struct {
	Org string `json:"org"`
}

// DockerConfig contains Docker registry configuration.
type DockerConfig struct {
	Registry string `json:"registry"`
}

// GCPConfig contains Google Cloud Platform configuration.
type GCPConfig struct {
	ProjectID string `json:"projectId"`
	Region    string `json:"region,omitempty"`
}

// KubernetesConfig contains Kubernetes configuration.
type KubernetesConfig struct {
	Namespace string `json:"namespace"`
	Context   string `json:"context,omitempty"`
}

// Project represents a project in the workspace.
type Project struct {
	ProjectType string                 `json:"projectType"`
	Language    string                 `json:"language"`
	Root        string                 `json:"root"`
	Tags        []string               `json:"tags,omitempty"`
	Architect   *Architect             `json:"architect,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProjectKind represents the kind of project
type ProjectKind string

const (
	ProjectKindApplication ProjectKind = "application"
	ProjectKindService     ProjectKind = "service"
	ProjectKindLibrary     ProjectKind = "library"
)

// LanguageType represents the programming language/framework
type LanguageType string

const (
	LanguageGo      LanguageType = "go"
	LanguageNestJS  LanguageType = "nestjs"
	LanguageAngular LanguageType = "angular"
	LanguageReact   LanguageType = "react"
	LanguageVue     LanguageType = "vue"
)

// NewConfig creates a new workspace configuration.
func NewConfig(name string) *Config {
	return &Config{
		Version: "1",
		Workspace: WorkspaceMetadata{
			Name:         name,
			ForgeVersion: "1.0.0",
		},
		Projects: make(map[string]Project),
	}
}

// LoadConfig loads the workspace configuration from the current directory.
func LoadConfig(dir string) (*Config, error) {
	configPath := filepath.Join(dir, ConfigFileName)
	return LoadConfigFrom(configPath)
}

// LoadConfigFrom loads the workspace configuration from the specified file.
func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// LoadConfigWithoutProjectValidation loads the workspace configuration without validating projects.
// This is useful during workspace initialization when projects are being added.
func LoadConfigWithoutProjectValidation(dir string) (*Config, error) {
	configPath := filepath.Join(dir, ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Only validate workspace name
	if config.Workspace.Name == "" {
		return nil, fmt.Errorf("workspace.name is required")
	}

	return &config, nil
}

// Save saves the configuration to the default location.
func (c *Config) Save(dir string) error {
	return c.SaveToDir(dir)
}

// SaveToDir saves the configuration to the specified directory.
func (c *Config) SaveToDir(dir string) error {
	configPath := filepath.Join(dir, ConfigFileName)
	return c.SaveTo(configPath)
}

// SaveTo saves the configuration to the specified file.
func (c *Config) SaveTo(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// AddProject adds a project to the workspace.
func (c *Config) AddProject(name string, project *Project) error {
	if _, exists := c.Projects[name]; exists {
		return fmt.Errorf("project %q already exists", name)
	}

	c.Projects[name] = *project
	return nil
}

// RemoveProject removes a project from the workspace.
func (c *Config) RemoveProject(name string) error {
	if _, exists := c.Projects[name]; !exists {
		return fmt.Errorf("project %q not found", name)
	}

	delete(c.Projects, name)
	return nil
}

// GetProject retrieves a project by name.
func (c *Config) GetProject(name string) *Project {
	if project, exists := c.Projects[name]; exists {
		return &project
	}
	return nil
}

// ListProjects returns all projects.
func (c *Config) ListProjects() []Project {
	projects := make([]Project, 0, len(c.Projects))
	for _, project := range c.Projects {
		projects = append(projects, project)
	}
	return projects
}

// Validate validates the workspace configuration.
func (c *Config) Validate() error {
	// Check workspace name
	if c.Workspace.Name == "" {
		return fmt.Errorf("workspace.name is required")
	}

	// Check projects exist
	if len(c.Projects) == 0 {
		return fmt.Errorf("at least one project is required")
	}

	// Validate each project
	for name, project := range c.Projects {
		if err := c.validateProject(name, project); err != nil {
			return fmt.Errorf("project %q: %w", name, err)
		}
	}

	return nil
}

// validateProject validates a single project configuration.
func (c *Config) validateProject(name string, project Project) error {
	// Check project root
	if project.Root == "" {
		return fmt.Errorf("root is required")
	}

	// Check architect section
	if project.Architect == nil {
		return fmt.Errorf("architect is required")
	}

	// Check build configuration
	if project.Architect.Build == nil {
		return fmt.Errorf("architect.build is required")
	}

	if project.Architect.Build.Configurations == nil || len(project.Architect.Build.Configurations) == 0 {
		return fmt.Errorf("architect.build.configurations must have at least one configuration")
	}

	// For libraries, deploy configuration is optional
	// Only services and applications require deployment configuration
	if project.ProjectType != "library" {
		// Check deploy configuration
		if project.Architect.Deploy == nil {
			return fmt.Errorf("architect.deploy is required")
		}

		if project.Architect.Deploy.Configurations == nil || len(project.Architect.Deploy.Configurations) == 0 {
			return fmt.Errorf("architect.deploy.configurations must have at least one configuration")
		}

		// Validate configuration keys match between build and deploy
		buildKeys := make(map[string]bool)
		for key := range project.Architect.Build.Configurations {
			buildKeys[key] = true
		}

		deployKeys := make(map[string]bool)
		for key := range project.Architect.Deploy.Configurations {
			deployKeys[key] = true
		}

		// Check that all build configs exist in deploy configs
		for buildKey := range buildKeys {
			if !deployKeys[buildKey] {
				return fmt.Errorf("build configuration %q does not have a matching deploy configuration", buildKey)
			}
		}

		// Check that all deploy configs exist in build configs
		for deployKey := range deployKeys {
			if !buildKeys[deployKey] {
				return fmt.Errorf("deploy configuration %q does not have a matching build configuration", deployKey)
			}
		}
	}

	return nil
}
