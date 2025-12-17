package sync

import (
	"fmt"
	"os"
	"path/filepath"
)

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
	data := JSBuildData{
		WorkspaceName: s.config.Workspace.Name,
		ServiceName:   serviceName,
		PackagePath:   serviceRoot,
	}

	content, err := s.engine.RenderTemplate("bazel/nestjs.BUILD.bazel.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	buildPath := filepath.Join(s.workspaceRoot, serviceRoot, "BUILD.bazel")

	if s.dryRun {
		fmt.Printf("Would write: %s\n", buildPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(buildPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(buildPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write BUILD.bazel: %w", err)
	}

	report.CreatedFiles = append(report.CreatedFiles, buildPath)
	return nil
}

// generateAngularBuild creates BUILD.bazel for an Angular application.
func (s *Syncer) generateAngularBuild(appName, appRoot string, report *SyncReport) error {
	data := JSBuildData{
		WorkspaceName: s.config.Workspace.Name,
		AppName:       appName,
		PackagePath:   appRoot,
	}

	content, err := s.engine.RenderTemplate("bazel/angular.BUILD.bazel.tmpl", data)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	buildPath := filepath.Join(s.workspaceRoot, appRoot, "BUILD.bazel")

	if s.dryRun {
		fmt.Printf("Would write: %s\n", buildPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(buildPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(buildPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write BUILD.bazel: %w", err)
	}

	report.CreatedFiles = append(report.CreatedFiles, buildPath)
	return nil
}
