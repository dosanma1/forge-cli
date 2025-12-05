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
	Version        string                       `json:"version"`
	Workspace      WorkspaceMetadata            `json:"workspace"`
	Projects       map[string]Project           `json:"projects"`
	Build          *BuildConfig                 `json:"build,omitempty"`
	Environments   map[string]EnvironmentConfig `json:"environments,omitempty"`
	Infrastructure *InfrastructureConfig        `json:"infrastructure,omitempty"`
}

// BuildConfig contains build-related configuration.
type BuildConfig struct {
	GoVersion   string          `json:"goVersion,omitempty"`
	NodeVersion string          `json:"nodeVersion,omitempty"`
	Registry    string          `json:"registry,omitempty"`
	Cache       *CacheConfig    `json:"cache,omitempty"`
	Parallel    *ParallelConfig `json:"parallel,omitempty"`
}

// InfrastructureConfig contains infrastructure configuration.
type InfrastructureConfig struct {
	Kubernetes       *KubernetesInfra `json:"kubernetes,omitempty"`
	CloudRun         *CloudRunInfra   `json:"cloudrun,omitempty"`
	DeploymentTarget string           `json:"deploymentTarget,omitempty"` // "kubernetes" | "cloudrun" | "both"
}

// KubernetesInfra contains Kubernetes infrastructure config.
type KubernetesInfra struct {
	Cluster   string `json:"cluster,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// CloudRunInfra contains Cloud Run infrastructure config.
type CloudRunInfra struct {
	Region  string `json:"region,omitempty"`
	Project string `json:"project,omitempty"`
}

// EnvironmentConfig contains environment-specific deployment configuration.
type EnvironmentConfig struct {
	Name        string            `json:"name"`
	Profile     string            `json:"profile,omitempty"`
	Cluster     string            `json:"cluster,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Registry    string            `json:"registry,omitempty"`
	Region      string            `json:"region,omitempty"`
	Description string            `json:"description,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
}

// CacheConfig contains build cache configuration.
type CacheConfig struct {
	RemoteURL string `json:"remoteUrl,omitempty"`
}

// ParallelConfig contains parallel build configuration.
type ParallelConfig struct {
	Workers int `json:"workers,omitempty"`
}

// WorkspaceMetadata contains workspace-level metadata.
type WorkspaceMetadata struct {
	Name         string            `json:"name"`
	ForgeVersion string            `json:"forgeVersion"`
	GitHub       *GitHubConfig     `json:"github,omitempty"`
	Docker       *DockerConfig     `json:"docker,omitempty"`
	GCP          *GCPConfig        `json:"gcp,omitempty"`
	Kubernetes   *KubernetesConfig `json:"kubernetes,omitempty"`
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
	Name         string                              `json:"name"`
	Type         ProjectType                         `json:"type"`
	Root         string                              `json:"root"`
	Tags         []string                            `json:"tags,omitempty"`
	Port         int                                 `json:"port,omitempty"`
	Environments map[string]ProjectEnvironmentConfig `json:"environments,omitempty"`
}

// ProjectEnvironmentConfig contains per-project environment configuration.
type ProjectEnvironmentConfig struct {
	Replicas         int                     `json:"replicas,omitempty"`
	Resources        *ProjectResourcesConfig `json:"resources,omitempty"`
	Variables        map[string]string       `json:"variables,omitempty"`
	DeploymentTarget string                  `json:"deploymentTarget,omitempty"` // Override workspace-level deployment target
}

// ProjectResourcesConfig contains resource limits/requests.
type ProjectResourcesConfig struct {
	Memory string `json:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty"`
}

// ProjectType represents the type of project.
type ProjectType string

const (
	ProjectTypeGoService     ProjectType = "go-service"
	ProjectTypeAngularApp    ProjectType = "angular-app"
	ProjectTypeSharedLib     ProjectType = "shared-lib"
	ProjectTypeTypescriptLib ProjectType = "typescript-lib"
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
func (c *Config) AddProject(project *Project) error {
	if _, exists := c.Projects[project.Name]; exists {
		return fmt.Errorf("project %q already exists", project.Name)
	}

	c.Projects[project.Name] = *project
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
