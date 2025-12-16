package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/generator"
	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/internal/ui"
	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:     "generate [type] [name]",
	Aliases: []string{"g"},
	Short:   "Generate code from schematics",
	Long: `Generate application components using Forge generators.

Available types:
  service     Generate a new microservice (Go, NestJS)
  app         Generate a new application (Angular, React)
  library     Generate a shared library

Examples:
  forge generate service user-service --lang=go
  forge generate service api-gateway --lang=nestjs
  forge g service payment-service
  forge generate app admin-portal --lang=angular
  forge g app web-app
  forge g library shared/auth`,
}

var (
	serviceLanguage string
	appLanguage     string
)

var generateServiceCmd = &cobra.Command{
	Use:   "service [name]",
	Short: "Generate a new microservice",
	Long: `Generate a new microservice with Forge patterns.

Supports multiple languages:
- Go: Standard Go microservice with HTTP server
- NestJS: TypeScript microservice with NestJS framework

The service will include:
- Main application with HTTP server
- Logging and observability setup
- Health check endpoint
- Example API route
- Dockerfile for containerization
- Deployment configurations
- README with documentation

Examples:
  forge generate service user-service --lang=go
  forge generate service api-gateway --lang=nestjs
  forge g service payment-service`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGenerateService,
}

var generateAppCmd = &cobra.Command{
	Use:   "app [name]",
	Short: "Generate a new application",
	Long: `Generate a new frontend application with Forge patterns.

Supports multiple frameworks:
- Angular: Standalone Angular application with Tailwind CSS
- React: React application (coming soon)

The application will include:
- Framework-specific configuration
- Tailwind CSS setup
- TypeScript configuration
- Package.json with dependencies
- Deployment configurations

Examples:
  forge generate app web-app --lang=angular
  forge generate app admin-portal --lang=angular
  forge g app dashboard`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGenerateApp,
}

var generateLibraryCmd = &cobra.Command{
	Use:   "library <path>",
	Short: "Generate a shared library",
	Long: `Generate a shared library at the specified path.

Examples:
  forge g library shared/auth
  forge g library shared/utils/logging`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerateLibrary,
}

func init() {
	generateServiceCmd.Flags().StringVarP(&serviceLanguage, "lang", "l", "", "Service language (go, nestjs)")
	generateAppCmd.Flags().StringVarP(&appLanguage, "lang", "l", "", "Application language (angular, react)")

	generateCmd.AddCommand(generateServiceCmd)
	generateCmd.AddCommand(generateAppCmd)
	generateCmd.AddCommand(generateLibraryCmd)

	// Keep legacy commands for backward compatibility
	generateCmd.AddCommand(generateNestJSCmd)
	generateCmd.AddCommand(generateFrontendCmd)
	generateNestJSCmd.Hidden = true
	generateFrontendCmd.Hidden = true
}

var generateNestJSCmd = &cobra.Command{
	Use:   "nestjs <name>",
	Short: "Generate a new NestJS microservice",
	Long: `Generate a new NestJS microservice with Forge patterns.

This will create:
- NestJS application structure
- Health check endpoint
- Dockerfile for containerization
- Helm deployment values
- Cloud Run configuration
- TypeScript configuration
- Package.json with dependencies

Examples:
  forge generate nestjs api-gateway
  forge g nestjs payment-service`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerateNestJS,
}

func runGenerateNestJS(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Create generator
	gen := generator.NewNestJSServiceGenerator()

	// Prepare options
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      serviceName,
		DryRun:    false,
	}

	// Generate service
	ctx := context.Background()
	if err := gen.Generate(ctx, opts); err != nil {
		return fmt.Errorf("failed to generate NestJS service: %w", err)
	}

	return nil
}

var generateFrontendCmd = &cobra.Command{
	Use:   "frontend <name>",
	Short: "Generate a new Angular frontend application",
	Long: `Generate a new Angular frontend application with Forge patterns.

This will create:
- Angular workspace configuration (first app only)
- Standalone Angular application
- Tailwind CSS configuration
- TypeScript configuration
- Package.json with dependencies

Examples:
  forge generate frontend web-app
  forge g frontend admin-portal`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerateFrontend,
}

