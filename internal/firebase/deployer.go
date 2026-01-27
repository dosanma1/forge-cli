package firebase

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// Deploy deploys a project to Firebase Hosting using the Firebase CLI.
// It changes to the project's Firebase config directory and runs:
// 1. firebase use <projectId>
// 2. firebase deploy --only hosting (or --only hosting:<target> if target specified)
func Deploy(ctx context.Context, projectName string, project workspace.Project, options map[string]interface{}, env string) error {
	// Extract projectId (required)
	projectID, ok := options["projectId"].(string)
	if !ok || projectID == "" {
		return fmt.Errorf("firebase deployer requires 'projectId' in deploy options for project %q", projectName)
	}

	// Extract optional target
	target, _ := options["target"].(string)

	// Extract configPath (defaults to project root)
	configPath, ok := options["configPath"].(string)
	if !ok || configPath == "" {
		configPath = project.Root
	} else {
		configPath = filepath.Join(project.Root, configPath)
	}

	// Set Firebase project
	if err := setFirebaseProject(ctx, configPath, projectID); err != nil {
		return fmt.Errorf("failed to set Firebase project: %w", err)
	}

	// Deploy to Firebase Hosting
	if err := deployToHosting(ctx, configPath, target); err != nil {
		return fmt.Errorf("failed to deploy to Firebase Hosting: %w", err)
	}

	return nil
}

// setFirebaseProject runs "firebase use <projectId>"
func setFirebaseProject(ctx context.Context, workDir string, projectID string) error {
	cmd := exec.CommandContext(ctx, "firebase", "use", projectID)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("firebase use failed: %w", err)
	}

	return nil
}

// deployToHosting runs "firebase deploy --only hosting" or "firebase deploy --only hosting:<target>"
func deployToHosting(ctx context.Context, workDir string, target string) error {
	args := []string{"deploy", "--only"}
	if target != "" {
		args = append(args, fmt.Sprintf("hosting:%s", target))
	} else {
		args = append(args, "hosting")
	}

	cmd := exec.CommandContext(ctx, "firebase", args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("firebase deploy failed: %w", err)
	}

	return nil
}
