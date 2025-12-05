package bazel

import (
	"fmt"
	"regexp"
	"strings"
)

// ErrorTranslator converts Bazel errors to user-friendly messages.
type ErrorTranslator struct{}

// NewErrorTranslator creates a new error translator.
func NewErrorTranslator() *ErrorTranslator {
	return &ErrorTranslator{}
}

// Translate converts a Bazel error to a user-friendly message.
func (t *ErrorTranslator) Translate(bazelError string) string {
	// Parse common Bazel error patterns and translate to user terms

	// Missing dependency error
	if strings.Contains(bazelError, "no such package") {
		return t.translateMissingPackage(bazelError)
	}

	// Compilation error
	if strings.Contains(bazelError, "compilation failed") {
		return t.translateCompilationError(bazelError)
	}

	// Test failure
	if strings.Contains(bazelError, "FAILED") && strings.Contains(bazelError, "test") {
		return t.translateTestFailure(bazelError)
	}

	// Build file error
	if strings.Contains(bazelError, "BUILD") {
		return t.translateBuildFileError(bazelError)
	}

	// Default: return cleaned error
	return t.cleanError(bazelError)
}

// translateMissingPackage converts missing package errors.
func (t *ErrorTranslator) translateMissingPackage(err string) string {
	// Extract service name from path like //backend/services/api-server
	re := regexp.MustCompile(`//backend/services/([^/]+)`)
	if matches := re.FindStringSubmatch(err); len(matches) > 1 {
		return fmt.Sprintf("Service '%s' not found. Did you forget to run 'forge generate'?", matches[1])
	}
	return "Service not found. Run 'forge generate' to update build files."
}

// translateCompilationError converts compilation errors.
func (t *ErrorTranslator) translateCompilationError(err string) string {
	// Extract service name and file
	re := regexp.MustCompile(`backend/services/([^/]+)/.*?([^/]+\.go)`)
	if matches := re.FindStringSubmatch(err); len(matches) > 2 {
		return fmt.Sprintf("Build failed in service '%s' (file: %s)\n%s",
			matches[1], matches[2], t.extractErrorDetail(err))
	}
	return fmt.Sprintf("Build failed:\n%s", t.extractErrorDetail(err))
}

// translateTestFailure converts test failures.
func (t *ErrorTranslator) translateTestFailure(err string) string {
	re := regexp.MustCompile(`//backend/services/([^/]+)`)
	if matches := re.FindStringSubmatch(err); len(matches) > 1 {
		return fmt.Sprintf("Tests failed in service '%s'\nRun 'forge test %s --verbose' for details",
			matches[1], matches[1])
	}
	return "Tests failed. Run 'forge test --verbose' for details."
}

// translateBuildFileError converts BUILD file errors.
func (t *ErrorTranslator) translateBuildFileError(err string) string {
	return "Build configuration error. Run 'forge generate' to fix BUILD files."
}

// extractErrorDetail extracts the most relevant error details.
func (t *ErrorTranslator) extractErrorDetail(err string) string {
	lines := strings.Split(err, "\n")
	var relevant []string
	for _, line := range lines {
		// Skip Bazel internal lines
		if strings.Contains(line, "ERROR:") ||
			strings.Contains(line, "FAILED:") ||
			(!strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "") {
			relevant = append(relevant, strings.TrimSpace(line))
		}
	}
	if len(relevant) > 5 {
		relevant = relevant[:5]
		relevant = append(relevant, "... (run with --verbose for full output)")
	}
	return strings.Join(relevant, "\n")
}

// cleanError removes Bazel-specific jargon.
func (t *ErrorTranslator) cleanError(err string) string {
	// Remove Bazel target syntax
	cleaned := regexp.MustCompile(`//[^:\s]+:[^\s]+`).ReplaceAllString(err, "[target]")

	// Remove Bazel command lines
	cleaned = regexp.MustCompile(`bazel-bin/.*`).ReplaceAllString(cleaned, "[build output]")

	return cleaned
}
