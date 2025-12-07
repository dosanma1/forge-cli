package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/generator"
	"github.com/dosanma1/forge-cli/internal/ui"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:     "generate [type] [name]",
	Aliases: []string{"g"},
	Short:   "Generate code from schematics",
	Long: `Generate application components using Forge generators.

Available types:
  service     Generate a new Go microservice
  nestjs      Generate a new NestJS microservice
  frontend    Generate an Angular application
  library     Generate a shared library

Examples:
  forge generate service user-service
  forge generate nestjs api-gateway
  forge g service payment-service
  forge generate frontend admin-app
  forge g library shared/auth`,
}

var generateServiceCmd = &cobra.Command{
	Use:   "service [name]",
	Short: "Generate a new Go microservice",
	Long: `Generate a new Go microservice with Forge patterns.

The service will include:
- Main application with HTTP server
- Logging and observability setup
- Health check endpoint
- Example API route
- Dockerfile for containerization
- README with documentation

Examples:
  forge generate service user-service
  forge g service payment-service`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerateService,
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
	generateCmd.AddCommand(generateServiceCmd)
	generateCmd.AddCommand(generateNestJSCmd)
	generateCmd.AddCommand(generateFrontendCmd)
	generateCmd.AddCommand(generateLibraryCmd)
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
	serviceName := args[0]

	// Create generator
	gen := generator.NewServiceGenerator()

	// Prepare options
	opts := generator.GeneratorOptions{
		OutputDir: ".",
		Name:      serviceName,
		DryRun:    false,
	}

	// Generate service
	ctx := context.Background()
	if err := gen.Generate(ctx, opts); err != nil {
		return fmt.Errorf("failed to generate service: %w", err)
	}

	return nil
}

func runGenerateLibrary(cmd *cobra.Command, args []string) error {
	libPath := args[0]

	fmt.Println(ui.TitleStyle.Render("📦 Generate Library"))
	fmt.Println()

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

	fmt.Printf("\nCreating %s library at %s...\n\n", libType, ui.SuccessStyle.Render(libPath))

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

	fmt.Println(ui.SuccessStyle.Render("✓ Library created successfully!"))
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

	// Try to add to go.work if it exists
	workspacePath, err := findGoWorkspace()
	if err == nil {
		if err := addToGoWorkspace(workspacePath, path); err != nil {
			fmt.Printf("⚠️  Could not add to go.work: %v\n", err)
		} else {
			fmt.Println(ui.SuccessStyle.Render("✓ Added to go.work"))
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

	// Add new use directive before the closing parenthesis or after "use ("
	if strings.Contains(workContent, "use (") {
		workContent = strings.Replace(workContent, "use (", fmt.Sprintf("use (\n\t./%s", relPath), 1)
	} else if strings.Contains(workContent, "use") {
		workContent = strings.Replace(workContent, ")", fmt.Sprintf("\t./%s\n)", relPath), 1)
	} else {
		workContent += fmt.Sprintf("\n\nuse (\n\t./%s\n)\n", relPath)
	}

	return os.WriteFile(workspacePath, []byte(workContent), 0644)
}
