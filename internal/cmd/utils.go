package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/pkg/workspace"
)

// findWorkspaceRoot finds the workspace root by looking for forge.json
func findWorkspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Traverse up the directory tree looking for forge.json
	for {
		configPath := filepath.Join(dir, workspace.ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return dir, nil
		}

		// Check if we've reached the root
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("forge.json not found in current directory or any parent directory")
}

// serviceToTarget converts a service name to a Bazel target
// Examples:
//   - "api-server" -> "//backend/services/api-server:api-server"
//   - "web-app" -> "//frontend/apps/web-app:web-app"
func serviceToTarget(service string) string {
	// If already a Bazel target, return as-is
	if strings.HasPrefix(service, "//") {
		return service
	}

	// Try common patterns
	// Backend services: //backend/services/{name}:{name}
	// Frontend apps: //frontend/apps/{name}:{name}
	// For now, assume backend service pattern
	// A more sophisticated implementation would check the workspace config
	return fmt.Sprintf("//backend/services/%s:%s", service, service)
}

// extractServiceNames extracts service names from a list of arguments
// Handles both service names and Bazel targets
func extractServiceNames(args []string) []string {
	if len(args) == 0 {
		return nil
	}

	names := make([]string, 0, len(args))
	for _, arg := range args {
		// If it's a Bazel target, extract the service name
		if strings.HasPrefix(arg, "//") {
			// Extract name from pattern like "//backend/services/api-server:api-server"
			parts := strings.Split(arg, ":")
			if len(parts) == 2 {
				names = append(names, parts[1])
			} else {
				// Extract from path
				pathParts := strings.Split(arg, "/")
				if len(pathParts) > 0 {
					names = append(names, pathParts[len(pathParts)-1])
				}
			}
		} else {
			// Plain service name
			names = append(names, arg)
		}
	}

	return names
}