func runGenerateFrontend(cmd *cobra.Command, args []string) error {
	appName := args[0]

	// Create generator
	gen := generator.NewFrontendGenerator()

	// Prepare options
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      appName,
		DryRun:    false,
	}

	// Generate frontend
	ctx := context.Background()
	if err := gen.Generate(ctx, opts); err != nil {
		return fmt.Errorf("failed to generate frontend: %w", err)
	}

	return nil
}

func runGenerateService(cmd *cobra.Command, args []string) error {
	var serviceName string

	// Prompt for name if not provided
	if len(args) == 0 {
		name, err := ui.AskText("Service name:", "")
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		serviceName = name
	} else {
		serviceName = args[0]
	}

	// Prompt for language if not provided
	if serviceLanguage == "" {
		_, lang, err := ui.AskSelect("Select service language:", []string{"Go", "NestJS"})
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		serviceLanguage = strings.ToLower(lang)
	}

	// Normalize language
	serviceLanguage = strings.ToLower(serviceLanguage)

	// Prompt for deployer selection
	_, deployerChoice, err := ui.AskSelect("Select deployment target:", []string{"Helm (Kubernetes)", "CloudRun"})
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}

	// Map display names to internal names
	var deployer string
	switch deployerChoice {
	case "Helm (Kubernetes)":
		deployer = "helm"
	case "CloudRun":
		deployer = "cloudrun"
	default:
		deployer = "helm"
	}

	// Create appropriate generator
	var gen generator.Generator
	switch serviceLanguage {
	case "go":
		gen = generator.NewServiceGenerator()
	case "nestjs":
		gen = generator.NewNestJSServiceGenerator()
	default:
		return fmt.Errorf("unsupported service language: %s (supported: go, nestjs)", serviceLanguage)
	}

	// Prepare options with deployer data
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      serviceName,
		DryRun:    false,
		Data: map[string]interface{}{
			"deployer": deployer,
		},
	}

	// Generate service
	ctx := context.Background()
	if err := gen.Generate(ctx, opts); err != nil {
		return fmt.Errorf("failed to generate %s service: %w", serviceLanguage, err)
	}

	return nil
}

func runGenerateApp(cmd *cobra.Command, args []string) error {
	var appName string

	// Prompt for name if not provided
	if len(args) == 0 {
		name, err := ui.AskText("Application name:", "")
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		appName = name
	} else {
		appName = args[0]
	}

	// Prompt for language if not provided
	if appLanguage == "" {
		_, lang, err := ui.AskSelect("Select application framework:", []string{"Angular", "React"})
		if err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		appLanguage = strings.ToLower(lang)
	}

	// Normalize language
	appLanguage = strings.ToLower(appLanguage)

	// Prompt for deployer selection
	_, deployerChoice, err := ui.AskSelect("Select deployment target:", []string{"Firebase", "Helm (Kubernetes)", "CloudRun"})
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}

	// Map display names to internal names
	var deployer string
	switch deployerChoice {
	case "Firebase":
		deployer = "firebase"
	case "Helm (Kubernetes)":
		deployer = "helm"
	case "CloudRun":
		deployer = "cloudrun"
	default:
		deployer = "firebase"
	}

	// Create appropriate generator
	var gen generator.Generator
	switch appLanguage {
	case "angular":
		gen = generator.NewFrontendGenerator()
	case "react":
		return fmt.Errorf("React support coming soon")
	default:
		return fmt.Errorf("unsupported app framework: %s (supported: angular, react)", appLanguage)
	}

	// Prepare options with deployer data
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      appName,
		DryRun:    false,
		Data: map[string]interface{}{
			"deployer": deployer,
		},
	}

	// Generate app
	ctx := context.Background()
	if err := gen.Generate(ctx, opts); err != nil {
		return fmt.Errorf("failed to generate %s app: %w", appLanguage, err)
	}

	return nil
}

func runGenerateLibrary(cmd *cobra.Command, args []string) error {
	libPath := args[0]

	// Determine library type
	_, libType, err := ui.AskSelect("Select library type:", []string{"Go", "TypeScript"})
	if err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}

	absPath, err := filepath.Abs(libPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); !os.IsNotExist(err) {
		return fmt.Errorf("path already exists: %s", libPath)
	}

	fmt.Printf("CREATE %s (%s)\n", libPath, libType)

	switch libType {
	case "Go":
		if err := generateGoLibrary(absPath); err != nil {
			return err
		}
	case "TypeScript":
		if err := generateTypeScriptLibrary(absPath); err != nil {
			return err
		}
	}

	fmt.Println("✔ Library created successfully.")
	return nil
}

