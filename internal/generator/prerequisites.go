package generator

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// PrerequisiteError represents a missing or incompatible prerequisite.
type PrerequisiteError struct {
	Tool           string
	MinVersion     string
	CurrentVersion string
	Message        string
}

func (e *PrerequisiteError) Error() string {
	return e.Message
}

// CheckNodeJS validates that Node.js is installed and meets minimum version requirements.
// Returns nil if valid, PrerequisiteError with detailed instructions if invalid.
func CheckNodeJS() error {
	// Check if node command exists
	cmd := exec.Command("node", "--version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &PrerequisiteError{
			Tool:    "Node.js",
			Message: formatNodeJSMissingError(),
		}
	}

	// Parse version
	version := strings.TrimSpace(string(output))
	version = strings.TrimPrefix(version, "v") // Remove 'v' prefix (e.g., v20.0.0 -> 20.0.0)

	majorVersion, err := parseNodeVersion(version)
	if err != nil {
		return &PrerequisiteError{
			Tool:           "Node.js",
			CurrentVersion: version,
			Message: fmt.Sprintf(`Failed to parse Node.js version: %s

Please ensure Node.js is properly installed.
Run: node --version

Installation: %s
`, version, getNodeInstallInstructions()),
		}
	}

	// Require Node.js 16+, warn if <18
	if majorVersion < 16 {
		return &PrerequisiteError{
			Tool:           "Node.js",
			MinVersion:     "16.0.0",
			CurrentVersion: version,
			Message: fmt.Sprintf(`Incompatible Node.js version

The generator requires Node.js 16.0.0 or later.
Current version: %s

Please upgrade Node.js: %s
`, version, getNodeInstallInstructions()),
		}
	}

	// Warn if <18 but continue
	if majorVersion < 18 {
		fmt.Printf("⚠️  Warning: Node.js %s detected. Node.js 18+ is recommended for best compatibility.\n", version)
	}

	return nil
}

// CheckNPM validates that npm and npx are available.
func CheckNPM() error {
	// Check for npx (included with npm 5.2.0+)
	if _, err := exec.LookPath("npx"); err != nil {
		return &PrerequisiteError{
			Tool:    "npx",
			Message: formatNPXMissingError(),
		}
	}

	return nil
}

// parseNodeVersion extracts the major version number from a version string.
func parseNodeVersion(version string) (int, error) {
	// Handle versions like "20.0.0" or "18.19.0"
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid version format: %s", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid major version: %s", parts[0])
	}

	return major, nil
}

// formatNodeJSMissingError returns a platform-specific error message for missing Node.js.
func formatNodeJSMissingError() string {
	return fmt.Sprintf(`Node.js is required but not found

The generator requires Node.js 16.0.0 or later to be installed.

%s

After installation, verify with: node --version
`, getNodeInstallInstructions())
}

// formatNPXMissingError returns a platform-specific error message for missing npx.
func formatNPXMissingError() string {
	return fmt.Sprintf(`npx is required but not found

npx is included with npm 5.2.0+. Please ensure Node.js and npm are properly installed.

%s

To verify npm installation: npm --version
To upgrade npm: npm install -g npm@latest

After installation, verify with: npx --version
`, getNodeInstallInstructions())
}

// getNodeInstallInstructions returns platform-specific Node.js installation instructions.
func getNodeInstallInstructions() string {
	switch runtime.GOOS {
	case "darwin":
		return `To install Node.js on macOS:
  • Homebrew:  brew install node
  • Download:  https://nodejs.org/en/download/`
	case "linux":
		return `To install Node.js on Linux:
  • Ubuntu/Debian:  sudo apt update && sudo apt install nodejs npm
  • Fedora:         sudo dnf install nodejs npm
  • Arch:           sudo pacman -S nodejs npm
  • Using nvm:      https://github.com/nvm-sh/nvm
  • Download:       https://nodejs.org/en/download/package-manager`
	case "windows":
		return `To install Node.js on Windows:
  • Chocolatey:  choco install nodejs
  • Winget:      winget install OpenJS.NodeJS
  • Download:    https://nodejs.org/en/download/`
	default:
		return `To install Node.js:
  • Download: https://nodejs.org/en/download/`
	}
}
