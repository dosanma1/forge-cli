package builder

// ArtifactType represents the type of artifact produced by a builder
type ArtifactType string

const (
	// ArtifactTypeImage represents a container image (tarball)
	ArtifactTypeImage ArtifactType = "image"
	// ArtifactTypeStatic represents static files (HTML, CSS, JS)
	ArtifactTypeStatic ArtifactType = "static"
	// ArtifactTypeBinary represents a compiled binary
	ArtifactTypeBinary ArtifactType = "binary"
	// ArtifactTypeTar represents a tarball archive
	ArtifactTypeTar ArtifactType = "tar"
)

// BuildArtifact represents the output of a build operation
type BuildArtifact struct {
	// Type of artifact produced
	Type ArtifactType
	// Path to the artifact (local filesystem path)
	Path string
	// Tag for container images
	Tag string
	// ImageName for container images (e.g., "gcr.io/project/service")
	ImageName string
	// Metadata contains additional build information
	Metadata map[string]interface{}
}
