package template

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	forgeRepoURL     = "https://raw.githubusercontent.com/dosanma1/forge"
	templateCacheDir = ".forge/templates"
)

// FetchTemplates fetches templates from forge repo based on version
// Resolution order: cache → remote fetch → embedded fallback
func FetchTemplates(forgeVersion string) (string, error) {
	// Get cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, templateCacheDir, forgeVersion)

	// Check if cached version exists
	if _, err := os.Stat(cacheDir); err == nil {
		return cacheDir, nil
	}

	// Try to fetch from remote
	if err := fetchRemoteTemplates(forgeVersion, cacheDir); err != nil {
		// Fall back to embedded templates
		return "", fmt.Errorf("failed to fetch templates, using embedded fallback: %w", err)
	}

	return cacheDir, nil
}

// ClearTemplateCache clears the template cache for a specific version or all versions
func ClearTemplateCache(forgeVersion string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	if forgeVersion == "" {
		// Clear all cache
		cacheDir := filepath.Join(homeDir, templateCacheDir)
		if err := os.RemoveAll(cacheDir); err != nil {
			return fmt.Errorf("failed to clear template cache: %w", err)
		}
		return nil
	}

	// Clear specific version
	versionCacheDir := filepath.Join(homeDir, templateCacheDir, forgeVersion)
	if err := os.RemoveAll(versionCacheDir); err != nil {
		return fmt.Errorf("failed to clear template cache for version %s: %w", forgeVersion, err)
	}

	return nil
}

// fetchRemoteTemplates downloads templates from forge repo
func fetchRemoteTemplates(version string, destDir string) error {
	// Create cache directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Template files to fetch
	templates := []string{
		"infra/kind-config.yaml",
		"infra/helm/go-service/Chart.yaml",
		"infra/helm/go-service/values.yaml",
		"infra/helm/go-service/templates/_helpers.tpl",
		"infra/helm/go-service/templates/deployment.yaml",
		"infra/helm/go-service/templates/service.yaml",
		"infra/helm/go-service/templates/serviceaccount.yaml",
		"infra/helm/go-service/templates/configmap.yaml",
		"infra/helm/go-service/templates/secret.yaml",
		"infra/helm/go-service/templates/hpa.yaml",
		"infra/helm/go-service/templates/pdb.yaml",
		"infra/helm/go-service/templates/ingress.yaml",
		"infra/helm/go-service/templates/NOTES.txt",
		"infra/helm/nestjs-service/Chart.yaml",
		"infra/helm/nestjs-service/values.yaml",
		"infra/helm/nestjs-service/templates/_helpers.tpl",
		"infra/helm/nestjs-service/templates/deployment.yaml",
		"infra/helm/nestjs-service/templates/service.yaml",
		"infra/helm/nestjs-service/templates/serviceaccount.yaml",
		"infra/helm/nestjs-service/templates/configmap.yaml",
		"infra/helm/nestjs-service/templates/secret.yaml",
		"infra/helm/nestjs-service/templates/hpa.yaml",
		"infra/helm/nestjs-service/templates/pdb.yaml",
		"infra/helm/nestjs-service/templates/ingress.yaml",
		"infra/helm/nestjs-service/templates/NOTES.txt",
		"infra/cloudrun/go-service/service.yaml",
		"infra/cloudrun/nestjs-service/service.yaml",
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for _, template := range templates {
		url := fmt.Sprintf("%s/v%s/templates/%s", forgeRepoURL, version, template)
		destPath := filepath.Join(destDir, template)

		if err := downloadFile(client, url, destPath); err != nil {
			return fmt.Errorf("failed to download %s: %w", template, err)
		}
	}

	return nil
}

// downloadFile downloads a file from URL to destination
func downloadFile(client *http.Client, url, destPath string) error {
	// Create parent directory
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download file
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch (status %d)", resp.StatusCode)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy content
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetTemplateSource returns the template source directory based on forgeVersion
// Falls back to embedded templates if fetching fails
func GetTemplateSource(forgeVersion string) string {
	if forgeVersion == "" {
		return "" // Use embedded
	}

	// Normalize version (remove 'v' prefix if present)
	version := strings.TrimPrefix(forgeVersion, "v")

	// Try to get cached or fetch templates
	templateDir, err := FetchTemplates(version)
	if err != nil {
		// Fall back to embedded templates
		return ""
	}

	return templateDir
}
