// Package skaffold provides Skaffold API integration for Forge CLI.
package skaffold

// BuildOptions contains options for Skaffold build operations.
type BuildOptions struct {
	// Profile is the Skaffold profile to use
	Profile string

	// Push determines whether to push images to registry
	Push bool

	// Verbose enables verbose output
	Verbose bool
}

// DeployOptions contains options for Skaffold deploy operations.
type DeployOptions struct {
	// Profile is the Skaffold profile to use
	Profile string

	// SkipBuild skips the build phase
	SkipBuild bool

	// Verbose enables verbose output
	Verbose bool

	// Debug enables debug output including generated Skaffold config
	Debug bool

	// Tail streams logs after deployment
	Tail bool

	// PortForward enables port forwarding
	PortForward bool
}

// RunOptions contains options for Skaffold dev/run operations.
type RunOptions struct {
	// Profile is the Skaffold profile to use
	Profile string

	// Verbose enables verbose output
	Verbose bool

	// Tail streams logs
	Tail bool

	// PortForward enables port forwarding
	PortForward bool
}
