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
  handler     Generate a new HTTP handler
  middleware  Generate middleware
  frontend    Generate an Angular application

Examples:
  forge generate service user-service
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
