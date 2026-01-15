package skaffold

import (
	"fmt"

	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/dosanma1/forge-cli/internal/workspace"
)

// CreateBazelArtifact creates a Skaffold Bazel artifact configuration for a project.
func CreateBazelArtifact(projectName string, project workspace.Project, registry string, bazelArgs []string) *latest.Artifact {
	target := GenerateBazelTarget(project.Root, project.Language)
	
	// If registry is empty, use just the project name (for local development)
	// This matches the BUILD.bazel file's repo_tags
	var imageName string
	if registry == "" {
		imageName = projectName
	} else {
		imageName = fmt.Sprintf("%s/%s", registry, projectName)
	}

	artifact := &latest.Artifact{
		ImageName: imageName,
		Workspace: project.Root,
		ArtifactType: latest.ArtifactType{
			BazelArtifact: &latest.BazelArtifact{
				BuildTarget: target,
				BuildArgs:   bazelArgs,
			},
		},
	}

	return artifact
}

// CreateBazelArtifacts creates Bazel artifacts for multiple projects.
func CreateBazelArtifacts(projects map[string]workspace.Project, projectNames []string, registry string, platform string) []*latest.Artifact {
	artifacts := make([]*latest.Artifact, 0, len(projectNames))

	// Get platform-specific Bazel args
	bazelArgs := GetBazelPlatformArgs(platform)

	for _, projectName := range projectNames {
		project, exists := projects[projectName]
		if !exists {
			continue
		}

		artifact := CreateBazelArtifact(projectName, project, registry, bazelArgs)
		artifacts = append(artifacts, artifact)
	}

	return artifacts
}

// GetRegistryFromOptions extracts the registry from build options.
// Falls back to the provided default if not specified in options.
func GetRegistryFromOptions(options map[string]interface{}, defaultRegistry string) string {
	if registry, ok := options["registry"].(string); ok && registry != "" {
		return registry
	}
	return defaultRegistry
}
