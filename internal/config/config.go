package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the .forge.yaml configuration file.
type Config struct {
	// Project configuration
	Project ProjectConfig `yaml:"project"`

	// Build configuration
	Build BuildConfig `yaml:"build"`

	// Services in the workspace
	Services []Service `yaml:"services"`

	// Frontend applications
	Frontend []Frontend `yaml:"frontend,omitempty"`

	// Infrastructure configuration
	Infrastructure InfrastructureConfig `yaml:"infrastructure,omitempty"`
}

// ProjectConfig holds project-level settings.
type ProjectConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

// BuildConfig holds build system settings.
type BuildConfig struct {
	GoVersion   string `yaml:"go_version"`
	NodeVersion string `yaml:"node_version"`
	Registry    string `yaml:"registry,omitempty"`
}

// Service represents a backend service.
type Service struct {
	Name        string            `yaml:"name"`
	Type        string            `yaml:"type"` // go, python, etc.
	Path        string            `yaml:"path"`
	Port        int               `yaml:"port"`
	Deployments []string          `yaml:"deployments,omitempty"` // kubernetes, cloudrun
	Env         map[string]string `yaml:"env,omitempty"`
}

// Frontend represents a frontend application.
type Frontend struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"` // angular, react, etc.
	Path string `yaml:"path"`
	Port int    `yaml:"port"`
}

// InfrastructureConfig holds infrastructure settings.
type InfrastructureConfig struct {
	Kubernetes KubernetesConfig `yaml:"kubernetes,omitempty"`
	CloudRun   CloudRunConfig   `yaml:"cloudrun,omitempty"`
}

// KubernetesConfig holds Kubernetes deployment settings.
type KubernetesConfig struct {
	Cluster   string `yaml:"cluster"`
	Namespace string `yaml:"namespace"`
	Context   string `yaml:"context,omitempty"`
}

// CloudRunConfig holds Cloud Run deployment settings.
type CloudRunConfig struct {
	Region  string `yaml:"region"`
	Project string `yaml:"project"`
}

// Load reads and parses the .forge.yaml file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults
	config.applyDefaults()

	// Validate
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Save writes the config to a file.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Project.Name == "" {
		return fmt.Errorf("project.name is required")
	}

	if c.Build.GoVersion == "" {
		return fmt.Errorf("build.go_version is required")
	}

	// Validate services
	serviceNames := make(map[string]bool)
	for _, svc := range c.Services {
		if svc.Name == "" {
			return fmt.Errorf("service name is required")
		}
		if serviceNames[svc.Name] {
			return fmt.Errorf("duplicate service name: %s", svc.Name)
		}
		serviceNames[svc.Name] = true

		if svc.Type == "" {
			return fmt.Errorf("service %s: type is required", svc.Name)
		}
	}

	return nil
}

// applyDefaults sets default values for missing fields.
func (c *Config) applyDefaults() {
	if c.Build.GoVersion == "" {
		c.Build.GoVersion = "1.24.4"
	}
	if c.Build.NodeVersion == "" {
		c.Build.NodeVersion = "20.18.1"
	}
	if c.Project.Version == "" {
		c.Project.Version = "0.1.0"
	}

	// Set default paths for services
	for i := range c.Services {
		if c.Services[i].Path == "" {
			c.Services[i].Path = fmt.Sprintf("backend/services/%s", c.Services[i].Name)
		}
		if c.Services[i].Port == 0 {
			c.Services[i].Port = 8080 + i
		}
	}

	// Set default paths for frontend
	for i := range c.Frontend {
		if c.Frontend[i].Path == "" {
			c.Frontend[i].Path = fmt.Sprintf("frontend/projects/%s", c.Frontend[i].Name)
		}
		if c.Frontend[i].Port == 0 {
			c.Frontend[i].Port = 4200 + i
		}
	}
}

// GetService finds a service by name.
func (c *Config) GetService(name string) (*Service, error) {
	for _, svc := range c.Services {
		if svc.Name == name {
			return &svc, nil
		}
	}
	return nil, fmt.Errorf("service not found: %s", name)
}

// AddService adds a new service to the configuration.
func (c *Config) AddService(svc Service) error {
	// Check for duplicates
	for _, existing := range c.Services {
		if existing.Name == svc.Name {
			return fmt.Errorf("service already exists: %s", svc.Name)
		}
	}

	c.Services = append(c.Services, svc)
	return nil
}

// NewDefaultConfig creates a new config with sensible defaults.
func NewDefaultConfig(projectName string) *Config {
	return &Config{
		Project: ProjectConfig{
			Name:        projectName,
			Description: "A forge-powered microservices workspace",
			Version:     "0.1.0",
		},
		Build: BuildConfig{
			GoVersion:   "1.24.4",
			NodeVersion: "20.18.1",
		},
		Services: []Service{},
		Frontend: []Frontend{},
		Infrastructure: InfrastructureConfig{
			Kubernetes: KubernetesConfig{
				Cluster:   "kind-" + projectName,
				Namespace: "default",
			},
			CloudRun: CloudRunConfig{
				Region:  "us-central1",
				Project: "your-gcp-project",
			},
		},
	}
}
