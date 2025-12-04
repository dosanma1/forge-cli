package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dosanma1/forge-cli/internal/template"
	"github.com/dosanma1/forge-cli/internal/workspace"
)

// ServiceGenerator generates a new Go microservice.
type ServiceGenerator struct {
	engine *template.Engine
}

// NewServiceGenerator creates a new service generator.
func NewServiceGenerator() *ServiceGenerator {
	return &ServiceGenerator{
		engine: template.NewEngine(),
	}
}

// Name returns the generator name.
func (g *ServiceGenerator) Name() string {
	return "service"
}

// Description returns the generator description.
func (g *ServiceGenerator) Description() string {
	return "Generate a new Go microservice with Forge patterns"
}

// Generate creates a new service.
func (g *ServiceGenerator) Generate(ctx context.Context, opts GeneratorOptions) error {
	serviceName := opts.Name
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}

	// Validate name
	if err := workspace.ValidateName(serviceName); err != nil {
		return fmt.Errorf("invalid service name: %w", err)
	}

	// Load workspace config
	config, err := workspace.LoadConfig(opts.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Check if service already exists
	if config.GetProject(serviceName) != nil {
		return fmt.Errorf("project %q already exists", serviceName)
	}

	serviceDir := filepath.Join(opts.OutputDir, "backend/services", serviceName)

	if opts.DryRun {
		fmt.Printf("Would create service: %s\n", serviceDir)
		return nil
	}

	// Create service directory
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create service directory: %w", err)
	}

	// Prepare template data
	githubOrg := "github.com/yourorg"
	if config.Workspace.GitHub != nil {
		githubOrg = config.Workspace.GitHub.Org
	}

	data := map[string]interface{}{
		"ServiceName":       serviceName,
		"ServiceNamePascal": template.Pascalize(serviceName),
		"ServiceNameCamel":  template.Camelize(serviceName),
		"ModulePath":        fmt.Sprintf("%s/%s", githubOrg, config.Workspace.Name),
	}

	// Create main.go
	mainTemplate := `package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dosanma1/forge/pkg/config"
	"github.com/dosanma1/forge/pkg/http"
	"github.com/dosanma1/forge/pkg/log"
	"github.com/dosanma1/forge/pkg/observability"
)

func main() {
	// Initialize logger
	logger := log.NewLogger("{{.ServiceName}}", log.LevelInfo)
	
	// Load configuration
	cfg := config.NewEnvConfig("{{.ServiceNamePascal | upper}}")
	port := cfg.GetInt("PORT", 8080)
	
	// Initialize observability
	tracer := observability.NewTracer("{{.ServiceName}}", "1.0.0")
	defer tracer.Shutdown(context.Background())
	
	// Create router
	router := http.NewRouter()
	
	// Register middleware
	router.Use(http.LoggingMiddleware(logger))
	router.Use(http.RecoveryMiddleware(logger))
	router.Use(http.CORSMiddleware(http.CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
	}))
	
	// Register routes
	registerRoutes(router, logger, tracer)
	
	// Start server
	addr := fmt.Sprintf(":%d", port)
	logger.Info("Starting server", "addr", addr)
	
	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		if err := router.Start(addr); err != nil {
			logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()
	
	<-quit
	logger.Info("Shutting down server...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := router.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", "error", err)
	}
}

func registerRoutes(router *http.Router, logger *log.Logger, tracer *observability.Tracer) {
	// Health check
	router.GET("/health", func(ctx *http.Context) error {
		return ctx.JSON(200, map[string]string{"status": "ok"})
	})
	
	// API v1 group
	v1 := router.Group("/api/v1")
	{
		// TODO: Add your routes here
		v1.GET("/example", func(ctx *http.Context) error {
			return ctx.Success("{{.ServiceNamePascal}} is running")
		})
	}
}
`

	mainPath := filepath.Join(serviceDir, "main.go")
	if err := g.engine.RenderToFile(mainTemplate, data, mainPath); err != nil {
		return fmt.Errorf("failed to create main.go: %w", err)
	}

	// Create go.mod
	modTemplate := `module {{.ModulePath}}/backend/services/{{.ServiceName}}

go 1.23

require github.com/dosanma1/forge v1.0.0
`

	modPath := filepath.Join(serviceDir, "go.mod")
	if err := g.engine.RenderToFile(modTemplate, data, modPath); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	// Create Dockerfile
	dockerfileContent := `FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/service .
EXPOSE 8080
CMD ["./service"]
`

	dockerfilePath := filepath.Join(serviceDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}

	// Create README.md
	readmeTemplate := `# {{.ServiceNamePascal}}

A Forge-based microservice.

## Running Locally

` + "```bash" + `
# Install dependencies
go mod tidy

# Run service
go run main.go

# Run with environment variables
{{.ServiceNamePascal | upper}}_PORT=9090 go run main.go
` + "```" + `

## Environment Variables

- ` + "`{{.ServiceNamePascal | upper}}_PORT`" + ` - HTTP server port (default: 8080)

## Endpoints

- ` + "`GET /health`" + ` - Health check
- ` + "`GET /api/v1/example`" + ` - Example endpoint

## Building

` + "```bash" + `
# Build binary
go build -o {{.ServiceName}} .

# Build Docker image
docker build -t {{.ServiceName}}:latest .
` + "```" + `
`

	readmePath := filepath.Join(serviceDir, "README.md")
	if err := g.engine.RenderToFile(readmeTemplate, data, readmePath); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	// Add project to workspace config
	project := &workspace.Project{
		Name: serviceName,
		Type: workspace.ProjectTypeGoService,
		Root: fmt.Sprintf("backend/services/%s", serviceName),
		Tags: []string{"backend", "service"},
	}

	if err := config.AddProject(project); err != nil {
		return fmt.Errorf("failed to add project to config: %w", err)
	}

	if err := config.SaveToDir(opts.OutputDir); err != nil {
		return fmt.Errorf("failed to save workspace config: %w", err)
	}

	fmt.Printf("✓ Service %q created successfully\n", serviceName)
	fmt.Printf("✓ Location: %s\n", serviceDir)
	fmt.Printf("✓ Run 'cd %s && go mod tidy' to install dependencies\n", serviceDir)
	fmt.Printf("✓ Run 'cd %s && go run main.go' to start the service\n", serviceDir)

	return nil
}
