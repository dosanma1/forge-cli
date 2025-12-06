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
	Environments   map[string]EnvironmentConfig `json:"environments,omitempty"`
	Infrastructure *InfrastructureConfig        `json:"infrastructure,omitempty"`
}

// Deprecated build config structures - kept for backward compatibility during migration
type BuildConfig struct {
	GoVersion   string          `json:"goVersion,omitempty"`
	NodeVersion string          `json:"nodeVersion,omitempty"`
	Registry    string          `json:"registry,omitempty"`
	Platforms   []string        `json:"platforms,omitempty"`
	TagPolicy   string          `json:"tagPolicy,omitempty"`
	Cache       *CacheConfig    `json:"cache,omitempty"`
	Parallel    *ParallelConfig `json:"parallel,omitempty"`
}

// InfrastructureConfig contains infrastructure configuration.
type InfrastructureConfig struct {
	GKE        *GKEInfra        `json:"gke,omitempty"`
	Kubernetes *KubernetesInfra `json:"kubernetes,omitempty"`
	CloudRun   *CloudRunInfra   `json:"cloudrun,omitempty"`
	Firebase   *FirebaseInfra   `json:"firebase,omitempty"`
	Kind       *KindInfra       `json:"kind,omitempty"`
}

// FirebaseInfra contains Firebase infrastructure config.
type FirebaseInfra struct {
	Projects map[string]string `json:"projects,omitempty"` // Environment name -> Firebase project ID
}

// KindInfra contains Kind (local Kubernetes) configuration.
type KindInfra struct {
	ConfigPath string `json:"configPath,omitempty"`
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
	ProjectID string `json:"projectId,omitempty"`
	Region    string `json:"region,omitempty"`
}

// EnvironmentConfig contains environment-specific deployment configuration.
type EnvironmentConfig struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Target      string `json:"target,omitempty"` // "gke" | "cloudrun" | "kind"
	Cluster     string `json:"cluster,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Registry    string `json:"registry,omitempty"`
	Region      string `json:"region,omitempty"`
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
	Paths        *WorkspacePaths   `json:"paths,omitempty"`
	GitHub       *GitHubConfig     `json:"github,omitempty"`
	Docker       *DockerConfig     `json:"docker,omitempty"`
	GCP          *GCPConfig        `json:"gcp,omitempty"`
	Kubernetes   *KubernetesConfig `json:"kubernetes,omitempty"`
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
	Name     string                 `json:"name"`
	Type     ProjectType            `json:"type"`
	Root     string                 `json:"root"`
	Tags     []string               `json:"tags,omitempty"`
	Build    *ProjectBuildConfig    `json:"build,omitempty"`
	Deploy   *ProjectDeployConfig   `json:"deploy,omitempty"`
	Local    *ProjectLocalConfig    `json:"local,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ProjectBuildConfig contains project-specific build configuration.
type ProjectBuildConfig struct {
	GoVersion         string            `json:"goVersion,omitempty"`
	NodeVersion       string            `json:"nodeVersion,omitempty"`
	Registry          string            `json:"registry,omitempty"`
	Dockerfile        string            `json:"dockerfile,omitempty"`
	Platforms         []string          `json:"platforms,omitempty"`
	BuildCommand      string            `json:"buildCommand,omitempty"`
	OutputPath        string            `json:"outputPath,omitempty"`
	EnvironmentMapper map[string]string `json:"environmentMapper,omitempty"` // Maps forge environments to build configs (Angular)
}

// ProjectDeployConfig contains project deployment configuration.
type ProjectDeployConfig struct {
	Targets    []string               `json:"targets,omitempty"`
	ConfigPath string                 `json:"configPath,omitempty"`
	Firebase   *ProjectDeployFirebase `json:"firebase,omitempty"`
}

// ProjectDeployFirebase contains Firebase deployment configuration for a project.
type ProjectDeployFirebase struct {
	Project string `json:"project,omitempty"`
	Site    string `json:"site,omitempty"`
}

// ProjectLocalConfig contains local development configuration overrides.
type ProjectLocalConfig struct {
	CloudRun *ProjectLocalCloudRun `json:"cloudrun,omitempty"`
	GKE      *ProjectLocalGKE      `json:"gke,omitempty"`
	Firebase *ProjectLocalFirebase `json:"firebase,omitempty"`
}

// ProjectLocalCloudRun contains local Cloud Run configuration.
type ProjectLocalCloudRun struct {
	Port int               `json:"port,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
}

// ProjectLocalGKE contains local Kubernetes configuration.
type ProjectLocalGKE struct {
	Port int               `json:"port,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
}

// ProjectLocalFirebase contains local Firebase configuration.
type ProjectLocalFirebase struct {
	Port int `json:"port,omitempty"`
}

// ProjectType represents the type of project.
type ProjectType string

const (
	ProjectTypeGo      ProjectType = "go"
	ProjectTypeNestJS  ProjectType = "nestjs"
	ProjectTypeAngular ProjectType = "angular"
	ProjectTypeReact   ProjectType = "react"
	ProjectTypeVue     ProjectType = "vue"
	ProjectTypeShared  ProjectType = "shared"
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
