package skaffold

import (
	"context"
	"fmt"
	"os"

	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/runner"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/runner/runcontext"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/schema/latest"
)

// Executor wraps the Skaffold API for build, deploy, and run operations.
type Executor struct {
	config        *latest.SkaffoldConfig
	workspaceRoot string
}

// NewExecutor creates a new Skaffold executor with the given configuration.
func NewExecutor(config *latest.SkaffoldConfig, workspaceRoot string) *Executor {
	return &Executor{
		config:        config,
		workspaceRoot: workspaceRoot,
	}
}

// Build executes a Skaffold build with the specified profile.
func (e *Executor) Build(ctx context.Context, opts BuildOptions) error {
	if opts.Profile == "" {
		return fmt.Errorf("profile is required for build")
	}

	// Update push setting if specified
	if opts.Push {
		if e.config.Pipeline.Build.BuildType.LocalBuild != nil {
			e.config.Pipeline.Build.BuildType.LocalBuild.Push = boolPtr(true)
		}
	}

	// Create run context
	runCtx, err := e.createRunContext(ctx, opts.Profile, opts.Verbose)
	if err != nil {
		return fmt.Errorf("failed to create run context: %w", err)
	}

	// Create runner
	r, err := runner.NewForConfig(ctx, runCtx)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Get output writer
	out := os.Stdout

	// Execute build with correct signature: Build(ctx, out, artifacts) ([]graph.Artifact, error)
	_, err = r.Build(ctx, out, runCtx.Artifacts())
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	return nil
}

// Deploy executes a Skaffold deploy with the specified profile.
func (e *Executor) Deploy(ctx context.Context, opts DeployOptions) error {
	if opts.Profile == "" {
		return fmt.Errorf("profile is required for deploy")
	}

	// Create run context
	runCtx, err := e.createRunContext(ctx, opts.Profile, opts.Verbose)
	if err != nil {
		return fmt.Errorf("failed to create run context: %w", err)
	}

	// Create runner
	r, err := runner.NewForConfig(ctx, runCtx)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Get output writer
	out := os.Stdout

	if opts.SkipBuild {
		// TODO: Load existing build artifacts and deploy
		return fmt.Errorf("deploy without build not yet implemented")
	} else {
		// Build first
		buildResults, err := r.Build(ctx, out, runCtx.Artifacts())
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		// Render manifests
		manifestList, err := r.Render(ctx, out, buildResults, false)
		if err != nil {
			return fmt.Errorf("render failed: %w", err)
		}

		// Deploy with logs
		err = r.DeployAndLog(ctx, out, buildResults, manifestList)
		if err != nil {
			return fmt.Errorf("deploy failed: %w", err)
		}
	}

	return nil
}

// Run executes a Skaffold dev/run operation with the specified profile.
func (e *Executor) Run(ctx context.Context, opts RunOptions) error {
	if opts.Profile == "" {
		return fmt.Errorf("profile is required for run")
	}

	// Create run context
	runCtx, err := e.createRunContext(ctx, opts.Profile, opts.Verbose)
	if err != nil {
		return fmt.Errorf("failed to create run context: %w", err)
	}

	// Create runner
	r, err := runner.NewForConfig(ctx, runCtx)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Get output writer
	out := os.Stdout

	// Execute dev mode with correct signature: Dev(ctx, out, artifacts) error
	if err := r.Dev(ctx, out, runCtx.Artifacts()); err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	return nil
}

// createRunContext creates a Skaffold run context with the given profile.
func (e *Executor) createRunContext(ctx context.Context, profile string, verbose bool) (*runcontext.RunContext, error) {
	// Apply profile to config
	configWithProfile := e.applyProfile(profile)

	// Create pipelines from the config
	pipelines := runcontext.NewPipelines(
		map[string]latest.Pipeline{
			"default": configWithProfile.Pipeline,
		},
		[]string{"default"},
	)

	// Create run context manually (similar to test pattern)
	runCtx := &runcontext.RunContext{
		Opts: config.SkaffoldOptions{
			// Set trigger to manual for non-watch operations
			// This prevents "unsupported trigger" errors
			Trigger: "manual",
		},
		Pipelines:  pipelines,
		WorkingDir: e.workspaceRoot,
		RunID:      "forge-" + profile,
	}

	return runCtx, nil
}

// applyProfile applies the specified profile to the base configuration.
func (e *Executor) applyProfile(profileName string) *latest.SkaffoldConfig {
	// Clone base config
	config := *e.config

	// Find and apply the profile
	for _, profile := range e.config.Profiles {
		if profile.Name == profileName {
			// Override build artifacts if profile specifies them
			if len(profile.Build.Artifacts) > 0 {
				config.Pipeline.Build.Artifacts = profile.Build.Artifacts
			}

			// Merge build config
			if profile.Build.TagPolicy.GitTagger != nil {
				config.Pipeline.Build.TagPolicy = profile.Build.TagPolicy
			}

			// Override deploy config if profile specifies it
			if profile.Deploy.LegacyHelmDeploy != nil {
				config.Pipeline.Deploy.LegacyHelmDeploy = profile.Deploy.LegacyHelmDeploy
			}
			if profile.Deploy.CloudRunDeploy != nil {
				config.Pipeline.Deploy.CloudRunDeploy = profile.Deploy.CloudRunDeploy
			}

			break
		}
	}

	return &config
}
