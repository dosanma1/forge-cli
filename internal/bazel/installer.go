package bazel

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/schollz/progressbar/v3"
)

const (
	bazeliskVersion = "1.20.0"
	bazeliskBaseURL = "https://github.com/bazelbuild/bazelisk/releases/download"
)

// Installer handles Bazel/Bazelisk installation.
type Installer struct {
	forgeHome string
	verbose   bool
}

// NewInstaller creates a new Bazel installer.
func NewInstaller(verbose bool) *Installer {
	forgeHome := filepath.Join(os.Getenv("HOME"), ".forge")
	return &Installer{
		forgeHome: forgeHome,
		verbose:   verbose,
	}
}

// Install downloads and installs Bazelisk.
func (i *Installer) Install(ctx context.Context) error {
	bazelDir := filepath.Join(i.forgeHome, "bazel", "bin")
	if err := os.MkdirAll(bazelDir, 0755); err != nil {
		return fmt.Errorf("failed to create bazel directory: %w", err)
	}

	// Determine download URL based on OS and architecture
	downloadURL, _ := i.getBazeliskURL()
	targetPath := filepath.Join(bazelDir, "bazelisk")
	fmt.Printf("📦 Downloading Bazelisk %s...\n", bazeliskVersion)

	// Download with progress bar
	if err := i.downloadWithProgress(ctx, downloadURL, targetPath); err != nil {
		return fmt.Errorf("failed to download bazelisk: %w", err)
	}

	// Make executable
	if err := os.Chmod(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to make bazelisk executable: %w", err)
	}

	fmt.Println("✅ Bazelisk installed successfully!")
	fmt.Printf("   Location: %s\n", targetPath)
	fmt.Println("   Bazel will be automatically downloaded on first use.")
	return nil
}

// IsInstalled checks if Bazel is available.
func (i *Installer) IsInstalled() bool {
	_, err := findBazel()
	return err == nil
}

// getBazeliskURL returns the download URL for the current platform.
func (i *Installer) getBazeliskURL() (string, string) {
	var filename string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			filename = "bazelisk-darwin-arm64"
		} else {
			filename = "bazelisk-darwin-amd64"
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			filename = "bazelisk-linux-arm64"
		} else {
			filename = "bazelisk-linux-amd64"
		}
	case "windows":
		filename = "bazelisk-windows-amd64.exe"
	default:
		filename = "bazelisk-linux-amd64" // Default fallback
	}
	url := fmt.Sprintf("%s/v%s/%s", bazeliskBaseURL, bazeliskVersion, filename)
	return url, filename
}

// downloadWithProgress downloads a file with a progress bar.
func (i *Installer) downloadWithProgress(ctx context.Context, url, targetPath string) error {
	// This is a placeholder - in real implementation, use HTTP client
	// For now, using curl/wget as a quick solution

	// Create progress bar
	_ = progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*1000000), // 65ms
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	// TODO: Implement actual HTTP download with progress
	// For now, this is a simplified version
	return fmt.Errorf("download not yet implemented - please install bazelisk manually")
}

// GetVersion returns the installed Bazel version.
func (i *Installer) GetVersion(ctx context.Context) (string, error) {
	executor, err := NewExecutor(i.forgeHome, i.verbose)
	if err != nil {
		return "", err
	}
	return executor.Version(ctx)
}

// Update checks for and installs Bazel updates.
func (i *Installer) Update(ctx context.Context) error {
	fmt.Println("🔄 Checking for Bazel updates...")

	// Check current version
	currentVersion, err := i.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}
	fmt.Printf("   Current version: %s\n", currentVersion)

	// For bazelisk, updates are automatic - it downloads the version specified in .bazelversion
	fmt.Println("   Bazelisk will automatically use the version specified in your workspace")
	return nil
}

// Uninstall removes forge-managed Bazel installation.
func (i *Installer) Uninstall() error {
	bazelDir := filepath.Join(i.forgeHome, "bazel")
	if _, err := os.Stat(bazelDir); os.IsNotExist(err) {
		return fmt.Errorf("bazel installation not found")
	}
	if err := os.RemoveAll(bazelDir); err != nil {
		return fmt.Errorf("failed to remove bazel: %w", err)
	}
	fmt.Println("✅ Bazel uninstalled successfully")
	return nil
}

// downloadFile is a helper to download files (placeholder).
func downloadFile(url string, dest string) error {
	// TODO: Implement HTTP download
	return fmt.Errorf("not implemented")
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
