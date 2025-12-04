# Forge CLI - Quick Start Guide

## Installation

```bash
# From source (current)
cd /Users/domingosanzmarti/Projects/monorepo-cli
go build -o bin/forge cmd/forge/main.go

# Add to PATH (optional)
sudo cp bin/forge /usr/local/bin/forge
```

## Quick Start

### 1. Create a New Workspace

```bash
# With GitHub org
forge new my-project --github-org=mycompany

# With all options
forge new my-project \
  --github-org=mycompany \
  --docker-registry=gcr.io/mycompany \
  --gcp-project=my-gcp-project \
  --k8s-namespace=production
```

This creates:
```
my-project/
├── forge.json              # Workspace configuration
├── README.md               # Project documentation
├── .gitignore              # Git ignore rules
├── backend/
│   └── services/           # Microservices directory
├── frontend/
│   └── projects/           # Angular apps directory
├── infra/
│   ├── helm/               # Kubernetes charts
│   └── cloudrun/           # Cloud Run configs
├── shared/                 # Shared libraries
└── docs/                   # Documentation
```

### 2. Generate Your First Service

```bash
cd my-project
forge generate service user-service
```

This creates a complete Go microservice:
```
backend/services/user-service/
├── main.go                 # Main application
├── go.mod                  # Go module
├── Dockerfile              # Container image
└── README.md               # Service docs
```

The service includes:
- ✅ HTTP server with routing
- ✅ Structured logging (slog)
- ✅ OpenTelemetry tracing
- ✅ Middleware (logging, recovery, CORS)
- ✅ Health check endpoint
- ✅ Graceful shutdown
- ✅ Example API routes

### 3. Run the Service

```bash
cd backend/services/user-service

# Install dependencies
go mod tidy

# Run the service
go run main.go
```

Test it:
```bash
# Health check
curl http://localhost:8080/health

# Example endpoint
curl http://localhost:8080/api/v1/example
```

### 4. Generate More Services

```bash
# Back to workspace root
cd ../../../

# Generate another service
forge generate service payment-service
forge generate service notification-service
```

Each service is automatically registered in `forge.json`.

## Generated Service Structure

```go
// main.go - Complete working example
package main

import (
    "github.com/dosanma1/forge/pkg/http"
    "github.com/dosanma1/forge/pkg/log"
    "github.com/dosanma1/forge/pkg/observability"
    // ... more Forge imports
)

func main() {
    // Logger
    logger := log.NewLogger("service-name", log.LevelInfo)
    
    // Config
    cfg := config.NewEnvConfig("SERVICENAME")
    port := cfg.GetInt("PORT", 8080)
    
    // Tracing
    tracer := observability.NewTracer("service-name", "1.0.0")
    defer tracer.Shutdown(context.Background())
    
    // Router with middleware
    router := http.NewRouter()
    router.Use(http.LoggingMiddleware(logger))
    router.Use(http.RecoveryMiddleware(logger))
    
    // Routes
    router.GET("/health", healthHandler)
    
    // Start with graceful shutdown
    router.Start(fmt.Sprintf(":%d", port))
}
```

## Configuration

### forge.json

```json
{
  "version": "1",
  "workspace": {
    "name": "my-project",
    "forgeVersion": "1.0.0",
    "github": {
      "org": "mycompany"
    },
    "docker": {
      "registry": "gcr.io/mycompany"
    },
    "gcp": {
      "projectId": "my-gcp-project"
    },
    "kubernetes": {
      "namespace": "production"
    }
  },
  "projects": {
    "user-service": {
      "name": "user-service",
      "type": "go-service",
      "root": "backend/services/user-service",
      "tags": ["backend", "service"]
    }
  }
}
```

## Environment Variables

Services use environment-based configuration:

```bash
# Service-specific prefix (auto-generated from name)
USERSERVICE_PORT=9090
USERSERVICE_DEBUG=true
USERSERVICE_LOG_LEVEL=debug

# Run with custom config
USERSERVICE_PORT=9090 go run main.go
```

## Docker Support

Each service includes a production-ready Dockerfile:

```bash
cd backend/services/user-service

# Build image
docker build -t user-service:latest .

# Run container
docker run -p 8080:8080 user-service:latest
```

## Available Commands

```bash
# Workspace management
forge new <name>                    # Create workspace
forge new <name> --github-org=org   # With GitHub org

# Code generation
forge generate service <name>       # Generate Go service
forge g service <name>              # Short alias

# Coming soon
forge generate frontend <name>      # Generate Angular app
forge add handler <service> <path>  # Add HTTP handler
forge add middleware <service>      # Add middleware
```

## CLI Help

```bash
# Main help
forge --help

# Command help
forge new --help
forge generate --help
forge generate service --help
```

## Forge Library Usage

Your generated services use the Forge library (`github.com/dosanma1/forge`):

```go
// HTTP routing
router := http.NewRouter()
router.GET("/users/:id", getUserHandler)
router.POST("/users", createUserHandler)

// Middleware
router.Use(http.LoggingMiddleware(logger))
router.Use(http.RecoveryMiddleware(logger))
router.Use(http.CORSMiddleware(corsConfig))

// Route groups
v1 := router.Group("/api/v1")
v1.GET("/users", listUsersHandler)

// Logging with context
logger := log.NewLogger("my-service", log.LevelInfo)
logger.Info("User created", "user_id", userID)
ctx = log.ToContext(ctx, logger)

// Generic repositories
type UserRepository struct {
    database.BaseRepository[User]
}
repo := NewUserRepository(db)
users, _ := repo.FindAll(ctx)

// Tracing
tracer := observability.NewTracer("service", "1.0.0")
ctx, span := tracer.StartSpan(ctx, "operation")
defer span.End()

// Environment config
cfg := config.NewEnvConfig("MYSERVICE")
port := cfg.GetInt("PORT", 8080)
debug := cfg.GetBool("DEBUG", false)
```

## Next Steps

1. **Customize your services** - Add business logic to generated services
2. **Add more services** - Generate additional microservices as needed
3. **Add databases** - Use `forge/pkg/database` for GORM repositories
4. **Add authentication** - Use `forge/pkg/auth` for JWT middleware
5. **Add tests** - Use `forge/pkg/testing` for HTTP test helpers

## Examples

See generated services for working examples, or check:
- `/Users/domingosanzmarti/Projects/forge/examples/simple-api/` - Complete example
- Generated services in your workspace
- Forge library documentation

## Troubleshooting

### "Module not found" errors

```bash
# In service directory
go mod tidy
```

### Port already in use

```bash
# Use custom port
SERVICENAME_PORT=9090 go run main.go
```

### Import errors

The Forge library (`github.com/dosanma1/forge`) must be accessible. Currently it's a local module. To publish:

1. Push to GitHub: `github.com/dosanma1/forge`
2. Tag version: `git tag v1.0.0 && git push --tags`
3. Services will automatically fetch it via `go mod tidy`

## Version

Current version: **1.0.0**

Built with Go 1.23+
