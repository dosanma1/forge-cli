package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// updateRootSkaffold adds the service to the root skaffold.yaml requires section
func updateRootSkaffold(workspaceRoot, servicesPath, serviceName string) error {
	skaffoldPath := filepath.Join(workspaceRoot, "skaffold.yaml")

	// Check if root skaffold.yaml exists
	if _, err := os.Stat(skaffoldPath); os.IsNotExist(err) {
		// No root skaffold.yaml, skip update
		return nil
	}

	// Read the file
	content, err := os.ReadFile(skaffoldPath)
	if err != nil {
		return fmt.Errorf("failed to read skaffold.yaml: %w", err)
	}

	// Check if service is already in requires
	servicePath := filepath.Join(servicesPath, serviceName)
	if strings.Contains(string(content), "path: "+servicePath) {
		// Already exists, skip
		return nil
	}

	// Find the requires section and add the service
	lines := strings.Split(string(content), "\n")
	var newLines []string
	inRequires := false
	requiresIndent := ""
	inserted := false

	for i, line := range lines {
		newLines = append(newLines, line)

		if strings.Contains(line, "requires:") {
			inRequires = true
			// Get the indent of the next line
			if i+1 < len(lines) && strings.HasPrefix(lines[i+1], "- path:") {
				requiresIndent = strings.Split(lines[i+1], "- path:")[0]
			} else {
				requiresIndent = "- " // default
			}
			continue
		}

		if inRequires && !inserted {
			// Check if we're still in requires section
			if strings.TrimSpace(line) == "" || (!strings.HasPrefix(strings.TrimLeft(line, " "), "-") && strings.TrimSpace(line) != "") {
				// End of requires section, insert before this line
				newLines = newLines[:len(newLines)-1] // remove last line
				newLines = append(newLines, requiresIndent+"path: "+servicePath)
				newLines = append(newLines, line) // add back the line
				inserted = true
				inRequires = false
			} else if i == len(lines)-1 {
				// Last line and still in requires
				newLines = append(newLines, requiresIndent+"path: "+servicePath)
				inserted = true
			}
		}
	}

	// If we reached end without inserting and requires was found
	if inRequires && !inserted {
		newLines = append(newLines, requiresIndent+"path: "+servicePath)
	}

	// Write back
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(skaffoldPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write skaffold.yaml: %w", err)
	}

	return nil
}
