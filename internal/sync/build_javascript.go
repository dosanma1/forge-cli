package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

const nestjsBuildTemplate = `load("@rules_pkg//pkg:tar.bzl", "pkg_tar")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_load")

exports_files([
    "package.json",
    "tsconfig.json",
])

# Source files filegroup
filegroup(
    name = "src_files",
    srcs = glob(
        ["src/**/*"],
        allow_empty = False,
    ),
)

# Node modules filegroup
filegroup(
    name = "node_modules",
    srcs = glob(["node_modules/**/*"]),
)

# Build the NestJS application
genrule(
    name = "build",
    srcs = [
        "package.json",
        "tsconfig.json",
        "tsconfig.build.json",
        "nest-cli.json",
        ":node_modules",
        ":src_files",
    ],
    outs = ["dist.tar"],
    cmd = """
        set -e
        
        # Get node_modules directory path
        NODE_MODULES_FILE=$$(echo "$(locations :node_modules)" | awk '{print $$1}')
        NODE_MODULES_DIR=$$(dirname $$(dirname $$NODE_MODULES_FILE))
        NODE_MODULES_PATH=$$(realpath $$NODE_MODULES_DIR)
        
        # Set up working directory
        WORK_DIR=$$(mktemp -d)
        trap "rm -rf $$WORK_DIR" EXIT
        
        # Copy all config files
        cp $(location package.json) $$WORK_DIR/
        cp $(location tsconfig.json) $$WORK_DIR/
        cp $(location tsconfig.build.json) $$WORK_DIR/
        cp $(location nest-cli.json) $$WORK_DIR/
        
        # Copy src directory preserving structure
        mkdir -p $$WORK_DIR/src
        for src_file in $(locations :src_files); do
            # Get relative path from backend/services/{{.ServiceName}}/src/
            rel_path=$${src_file#backend/services/{{.ServiceName}}/src/}
            target_dir=$$(dirname $$WORK_DIR/src/$$rel_path)
            mkdir -p $$target_dir
            cp $$src_file $$target_dir/
        done
        
        # Save output path before changing directories
        OUT_PATH="$$(pwd)/$(location dist.tar)"
        mkdir -p $$(dirname $$OUT_PATH)
        
        # Symlink node_modules instead of copying
        ln -s $$NODE_MODULES_PATH $$WORK_DIR/node_modules
        
        # Build
        cd $$WORK_DIR
        ./node_modules/.bin/nest build
        
        # Copy node_modules for tarball (resolve symlink)
        rm $$WORK_DIR/node_modules
        cp -rL $$NODE_MODULES_PATH $$WORK_DIR/node_modules
        
        # Create tarball with dist and node_modules
        tar -czf $$OUT_PATH dist node_modules package.json
    """,
    visibility = ["//visibility:public"],
)

# Container image
pkg_tar(
    name = "tar",
    srcs = [":build"],
    package_dir = "/app",
)

oci_image(
    name = "image",
    base = "@distroless_nodejs",
    cmd = ["node", "dist/main.js"],
    tars = [":tar"],
    workdir = "/app",
)

# Load image into Docker (for Skaffold)
oci_load(
    name = "image.tar",
    image = ":image",
    repo_tags = ["{{.WorkspaceName}}/{{.ServiceName}}:latest"],
    format = "docker",
)

# Export tarball for Skaffold
filegroup(
    name = "image_tarball.tar",
    srcs = [":image.tar"],
    output_group = "tarball",
    visibility = ["//visibility:public"],
)
`

