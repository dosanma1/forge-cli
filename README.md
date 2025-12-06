# Forge Framework

<p align="center">
  <img src="assets/logo-cli.png" alt="Forge CLI Logo" width="200"/>
</p>

Forge is a comprehensive Go framework and CLI tool for building production-ready microservices with standardized patterns.

## Architecture

Forge consists of two repositories:

1. **forge** (`github.com/dosanma1/forge`) - Reusable Go library with standardized patterns
2. **forge-cli** (`github.com/dosanma1/forge-cli`) - CLI tool for scaffolding and code generation

## Installation

```bash
# Install forge-cli
go install github.com/dosanma1/forge-cli/cmd/forge@latest

# Or build from source
git clone https://github.com/dosanma1/forge-cli
cd forge-cli
go build -o forge cmd/forge/main.go
```

## Quick Start

### Create a New Workspace

```bash
# Interactive mode
forge new

# With options
forge new my-project \
  --github-org=mycompany \
  --docker-registry=gcr.io/mycompany \
  --gcp-project=my-gcp-project
```

### Generate a Service

```bash
cd my-project
forge generate service user-service
cd backend/services/user-service
go mod tidy
go run main.go
```

## Forge Library (`github.com/dosanma1/forge`)

### HTTP Package (`forge/pkg/http`)

```go
import "github.com/dosanma1/forge/pkg/http"

// Create router
router := http.NewRouter()

// Add middleware
router.Use(http.LoggingMiddleware(logger))
router.Use(http.RecoveryMiddleware(logger))

// Register routes
router.GET("/users", getUsersHandler)
router.POST("/users", createUserHandler)

// Route groups
v1 := router.Group("/api/v1")
v1.GET("/users/:id", getUserHandler)

// Start server
router.Start(":8080")
```

### Logging Package (`forge/pkg/log`)

```go
import "github.com/dosanma1/forge/pkg/log"

// Create logger
logger := log.NewLogger("my-service", log.LevelInfo)

// Log with context
logger.Info("User created", "user_id", userID)
logger.Error("Failed to create user", "error", err)

// Add logger to context
ctx = log.ToContext(ctx, logger)

// Retrieve from context
logger = log.FromContext(ctx)
```

### Database Package (`forge/pkg/database`)

```go
import (
	"github.com/dosanma1/forge/pkg/database"
	"gorm.io/gorm"
)

// Generic repository pattern
type User struct {
	gorm.Model
	Name  string
	Email string
}

type UserRepository struct {
	database.BaseRepository[User]
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		BaseRepository: database.NewBaseRepository[User](db),
	}
}

// Use repository
repo := NewUserRepository(db)
users, err := repo.FindAll(ctx)
user, err := repo.FindByID(ctx, 1)
err = repo.Create(ctx, &user)
```

### Observability Package (`forge/pkg/observability`)

```go
import "github.com/dosanma1/forge/pkg/observability"

// Initialize tracer
tracer := observability.NewTracer("my-service", "1.0.0")
defer tracer.Shutdown(ctx)

// Create spans
ctx, span := tracer.StartSpan(ctx, "operation-name")
defer span.End()

// Add events and errors
span.AddEvent("Processing started")
span.RecordError(err)
```

### Authentication Package (`forge/pkg/auth`)

```go
import "github.com/dosanma1/forge/pkg/auth"

// JWT middleware
validator := &MyTokenValidator{}
router.Use(auth.JWTMiddleware(validator, logger))

// Role-based access
router.Use(auth.RequireRole("admin"))

// Get user from context
user := auth.UserFromContext(ctx)
```

### Configuration Package (`forge/pkg/config`)

```go
import "github.com/dosanma1/forge/pkg/config"

// Environment-based config
cfg := config.NewEnvConfig("MYSERVICE")

// Get values with defaults
port := cfg.GetInt("PORT", 8080)
debug := cfg.GetBool("DEBUG", false)
timeout := cfg.GetDuration("TIMEOUT", time.Second*30)
hosts := cfg.GetStringSlice("HOSTS", []string{"localhost"})
```

### Testing Package (`forge/pkg/testing`)

