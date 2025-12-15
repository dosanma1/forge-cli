package deployer

import (
	"context"

	"github.com/dosanma1/forge-cli/internal/builder"
)

// DeployOptions contains options for deploying a project
type DeployOptions struct {
	// Project name being deployed
	Project string
	// Artifact produced by the builder (may be nil if skip-build)
	Artifact *builder.BuildArtifact
	// Builder name that produced the artifact
	Builder string
	// Configuration is the deploy configuration (development, production, etc.)
	Configuration string
	// Options are deployer-specific options from forge.json
	Options map[string]interface{}
	// Verbose enables detailed output
	Verbose bool
	// WorkspaceRoot is the absolute path to the workspace root
	WorkspaceRoot string
	// ProjectRoot is the absolute path to the project root
	ProjectRoot string
}

// Deployer is the interface that all deployers must implement
type Deployer interface {
	// Deploy executes the deployment
	Deploy(ctx context.Context, opts *DeployOptions) error

	// Name returns the deployer name (e.g., "@forge/helm:deploy")
	Name() string

	// SupportsSkaffold returns true if this deployer can work with Skaffold
	SupportsSkaffold() bool
}
