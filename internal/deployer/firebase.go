package deployer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/builder"
)

// FirebaseDeployer implements Firebase deployment
type FirebaseDeployer struct{}

// NewFirebaseDeployer creates a new Firebase deployer
func NewFirebaseDeployer() *FirebaseDeployer {
	return &FirebaseDeployer{}
}

// Name returns the deployer identifier
func (d *FirebaseDeployer) Name() string {
	return "@forge/firebase:deploy"
}

// SupportsSkaffold returns false as Firebase doesn't support Skaffold
func (d *FirebaseDeployer) SupportsSkaffold() bool {
	return false
}

// Deploy executes Firebase deployment
func (d *FirebaseDeployer) Deploy(ctx context.Context, opts *DeployOptions) error {
	if opts.Verbose {
		fmt.Printf("🚀 Deploying to Firebase: %s\n", opts.Project)
	}

	// Determine public directory
	var publicDir string

	if opts.Artifact != nil {
		// Validate artifact type
		if opts.Artifact.Type != builder.ArtifactTypeStatic && opts.Artifact.Type != builder.ArtifactTypeTar {
			return fmt.Errorf("firebase requires static files or tar archive, got %s", opts.Artifact.Type)
		}

		publicDir = opts.Artifact.Path
	} else {
		// If no artifact (skip-build), use default output path from options
		if outputPath, ok := opts.Options["outputPath"].(string); ok {
			publicDir = filepath.Join(opts.ProjectRoot, outputPath)
		} else {
			publicDir = filepath.Join(opts.ProjectRoot, "dist")
		}
	}

	// If artifact is a tar, extract it first
	if opts.Artifact != nil && opts.Artifact.Type == builder.ArtifactTypeTar {
		extractDir := filepath.Join(opts.WorkspaceRoot, ".forge", "firebase-deploy", opts.Project)
		if err := os.MkdirAll(extractDir, 0755); err != nil {
			return fmt.Errorf("failed to create extract directory: %w", err)
		}

		if opts.Verbose {
			fmt.Printf("   Extracting artifact to %s\n", extractDir)
		}

		// Extract tar
		cmd := exec.CommandContext(ctx, "tar", "-xzf", opts.Artifact.Path, "-C", extractDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract artifact: %w", err)
		}

		publicDir = extractDir
	}

	// Get Firebase project from options
	firebaseProject := ""
	if proj, ok := opts.Options["project"].(string); ok {
		firebaseProject = proj
	}

	// Build Firebase command
	args := []string{"deploy", "--only", "hosting"}

	if firebaseProject != "" {
		args = append(args, "--project", firebaseProject)
	}

	// Add public directory override
	args = append(args, "--public", publicDir)

	if opts.Verbose {
		fmt.Printf("   Running: firebase %v\n", args)
	}

	cmd := exec.CommandContext(ctx, "firebase", args...)
	cmd.Dir = opts.ProjectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("firebase deploy failed: %w", err)
	}

	if opts.Verbose {
		fmt.Printf("✅ Firebase deployment completed\n")
	}

	return nil
}
