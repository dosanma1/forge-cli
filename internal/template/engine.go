// Package template provides template rendering functionality.
package template

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
)

//go:embed all:templates
var templatesFS embed.FS

// Engine provides template rendering capabilities.
type Engine struct {
	funcMap template.FuncMap
}

// NewEngine creates a new template engine.
func NewEngine() *Engine {
	return &Engine{
		funcMap: template.FuncMap{
			"dasherize":  Dasherize,
			"camelize":   Camelize,
			"pascalize":  Pascalize,
			"underscore": Underscore,
			"kebabCase":  KebabCase,
			"snakeCase":  SnakeCase,
			"pluralize":  Pluralize,
			"upper":      strings.ToUpper,
			"lower":      strings.ToLower,
			"title":      strings.Title,
			"replace":    strings.ReplaceAll,
		},
	}
}

// Render renders a template string with the given data.
func (e *Engine) Render(templateStr string, data interface{}) (string, error) {
	tmpl, err := template.New("template").Funcs(e.funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderFile renders a template file with the given data.
func (e *Engine) RenderFile(path string, data interface{}) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	return e.Render(string(content), data)
}

// RenderTemplate renders an embedded template file with the given data.
func (e *Engine) RenderTemplate(templatePath string, data interface{}) (string, error) {
	// Read from embedded filesystem
	content, err := templatesFS.ReadFile("templates/" + templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded template %s: %w", templatePath, err)
	}

	return e.Render(string(content), data)
}

// ReadEmbeddedFile reads an embedded file without template rendering
func (e *Engine) ReadEmbeddedFile(templatePath string) ([]byte, error) {
	// Read from embedded filesystem
	content, err := templatesFS.ReadFile("templates/" + templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded file %s: %w", templatePath, err)
	}

	return content, nil
}

// RenderToFile renders a template and writes the result to a file.
func (e *Engine) RenderToFile(templateStr string, data interface{}, outputPath string) error {
	result, err := e.Render(templateStr, data)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// RenderToWriter renders a template and writes the result to a writer.
func (e *Engine) RenderToWriter(templateStr string, data interface{}, w *bytes.Buffer) error {
	tmpl, err := template.New("template").Funcs(e.funcMap).Parse(templateStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// Helper functions for string transformations

// Dasherize converts a string to dash-case (kebab-case).
func Dasherize(s string) string {
	return strings.ToLower(KebabCase(s))
}

// Camelize converts a string to camelCase.
func Camelize(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}

	result := strings.ToLower(words[0])
	for i := 1; i < len(words); i++ {
		result += strings.Title(strings.ToLower(words[i]))
	}

	return result
}

// Pascalize converts a string to PascalCase.
func Pascalize(s string) string {
	words := splitWords(s)
	for i := range words {
		words[i] = strings.Title(strings.ToLower(words[i]))
	}
	return strings.Join(words, "")
}

// Underscore converts a string to snake_case.
func Underscore(s string) string {
	return SnakeCase(s)
}

// KebabCase converts a string to kebab-case.
func KebabCase(s string) string {
	words := splitWords(s)
	return strings.Join(words, "-")
}

// SnakeCase converts a string to snake_case.
func SnakeCase(s string) string {
	words := splitWords(s)
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "_")
}

// Pluralize returns a simple plural form of a word.
func Pluralize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Simple English pluralization rules
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "z") || strings.HasSuffix(s, "ch") ||
		strings.HasSuffix(s, "sh") {
		return s + "es"
	}

	if strings.HasSuffix(s, "y") && len(s) > 1 {
		if !isVowel(rune(s[len(s)-2])) {
			return s[:len(s)-1] + "ies"
		}
	}

	return s + "s"
}

// splitWords splits a string into words for transformation.
func splitWords(s string) []string {
	// Handle kebab-case and snake_case
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")

	// Handle PascalCase and camelCase
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	s = re.ReplaceAllString(s, "${1} ${2}")

	// Split on spaces and filter empty strings
	words := strings.Fields(s)
	return words
}

// isVowel checks if a rune is a vowel.
func isVowel(r rune) bool {
	switch strings.ToLower(string(r)) {
	case "a", "e", "i", "o", "u":
		return true
	default:
		return false
	}
}