func generateGoLibrary(path string) error {
	// Create directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Get module path from user
	modulePath, err := ui.AskText("Go module path (e.g., github.com/org/lib):", "")
	if err != nil {
		return err
	}

	// Create go.mod
	goModContent := fmt.Sprintf(`module %s

go 1.23

require (
)
`, modulePath)

	goModPath := filepath.Join(path, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	// Create README.md
	libName := filepath.Base(path)
	readmeContent := fmt.Sprintf(`# %s

Shared library for %s

## Installation

`+"```"+`bash
go get %s
`+"```"+`

## Usage

`+"```"+`go
import "%s"
`+"```"+`
`, libName, libName, modulePath, modulePath)

	readmePath := filepath.Join(path, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	// Create basic .go file
	packageName := strings.ReplaceAll(filepath.Base(path), "-", "")
	mainGoContent := fmt.Sprintf(`package %s

// Add your library code here
`, packageName)

	mainGoPath := filepath.Join(path, packageName+".go")
	if err := os.WriteFile(mainGoPath, []byte(mainGoContent), 0644); err != nil {
		return fmt.Errorf("failed to create .go file: %w", err)
	}

	// Generate BUILD.bazel from template
	if err := generateLibraryBuildFile(path, modulePath, packageName); err != nil {
		return fmt.Errorf("failed to generate BUILD.bazel: %w", err)
	}

	// Register library in forge.json
	if err := registerLibraryInForgeConfig(path, modulePath); err != nil {
		return fmt.Errorf("failed to register library: %w", err)
	}

	// Try to add to go.work if it exists
	workspacePath, err := findGoWorkspace()
	if err == nil {
		if err := addToGoWorkspace(workspacePath, path); err != nil {
			fmt.Printf("⚠️  Could not add to go.work: %v\n", err)
		} else {
			fmt.Println("✔ Added to go.work")
		}
	}

	return nil
}

func generateTypeScriptLibrary(path string) error {
	// Create directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Get package name
	defaultName := filepath.Base(path)
	packageName, err := ui.AskText("Package name:", defaultName)
	if err != nil {
		return err
	}

	// Create package.json
	packageJSON := fmt.Sprintf(`{
  "name": "@shared/%s",
  "version": "0.0.1",
  "type": "module",
  "main": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "scripts": {
    "build": "tsc",
    "watch": "tsc --watch"
  },
  "devDependencies": {
    "typescript": "^5.7.2"
  }
}
`, packageName)

	packageJSONPath := filepath.Join(path, "package.json")
	if err := os.WriteFile(packageJSONPath, []byte(packageJSON), 0644); err != nil {
		return fmt.Errorf("failed to create package.json: %w", err)
	}

	// Create tsconfig.json
	tsconfigContent := `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ES2022",
    "moduleResolution": "bundler",
    "declaration": true,
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "skipLibCheck": true,
    "esModuleInterop": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
`

	tsconfigPath := filepath.Join(path, "tsconfig.json")
	if err := os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644); err != nil {
		return fmt.Errorf("failed to create tsconfig.json: %w", err)
	}

	// Create src directory and index.ts
	srcPath := filepath.Join(path, "src")
	if err := os.MkdirAll(srcPath, 0755); err != nil {
		return fmt.Errorf("failed to create src directory: %w", err)
	}

	indexContent := `export const version = '0.0.1';

// Add your library code here
`

	indexPath := filepath.Join(srcPath, "index.ts")
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		return fmt.Errorf("failed to create index.ts: %w", err)
	}

	// Create README.md
	readmeContent := fmt.Sprintf(`# %s

Shared TypeScript library

## Installation

`+"```"+`bash
npm install @shared/%s
`+"```"+`

## Usage

`+"```"+`typescript
import { version } from '@shared/%s';
`+"```"+`
`, packageName, packageName, packageName)

	readmePath := filepath.Join(path, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	return nil
}

func findGoWorkspace() (string, error) {
	// Look for go.work in current directory and parents
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		workPath := filepath.Join(dir, "go.work")
		if _, err := os.Stat(workPath); err == nil {
			return workPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.work not found")
		}
		dir = parent
	}
}

