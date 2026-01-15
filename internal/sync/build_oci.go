package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureOciSupport guarantees MODULE.bazel has rules_oci and a distroless base image.
// This is idempotent and will no-op if rules_oci is already present.
func (s *Syncer) ensureOciSupport() error {
	modulePath := filepath.Join(s.workspaceRoot, "MODULE.bazel")

	content, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	if strings.Contains(string(content), "rules_oci") {
		return nil
	}

	snippet := `

# OCI container rules
bazel_dep(name = "rules_oci", version = "2.0.0")

oci = use_extension("@rules_oci//oci:extensions.bzl", "oci")
oci.pull(
    name = "distroless_base",
    image = "gcr.io/distroless/static-debian12",
	tag = "latest",
)

use_repo(
    oci,
    "distroless_base",
)
`

	updated := string(content) + snippet

	if err := os.WriteFile(modulePath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to update MODULE.bazel with rules_oci: %w", err)
	}

	return nil
}

// ensureServiceImageTargets injects oci_image/oci_tarball rules for Go services
// that are built with @forge/bazel:build. Idempotent: skips if image_tarball already exists.
func (s *Syncer) ensureServiceImageTargets() error {
	for name, project := range s.config.Projects {
		if project.ProjectType != "service" {
			continue
		}

		if project.Architect == nil || project.Architect.Build == nil {
			continue
		}

		if project.Architect.Build.Builder != "@forge/bazel:build" {
			continue
		}

		buildFile := filepath.Join(s.workspaceRoot, project.Root, "cmd", "server", "BUILD.bazel")
		contentBytes, err := os.ReadFile(buildFile)
		if err != nil {
			// If the build file doesn't exist yet, skip silently (gazelle may not have generated it)
			continue
		}

		content := strings.ReplaceAll(string(contentBytes), "\r\n", "\n")

		// Normalize legacy oci_tarball loads even if the new targets already exist
		lines := []string{}
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "oci_tarball") && strings.HasPrefix(strings.TrimSpace(line), "load(") {
				continue
			}
			lines = append(lines, line)
		}
		content = strings.Join(lines, "\n")

		hasImageTarball := strings.Contains(content, "image_tarball")
		hasOciLoadRule := strings.Contains(content, "oci_load(")

		// Determine image repo tag
		registry := "gcr.io/your-project"
		if project.Architect.Build.Options != nil {
			if v, ok := project.Architect.Build.Options["registry"].(string); ok && v != "" {
				registry = v
			}
		}
		repoTag := fmt.Sprintf("%s/%s:local", registry, name)

		// Ensure load statements for pkg_tar and oci defs
		loads := []string{}
		if !strings.Contains(content, "pkg_tar") {
			loads = append(loads, "load(\"@rules_pkg//pkg:tar.bzl\", \"pkg_tar\")")
		}
		if !strings.Contains(content, "@rules_oci//oci:defs.bzl") || strings.Contains(content, "oci_tarball") {
			loads = append(loads, "load(\"@rules_oci//oci:defs.bzl\", \"oci_image\", \"oci_load\")")
		}

		// Insert new loads after existing load block
		if len(loads) > 0 {
			lines := strings.Split(content, "\n")
			insertIdx := 0
			for i, line := range lines {
				if strings.HasPrefix(line, "load(") {
					insertIdx = i + 1
				} else if strings.TrimSpace(line) == "" && i > 0 {
					insertIdx = i + 1
				}
			}
			newLines := append([]string{}, lines[:insertIdx]...)
			newLines = append(newLines, loads...)
			newLines = append(newLines, lines[insertIdx:]...)
			content = strings.Join(newLines, "\n")
		}

		// If image targets already exist, just persist any load fixes and continue
		if hasImageTarball && hasOciLoadRule && !strings.Contains(content, "oci_tarball") {
			if err := os.WriteFile(buildFile, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to update %s: %w", buildFile, err)
			}
			continue
		}

		// If an older oci_tarball rule snippet exists, strip everything from the first pkg_tar onwards
		if strings.Contains(content, "oci_tarball(") {
			if idx := strings.Index(content, "pkg_tar("); idx != -1 {
				content = strings.TrimRight(content[:idx], " \n\t") + "\n"
			}
		}

		// Append container rules when missing
		snippet := fmt.Sprintf(`

pkg_tar(
	name = "server_tar",
	srcs = [":server"],
	package_dir = "/app",
)

oci_image(
	name = "image",
	base = "@distroless_base",
	entrypoint = ["/app/server"],
	tars = [":server_tar"],
)

oci_load(
	name = "image.tar",
	image = ":image",
	repo_tags = ["%s"],
	format = "docker",
)

filegroup(
	name = "image_tarball.tar",
	srcs = [":image.tar"],
	output_group = "tarball",
	visibility = ["//visibility:public"],
)
`, repoTag)

		updated := content + snippet

		if err := os.WriteFile(buildFile, []byte(updated), 0644); err != nil {
			return fmt.Errorf("failed to update %s: %w", buildFile, err)
		}

		fmt.Printf("   Added container image targets to %s\n", filepath.Join(project.Root, "cmd", "server"))
	}

	return nil
}
