package skaffold

import (
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/schema/latest"
)

// GetDefaultConfig returns the default Skaffold configuration managed by forge-cli.
// This serves as the base configuration that profiles inherit from.
func GetDefaultConfig() *latest.SkaffoldConfig {
	return &latest.SkaffoldConfig{
		APIVersion: latest.Version,
		Kind:       "Config",
		Metadata: latest.Metadata{
			Name: "forge-config",
		},
		Pipeline: latest.Pipeline{
			Build: latest.BuildConfig{
				TagPolicy: latest.TagPolicy{
					GitTagger: &latest.GitTagger{
						Variant: "AbbrevCommitSha",
					},
				},
				BuildType: latest.BuildType{
					LocalBuild: &latest.LocalBuild{
						Push: boolPtr(false),
					},
				},
			},
			Deploy:      latest.DeployConfig{},
			PortForward: []*latest.PortForwardResource{},
		},
	}
}