const angularBuildTemplate = `load("@aspect_rules_js//npm:defs.bzl", "npm_package")
load("@aspect_rules_ts//ts:defs.bzl", "ts_config")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_load")
load("@rules_pkg//pkg:tar.bzl", "pkg_tar")

exports_files([
    "angular.json",
    "package.json",
    "tsconfig.json",
    "tsconfig.app.json",
    "tsconfig.spec.json",
])

# Node modules for this app
filegroup(
    name = "node_modules",
    srcs = glob(["node_modules/**/*"]),
    visibility = ["//visibility:public"],
)

# Source files filegroup
filegroup(
    name = "src_files",
    srcs = glob(
        ["src/**/*"],
        allow_empty = False,
    ),
)

# Public files filegroup
filegroup(
    name = "public_files",
    srcs = glob(
        ["public/**/*"],
        allow_empty = True,
    ),
)

# Build the Angular application
genrule(
    name = "build",
    srcs = [
        ":src_files",
        ":public_files",
        "angular.json",
        "package.json",
        "tsconfig.json",
        "tsconfig.app.json",
        "tsconfig.spec.json",
        ":node_modules",
    ],
    outs = ["dist.tar"],
    cmd = """
        set -e
        
        # Get node_modules directory path
        NODE_MODULES_FILE=$$(echo "$(locations :node_modules)" | awk '{print $$1}')
        NODE_MODULES_DIR=$$(dirname $$(dirname $$NODE_MODULES_FILE))
        NODE_MODULES_PATH=$$(realpath $$NODE_MODULES_DIR)
        
        # Save output path before changing directories
        OUT_PATH="$$(pwd)/$(location dist.tar)"
        
        # Set up working directory
        WORK_DIR=$$(mktemp -d)
        trap "rm -rf $$WORK_DIR" EXIT
        
        # Copy config files to working directory
        cp $(location angular.json) $$WORK_DIR/
        cp $(location package.json) $$WORK_DIR/
        cp $(location tsconfig.json) $$WORK_DIR/
        cp $(location tsconfig.app.json) $$WORK_DIR/
        cp $(location tsconfig.spec.json) $$WORK_DIR/
        
        # Copy src files preserving directory structure
        for src_file in $(locations :src_files); do
            # Get relative path (strip package path prefix)
            rel_path=$${src_file#{{.PackagePath}}/src/}
            # Create target directory
            target_dir=$$(dirname $$WORK_DIR/src/$$rel_path)
            mkdir -p $$target_dir
            # Copy file
            cp $$src_file $$target_dir/
        done
        
        # Copy public files if they exist
        for pub_file in $(locations :public_files); do
            rel_path=$${pub_file#{{.PackagePath}}/public/}
            target_dir=$$(dirname $$WORK_DIR/public/$$rel_path)
            mkdir -p $$target_dir
            cp $$pub_file $$target_dir/
        done
        
        # Symlink node_modules
        ln -s $$NODE_MODULES_PATH $$WORK_DIR/node_modules
        
        # Build Angular application
        cd $$WORK_DIR
        ./node_modules/.bin/ng build --configuration=production
        
        # Create tarball with build output
        tar -czf $$OUT_PATH -C dist .
    """,
    visibility = ["//visibility:public"],
)

# Container image for deployment
pkg_tar(
    name = "tar",
    srcs = [":build"],
    package_dir = "/usr/share/nginx/html",
)

oci_image(
    name = "image",
    base = "@distroless_nodejs",
    tars = [":tar"],
)

oci_load(
    name = "image.tar",
    image = ":image",
    repo_tags = ["{{.WorkspaceName}}/{{.AppName}}:latest"],
    format = "docker",
)

filegroup(
    name = "image_tarball.tar",
    srcs = [":image.tar"],
    output_group = "tarball",
    visibility = ["//visibility:public"],
)
`

// JSBuildData contains template data for JavaScript BUILD generation.
type JSBuildData struct {
	WorkspaceName string
	ServiceName   string
	AppName       string
	PackagePath   string
}

// syncJSBuildFiles regenerates BUILD.bazel for NestJS and Angular projects.
func (s *Syncer) syncJSBuildFiles(report *SyncReport) error {
	for name, project := range s.config.Projects {
		switch project.Language {
		case "nestjs":
			if err := s.generateNestJSBuild(name, project.Root, report); err != nil {
				return fmt.Errorf("failed to generate NestJS BUILD for %s: %w", name, err)
			}
		case "angular", "react":
			if err := s.generateAngularBuild(name, project.Root, report); err != nil {
				return fmt.Errorf("failed to generate Angular BUILD for %s: %w", name, err)
			}
		}
	}
	return nil
}

// generateNestJSBuild creates BUILD.bazel for a NestJS service.
func (s *Syncer) generateNestJSBuild(serviceName, serviceRoot string, report *SyncReport) error {
	tmpl, err := template.New("BUILD.bazel").Parse(nestjsBuildTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := JSBuildData{
		WorkspaceName: s.config.Workspace.Name,
		ServiceName:   serviceName,
		PackagePath:   serviceRoot,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	buildPath := filepath.Join(s.workspaceRoot, serviceRoot, "BUILD.bazel")

	if s.dryRun {
		fmt.Printf("Would write: %s\n", buildPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(buildPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(buildPath, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("failed to write BUILD.bazel: %w", err)
	}

	report.CreatedFiles = append(report.CreatedFiles, buildPath)
	return nil
}

// generateAngularBuild creates BUILD.bazel for an Angular application.
func (s *Syncer) generateAngularBuild(appName, appRoot string, report *SyncReport) error {
	tmpl, err := template.New("BUILD.bazel").Parse(angularBuildTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := JSBuildData{
		WorkspaceName: s.config.Workspace.Name,
		AppName:       appName,
		PackagePath:   appRoot,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	buildPath := filepath.Join(s.workspaceRoot, appRoot, "BUILD.bazel")

	if s.dryRun {
		fmt.Printf("Would write: %s\n", buildPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(buildPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(buildPath, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("failed to write BUILD.bazel: %w", err)
	}

	report.CreatedFiles = append(report.CreatedFiles, buildPath)
	return nil
}