func addToGoWorkspace(workspacePath, modulePath string) error {
	// Read existing go.work
	content, err := os.ReadFile(workspacePath)
	if err != nil {
		return err
	}

	workContent := string(content)
	relPath, err := filepath.Rel(filepath.Dir(workspacePath), modulePath)
	if err != nil {
		return err
	}

	// Check if already in workspace
	if strings.Contains(workContent, relPath) {
		return nil
	}

	// Add new use directive
	lines := strings.Split(workContent, "\n")
	var newLines []string
	inUseBlock := false
	added := false

	for i, line := range lines {
		newLines = append(newLines, line)

		// Check if we're entering a use block
		if strings.HasPrefix(strings.TrimSpace(line), "use (") {
			inUseBlock = true
			continue
		}

		// Check for inline use directive
		if !inUseBlock && strings.HasPrefix(strings.TrimSpace(line), "use ") {
			// Insert after the last standalone use directive
			if i+1 < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i+1]), "use ") {
				newLines = append(newLines, fmt.Sprintf("\nuse ./%s", relPath))
				added = true
			}
			continue
		}

		// If we're in a use block and hit the closing paren, add before it
		if inUseBlock && strings.TrimSpace(line) == ")" {
			// Insert before the closing paren
			newLines = newLines[:len(newLines)-1] // Remove the ")" we just added
			newLines = append(newLines, "")
			newLines = append(newLines, fmt.Sprintf("use ./%s", relPath))
			newLines = append(newLines, ")")
			added = true
			inUseBlock = false
		}
	}

	// If not added and no use block exists, create one
	if !added {
		newLines = append(newLines, "")
		newLines = append(newLines, "use (")
		newLines = append(newLines, fmt.Sprintf("\t./%s", relPath))
		newLines = append(newLines, ")")
	}

	workContent = strings.Join(newLines, "\n")
	return os.WriteFile(workspacePath, []byte(workContent), 0644)
}

// generateLibraryBuildFile creates BUILD.bazel for a library
func generateLibraryBuildFile(libPath, importPath, packageName string) error {
	// Read template
	templateContent, err := template.TemplatesFS.ReadFile("templates/library/BUILD.bazel.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read BUILD template: %w", err)
	}

	// Get library name from path (with dashes) for Bazel target
	libName := filepath.Base(libPath)

	// Prepare template data
	data := struct {
		PackageName string
		ImportPath  string
		Files       []string
		TestFiles   []string
		HasTests    bool
	}{
		PackageName: libName, // Use library name (with dashes) for Bazel target
		ImportPath:  importPath,
		Files:       []string{packageName + ".go"}, // Use package name (no dashes) for filename
		TestFiles:   []string{},
		HasTests:    false,
	}

	// Render template
	engine := template.NewEngine()
	rendered, err := engine.Render(string(templateContent), data)
	if err != nil {
		return fmt.Errorf("failed to render BUILD template: %w", err)
	}

	// Write BUILD.bazel
	buildPath := filepath.Join(libPath, "BUILD.bazel")
	if err := os.WriteFile(buildPath, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("failed to write BUILD.bazel: %w", err)
	}

	fmt.Println("✔ Generated BUILD.bazel")
	return nil
}

// registerLibraryInForgeConfig adds the library to forge.json
func registerLibraryInForgeConfig(libPath, importPath string) error {
	// Find workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}

	// Load workspace config
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Get relative path from workspace root
	relPath, err := filepath.Rel(workspaceRoot, libPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Create library name from path (e.g., "shared/go-kit" -> "go-kit")
	libName := filepath.Base(relPath)

	// Register library project
	project := &workspace.Project{
		ProjectType: "library",
		Language:    "go",
		Root:        relPath,
		Tags:        []string{"library", "shared"},
		Architect: &workspace.Architect{
			Build: &workspace.ArchitectTarget{
				Builder: "@forge/bazel:build",
				Options: map[string]interface{}{
					"target": "/...",
				},
				Configurations: map[string]interface{}{
					"production": map[string]interface{}{},
				},
				DefaultConfiguration: "production",
			},
		},
	}

	config.AddProject(libName, project)

	// Save config
	if err := config.SaveToDir(workspaceRoot); err != nil {
		return fmt.Errorf("failed to save forge.json: %w", err)
	}

	fmt.Printf("✔ Registered library in forge.json\n")
	return nil
}
