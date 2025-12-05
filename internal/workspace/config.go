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
	Platforms   []string        `json:"platforms,omitempty"` // Target platforms for multi-arch builds
	TagPolicy   string          `json:"tagPolicy,omitempty"` // "gitCommit" | "sha256" | "datetime" | "envTemplate"
	Cache       *CacheConfig    `json:"cache,omitempty"`
	Parallel    *ParallelConfig `json:"parallel,omitempty"`
}

// InfrastructureConfig contains infrastructure configuration.
type InfrastructureConfig struct {
	GKE              *GKEInfra        `json:"gke,omitempty"`
	Kubernetes       *KubernetesInfra `json:"kubernetes,omitempty"`
	CloudRun         *CloudRunInfra   `json:"cloudrun,omitempty"`
	DeploymentTarget string           `json:"deploymentTarget,omitempty"` // "gke" | "kubernetes" | "cloudrun" | "both"
}

// GKEInfra contains Google Kubernetes Engine (GKE) infrastructure config.
type GKEInfra struct {
	ProjectID                string `json:"projectId,omitempty"`
	ClusterName              string `json:"clusterName,omitempty"`
	Region                   string `json:"region,omitempty"` // Use region for regional clusters
	Namespace                string `json:"namespace,omitempty"`
	WorkloadIdentityProvider string `json:"workloadIdentityProvider,omitempty"` // For CI/CD authentication
	ServiceAccount           string `json:"serviceAccount,omitempty"`           // Service account email for Workload Identity
}

// KubernetesInfra contains generic Kubernetes infrastructure config (non-cloud provider).
type KubernetesInfra struct {
	Cluster   string `json:"cluster,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Context   string `json:"context,omitempty"` // Kubeconfig context
}

// CloudRunInfra contains Cloud Run infrastructure config.
type CloudRunInfra struct {
	Region  string `json:"region,omitempty"`
	Project string `json:"project,omitempty"`
}

// EnvironmentConfig contains environment-specific deployment configuration.
type EnvironmentConfig struct {
	Name        string            `json:"name"`
	Target      string            `json:"target,omitempty"` // "gke" | "kubernetes" | "cloudrun" (defaults to kubernetes)
	Profile     string            `json:"profile,omitempty"`
	Cluster     string            `json:"cluster,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Registry    string            `json:"registry,omitempty"`
	Region      string            `json:"region,omitempty"`
	Description string            `json:"description,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Deploy      *DeployConfig     `json:"deploy,omitempty"` // Deployment configuration per environment
}

// DeployConfig contains deployment-specific configuration.
type DeployConfig struct {
	Type     string                `json:"type"`               // "helm" | "cloudrun"
	Helm     *HelmConfig           `json:"helm,omitempty"`     // Helm-specific deployment config
	CloudRun *CloudRunDeployConfig `json:"cloudrun,omitempty"` // Cloud Run-specific deployment config
}

// HelmConfig contains Helm deployer configuration.
type HelmConfig struct {
	ReleasePrefix   string `json:"releasePrefix,omitempty"`
	CreateNamespace bool   `json:"createNamespace,omitempty"`
	Timeout         string `json:"timeout,omitempty"` // e.g., "5m"
	Wait            bool   `json:"wait,omitempty"`
	Atomic          bool   `json:"atomic,omitempty"`
}

// CloudRunDeployConfig contains Cloud Run deployer configuration.
type CloudRunDeployConfig struct {
	ProjectID            string `json:"projectId,omitempty"`
	Region               string `json:"region,omitempty"`
	CPU                  string `json:"cpu,omitempty"`    // e.g., "1", "2"
	Memory               string `json:"memory,omitempty"` // e.g., "512Mi", "1Gi"
	Concurrency          int    `json:"concurrency,omitempty"`
	MinInstances         int    `json:"minInstances,omitempty"`
	MaxInstances         int    `json:"maxInstances,omitempty"`
	Timeout              string `json:"timeout,omitempty"` // e.g., "300s"
	AllowUnauthenticated bool   `json:"allowUnauthenticated,omitempty"`
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
