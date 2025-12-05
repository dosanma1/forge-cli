package cmd

import (
	"context"
	"fmt"

	"github.com/dosanma1/forge-cli/internal/generator"
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
  handler     Generate a new HTTP handler
  middleware  Generate middleware
  frontend    Generate an Angular application

Examples:
  forge generate service user-service
  forge generate nestjs api-gateway
  forge g service payment-service
  forge generate frontend admin-app`,
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

func init() {
	generateCmd.AddCommand(generateServiceCmd)
	generateCmd.AddCommand(generateNestJSCmd)
	generateCmd.AddCommand(generateFrontendCmd)
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
