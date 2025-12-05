// Package config provides configuration resolution with precedence handling.
package config

import (
	"github.com/dosanma1/forge-cli/internal/workspace"
)

// Resolver handles configuration precedence: CLI flags > project.local > project.deploy > environment defaults
type Resolver struct {
	config      *workspace.Config
	environment string
}

// NewResolver creates a new configuration resolver.
func NewResolver(config *workspace.Config, environment string) *Resolver {
	return &Resolver{
		config:      config,
		environment: environment,
	}
}

// ResolveRegistry resolves the Docker registry for a project.
// Precedence: CLI flag > project.build.registry > environment.registry
func (r *Resolver) ResolveRegistry(projectName string, cliRegistry string) string {
	if cliRegistry != "" {
		return cliRegistry
	}

	project, exists := r.config.Projects[projectName]
	if exists && project.Build != nil && project.Build.Registry != "" {
		return project.Build.Registry
	}

	env, exists := r.config.Environments[r.environment]
	if exists && env.Registry != "" {
		return env.Registry
	}

	return ""
}

// ResolvePort resolves the service port for a project.
// Precedence: project.local.{target}.port (for local) > project.deploy.{target}.port > default (8080 for go, 3000 for nestjs)
func (r *Resolver) ResolvePort(projectName string, target string) int {
	project, exists := r.config.Projects[projectName]
	if !exists {
		return getDefaultPort(project.Type)
	}

	// For local environment, check local config
	if r.environment == "local" && project.Local != nil {
		switch target {
		case "cloudrun":
			if project.Local.CloudRun != nil && project.Local.CloudRun.Port != 0 {
				return project.Local.CloudRun.Port
			}
		case "gke", "kubernetes":
			if project.Local.GKE != nil && project.Local.GKE.Port != 0 {
				return project.Local.GKE.Port
			}
		}
	}

	// Check deploy config
	if project.Deploy != nil {
		switch target {
		case "cloudrun":
			if project.Deploy.CloudRun != nil && project.Deploy.CloudRun.Port != 0 {
				return project.Deploy.CloudRun.Port
			}
		case "gke", "kubernetes", "helm":
			if project.Deploy.Helm != nil && project.Deploy.Helm.Port != 0 {
				return project.Deploy.Helm.Port
			}
		}
	}

	return getDefaultPort(project.Type)
}

// ResolveRegion resolves the cloud region.
// Precedence: CLI flag > environment.region > infrastructure.{provider}.region
func (r *Resolver) ResolveRegion(cliRegion string, provider string) string {
	if cliRegion != "" {
		return cliRegion
	}

	env, exists := r.config.Environments[r.environment]
	if exists && env.Region != "" {
		return env.Region
	}

	if r.config.Infrastructure != nil {
		switch provider {
		case "gke":
			if r.config.Infrastructure.GKE != nil && r.config.Infrastructure.GKE.Region != "" {
				return r.config.Infrastructure.GKE.Region
			}
		case "cloudrun":
			if r.config.Infrastructure.CloudRun != nil && r.config.Infrastructure.CloudRun.Region != "" {
				return r.config.Infrastructure.CloudRun.Region
			}
		}
	}

	return "us-central1" // fallback default
}

// ResolveCluster resolves the Kubernetes cluster name.
// Precedence: CLI flag > environment.cluster > infrastructure.kubernetes.cluster
func (r *Resolver) ResolveCluster(cliCluster string) string {
	if cliCluster != "" {
		return cliCluster
	}

	env, exists := r.config.Environments[r.environment]
	if exists && env.Cluster != "" {
		return env.Cluster
	}

	if r.config.Infrastructure != nil && r.config.Infrastructure.Kubernetes != nil {
		return r.config.Infrastructure.Kubernetes.Cluster
	}

	return ""
}

// ResolveNamespace resolves the Kubernetes namespace.
// Precedence: CLI flag > environment.namespace > infrastructure.{provider}.namespace
func (r *Resolver) ResolveNamespace(cliNamespace string, provider string) string {
	if cliNamespace != "" {
		return cliNamespace
	}

	env, exists := r.config.Environments[r.environment]
	if exists && env.Namespace != "" {
		return env.Namespace
	}

	if r.config.Infrastructure != nil {
		switch provider {
		case "gke":
			if r.config.Infrastructure.GKE != nil && r.config.Infrastructure.GKE.Namespace != "" {
				return r.config.Infrastructure.GKE.Namespace
			}
		case "kubernetes":
			if r.config.Infrastructure.Kubernetes != nil && r.config.Infrastructure.Kubernetes.Namespace != "" {
				return r.config.Infrastructure.Kubernetes.Namespace
			}
		}
	}

	return "default"
}

// ResolveDeployTargets returns the deployment targets for a project.
// Defaults to ["helm"] if not specified.
func (r *Resolver) ResolveDeployTargets(projectName string) []string {
	project, exists := r.config.Projects[projectName]
	if !exists || project.Deploy == nil || len(project.Deploy.Targets) == 0 {
		return []string{"helm"} // default to helm
	}

	return project.Deploy.Targets
}

// ResolveConfigPath returns the deployment config path for a project.
// Defaults to "deploy" if not specified.
func (r *Resolver) ResolveConfigPath(projectName string) string {
	project, exists := r.config.Projects[projectName]
	if !exists || project.Deploy == nil || project.Deploy.ConfigPath == "" {
		return "deploy"
	}

	return project.Deploy.ConfigPath
}

// getDefaultPort returns the default port based on project type.
func getDefaultPort(projectType workspace.ProjectType) int {
	switch projectType {
	case workspace.ProjectTypeGoService:
		return 8080
	case workspace.ProjectTypeNestJSService:
		return 3000
	case workspace.ProjectTypeAngularApp:
		return 4200
	case workspace.ProjectTypeReactApp:
		return 3000
	case workspace.ProjectTypeVueApp:
		return 5173
	default:
		return 8080
	}
}
