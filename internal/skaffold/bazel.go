package skaffold

import (
	"fmt"
	"strings"
)

// GenerateBazelTarget generates a Bazel target path from a project root and language.
// Convention for Go services: //{project.root}/cmd/server:image.tar
// Convention for other services: //{project.root}/apps/{project-name}:image.tar (for Angular/NestJS)
// Skaffold requires Bazel targets to end with .tar for container images
// No validation is performed - let Bazel/Skaffold fail with helpful errors if target doesn't exist.
func GenerateBazelTarget(projectRoot, language string) string {
	var target string

	// Ensure the path starts with // for Bazel
	root := projectRoot
	if !strings.HasPrefix(root, "//") {
		root = "//" + root
	}

	// Generate target based on language/project type
	switch language {
	case "go":
		// Go services have the image target in cmd/server
		// Use image_tarball.tar which outputs the actual tarball file (not directory)
		target = fmt.Sprintf("%s/cmd/server:image_tarball.tar", root)
	case "typescript", "angular", "nestjs":
		// TypeScript/Angular/NestJS projects have different structure
		// For now, assume they follow the apps/{project-name} pattern
		// Extract project name from root (e.g., "frontend/apps/web-app" -> "web-app")
		parts := strings.Split(strings.TrimPrefix(root, "//"), "/")
		projectName := parts[len(parts)-1]
		target = fmt.Sprintf("%s:image.tar", root)
		// TODO: Verify this is correct for Angular apps
		_ = projectName
	default:
		// Default fallback
		target = fmt.Sprintf("%s:image.tar", root)
	}

	return target
}

// GetBazelPlatformArgs returns platform-specific Bazel arguments for builds.
// Uses bzlmod naming convention (@rules_go) instead of WORKSPACE (@io_bazel_rules_go)
func GetBazelPlatformArgs(platform string) []string {
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
		// Default to linux/amd64
		return []string{"--platforms=@rules_go//go/toolchain:linux_amd64"}
	}
}
