package deployer

import (
	"context"
	"fmt"
)

// HelmDeployer implements Helm deployment
// Note: This is used for direct Helm deployments when Skaffold is not available
// When Skaffold is available, deployment is handled by Skaffold orchestration
type HelmDeployer struct{}

// NewHelmDeployer creates a new Helm deployer
func NewHelmDeployer() *HelmDeployer {
	return &HelmDeployer{}
}

// Name returns the deployer identifier
func (d *HelmDeployer) Name() string {
	return "@forge/helm:deploy"
}

// SupportsSkaffold returns true as Helm works with Skaffold
func (d *HelmDeployer) SupportsSkaffold() bool {
	return true
}

// Deploy executes direct Helm deployment
// This is only called when Skaffold cannot be used
func (d *HelmDeployer) Deploy(ctx context.Context, opts *DeployOptions) error {
	if opts.Verbose {
		fmt.Printf("🚀 Deploying with Helm (direct mode): %s\n", opts.Project)
	}

	// TODO: Implement direct Helm deployment
	// This would be used for cases where Skaffold cannot be used
	// (e.g., builder doesn't support Skaffold)

	return fmt.Errorf("direct Helm deployment not yet implemented - use Skaffold-compatible builders")
}
