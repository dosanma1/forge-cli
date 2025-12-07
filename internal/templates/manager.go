// Package templates provides template fetching and management for forge-cli.
package templates

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	// ForgeRepoURL is the GitHub repository URL for forge templates
	ForgeRepoURL = "https://github.com/dosanma1/forge.git"

	// CacheTTL is the time to live for cached templates (7 days)
	CacheTTL = 7 * 24 * time.Hour
)

// Manager handles template fetching and caching.
type Manager struct {
	cacheDir      string
	workspaceRoot string
}

// NewManager creates a new template manager.
// If workspaceRoot is provided, cache is stored in <workspace>/.forge/cache/templates
// Otherwise falls back to ~/.forge/cache/templates for compatibility
func NewManager(workspaceRoot string) (*Manager, error) {
	var cacheDir string

	if workspaceRoot != "" {
		// Project-local cache (like Angular's .angular/cache)
		cacheDir = filepath.Join(workspaceRoot, ".forge", "cache", "templates")
	} else {
		// Global cache fallback
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = filepath.Join(homeDir, ".forge", "cache", "templates")
	}

	return &Manager{
		cacheDir:      cacheDir,
		workspaceRoot: workspaceRoot,
	}, nil
}

// FetchTemplates fetches templates for the specified forge version.
// It uses a local cache and only downloads if cache is missing or stale.
func (m *Manager) FetchTemplates(forgeVersion string) (string, error) {
	versionCacheDir := filepath.Join(m.cacheDir, forgeVersion)
	timestampFile := filepath.Join(versionCacheDir, ".timestamp")

	// Check if cache exists and is fresh
	if m.isCacheFresh(timestampFile) {
		return filepath.Join(versionCacheDir, "templates"), nil
	}

	// Create cache directory
	if err := os.MkdirAll(versionCacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Clone or pull forge repository
	if err := m.downloadTemplates(forgeVersion, versionCacheDir); err != nil {
		return "", fmt.Errorf("failed to download templates: %w", err)
	}

	// Update timestamp
	if err := m.updateTimestamp(timestampFile); err != nil {
		return "", fmt.Errorf("failed to update timestamp: %w", err)
	}

	return filepath.Join(versionCacheDir, "templates"), nil
}

// GetTemplatePath returns the path to a specific template.
func (m *Manager) GetTemplatePath(forgeVersion, projectType, target string) (string, error) {
	templatesDir, err := m.FetchTemplates(forgeVersion)
	if err != nil {
		return "", err
	}

	templatePath := filepath.Join(templatesDir, target, projectType)

	// Verify template exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return "", fmt.Errorf("template not found: %s/%s (forge version: %s)", target, projectType, forgeVersion)
	}

	return templatePath, nil
}

// ClearCache removes all cached templates.
func (m *Manager) ClearCache() error {
	return os.RemoveAll(m.cacheDir)
}

// ClearVersion removes cached templates for a specific version.
func (m *Manager) ClearVersion(forgeVersion string) error {
	versionCacheDir := filepath.Join(m.cacheDir, forgeVersion)
	return os.RemoveAll(versionCacheDir)
}

// isCacheFresh checks if the cached templates are still fresh.
func (m *Manager) isCacheFresh(timestampFile string) bool {
	info, err := os.Stat(timestampFile)
	if err != nil {
		return false
	}

	age := time.Since(info.ModTime())
	return age < CacheTTL
}

// downloadTemplates downloads forge templates using git clone.
func (m *Manager) downloadTemplates(forgeVersion, targetDir string) error {
	// Check if directory exists and remove it
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Git clone with specific version tag
	tag := fmt.Sprintf("v%s", forgeVersion)
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", tag, ForgeRepoURL, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Try without 'v' prefix
		cmd = exec.Command("git", "clone", "--depth", "1", "--branch", forgeVersion, ForgeRepoURL, targetDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone forge repository (tried tags: %s, %s): %w", tag, forgeVersion, err)
		}
	}

	return nil
}

// updateTimestamp creates or updates the timestamp file.
func (m *Manager) updateTimestamp(timestampFile string) error {
	return os.WriteFile(timestampFile, []byte(time.Now().Format(time.RFC3339)), 0644)
}