```go
import "github.com/dosanma1/forge/pkg/testing"

func TestAPI(t *testing.T) {
	// Create test server
	server := testing.NewTestServer(router)
	defer server.Close()

	// Make requests
	resp := server.GET("/api/users")

	// Assertions
	testing.AssertStatusCode(t, resp, 200)
	testing.AssertJSON(t, resp, map[string]interface{}{
		"count": 10,
	})
}

// Table-driven tests
tests := []testing.TestCase{
	{
		Name: "Valid input",
		Input: "test",
		Expected: true,
	},
	{
		Name: "Invalid input",
		Input: "",
		Expected: false,
	},
}

testing.RunTableTests(t, tests, func(tc testing.TestCase) {
	result := validate(tc.Input.(string))
	assert.Equal(t, tc.Expected, result)
})
```

## Forge CLI Commands

### `forge new [name]`

Create a new workspace:

```bash
forge new my-project
forge new my-project --github-org=mycompany
```

### `forge generate service [name]`

Generate a Go microservice:

```bash
forge generate service user-service
forge g service payment-service
```

### `forge generate frontend [name]` (Coming Soon)

Generate an Angular application:

```bash
forge generate frontend admin-app
```

### `forge clean`

Clean build artifacts and caches:

```bash
# Clean project caches (.forge, .angular) and run bazel clean --expunge
forge clean --cache

# Deep clean including global caches (with confirmation)
forge clean --deep
```

### `forge add handler [service] [endpoint]` (Coming Soon)

Add HTTP handler to a service:

```bash
forge add handler user-service /api/users
```

### `forge add middleware [service] [type]` (Coming Soon)

Add middleware to a service:

```bash
forge add middleware user-service auth
forge add middleware user-service logging
```

## Workspace Structure

```
my-project/
├── forge.json              # Workspace configuration
├── backend/
│   └── services/
│       └── user-service/
│           ├── main.go
│           ├── go.mod
│           ├── Dockerfile
│           └── README.md
├── frontend/
│   └── projects/
│       └── admin-app/
├── infra/
│   ├── helm/
│   └── cloudrun/
├── shared/                 # Shared libraries
└── docs/
```

## forge.json Configuration

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

## Project Types

- `go` - Go microservice
- `nestjs` - NestJS microservice
- `angular` - Angular application

## Version Management

Forge locks framework versions in `forge.json` to ensure consistent, reproducible builds across your team and CI/CD pipelines.

### Tool Versions

When you create a new workspace, Forge initializes locked versions in `forge.json`:

```json
{
  "workspace": {
    "toolVersions": {
      "angular": "21.0.2",
      "go": "1.23.4",
      "nestjs": "11.1.9",
      "node": "24.11.1",
      "bazel": "7.4.1"
    }
  }
}
```

### Why Lock Versions?

1. **Stability** - Prevents `@latest` from introducing breaking changes unexpectedly
2. **Bazel Compatibility** - Ensures framework versions work with Bazel build rules
3. **Team Consistency** - Everyone builds with same versions
4. **Reproducibility** - Builds work identically 6 months later

### Updating Versions

Forge uses Dependabot to monitor your project dependencies:

1. Dependabot creates PRs for `go.mod` and `package.json` updates
2. Review and merge Dependabot PRs after testing
3. Manually sync versions to `forge.json` `toolVersions` section
4. Run `forge build` to validate compatibility

See [Version Compatibility Matrix](docs/VERSION_COMPATIBILITY.md) for tested combinations.

### Cache Management

Forge uses project-local caches (like Angular's `.angular/cache/`):

```bash
# Clean project caches and Bazel artifacts
forge clean --cache

# Deep clean (includes global caches: ~/.cache/bazel, ~/go/pkg/mod/cache, ~/.npm)
forge clean --deep
```

Caches are stored in:

- `.forge/cache/` - Template cache (gitignored)
- `.angular/cache/` - Angular build cache (gitignored)
- `~/.cache/bazel` - Bazel global cache

## Philosophy

1. **Standardization** - One way to do things, consistently across all services
2. **Simplicity** - Use standard library when possible, minimal dependencies
3. **Type Safety** - Leverage Go generics for type-safe patterns
4. **Observability** - Built-in logging, tracing, and metrics
5. **Developer Experience** - Fast, intuitive CLI with sensible defaults

## Examples

See the `examples/` directory in the forge repository for complete working examples.

## Development

### Building forge-cli

```bash
cd forge-cli
go build -o forge cmd/forge/main.go
./forge --help
```

### Testing

```bash
# Test forge library
cd forge
go test ./...

# Test forge-cli
cd forge-cli
go test ./...
```

## Version

Current version: **1.0.0**

## License

MIT License - see LICENSE file for details
