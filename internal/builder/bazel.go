package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BazelBuilder implements direct Bazel builds
type BazelBuilder struct{}

// NewBazelBuilder creates a new Bazel builder
func NewBazelBuilder() *BazelBuilder {
	return &BazelBuilder{}
}

// Name returns the builder identifier
func (b *BazelBuilder) Name() string {
	return "@forge/bazel:build"
}

// Validate validates the build options
func (b *BazelBuilder) Validate(opts *BuildOptions) error {
	return nil
}

// Build executes a Bazel build
func (b *BazelBuilder) Build(ctx context.Context, opts *BuildOptions) (*BuildArtifact, error) {
	// Determine target from options or use default
	target := ":build"
	if t, ok := opts.Options["target"].(string); ok && t != "" {
		target = t
	}

	// Construct Bazel target path
	// Convert absolute path to Bazel package path
	relPath, err := filepath.Rel(opts.WorkspaceRoot, opts.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path: %w", err)
	}

	bazelTarget := fmt.Sprintf("//%s%s", relPath, target)

	if opts.Verbose {
		fmt.Printf("🔨 Building with Bazel: %s\n", bazelTarget)
	}

	// Build Bazel command
	args := []string{"build", bazelTarget}

	// Add platform flags if specified
	if opts.Platform != "" {
		args = append(args, getPlatformArgs(opts.Platform)...)
	}

	// Add configuration-specific flags
	if opts.Configuration == "local" || opts.Configuration == "development" {
		args = append(args, "--compilation_mode=dbg")
	} else {
		args = append(args, "--compilation_mode=opt")
	}

	cmd := exec.CommandContext(ctx, "bazel", args...)
	cmd.Dir = opts.WorkspaceRoot

	if opts.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("bazel build failed: %w", err)
	}

	// Determine artifact type based on target name
	artifactType := determineArtifactType(target)

	// Construct output path
	outputPath := filepath.Join(opts.WorkspaceRoot, "bazel-bin", relPath, strings.TrimPrefix(target, ":"))

	artifact := &BuildArtifact{
		Type: artifactType,
		Path: outputPath,
		Tag:  opts.Configuration,
		Metadata: map[string]interface{}{
			"target":   bazelTarget,
			"platform": opts.Platform,
		},
	}

	if opts.Verbose {
		fmt.Printf("✅ Bazel build completed: %s at %s\n", artifactType, outputPath)
	}

	return artifact, nil
}

// determineArtifactType infers the artifact type from the target name
func determineArtifactType(target string) ArtifactType {
	target = strings.ToLower(target)

	if strings.Contains(target, "image") || strings.Contains(target, ".tar") {
		return ArtifactTypeImage
	}
	if strings.Contains(target, "dist") || strings.Contains(target, "build") {
		// Could be static files or tarball
		if strings.HasSuffix(target, ".tar.gz") || strings.HasSuffix(target, ".tgz") {
			return ArtifactTypeTar
		}
		return ArtifactTypeStatic
	}

	// Default to binary for executables
	return ArtifactTypeBinary
}

// getPlatformArgs returns Bazel platform arguments for the given platform
func getPlatformArgs(platform string) []string {
	switch platform {
	case "linux/amd64":
		return []string{"--platforms=@rules_go//go/toolchain:linux_amd64"}
	case "linux/arm64":
		return []string{"--platforms=@rules_go//go/toolchain:linux_arm64"}
	case "darwin/amd64":
		return []string{"--platforms=@rules_go//go/toolchain:darwin_amd64"}
	case "darwin/arm64":
		return []string{"--platforms=@rules_go//go/toolchain:darwin_arm64"}
	default:
		return []string{"--platforms=@rules_go//go/toolchain:linux_amd64"}
	}
}
