# Forge CLI AGENTS.md

## Project Overview

**Forge CLI** (`forge-cli`) is the command-line tool for the Forge framework. It handles scaffolding, code generation, and project management for Forge workspaces.

## Build and Test

- **Build**: `make build` (outputs to `bin/forge`)
- **Test**: `make test` (`go test -v ./...`)
- **Install**: `make install` (installs to `$GOPATH/bin`)
- **Format**: `make fmt`
- **Lint**: `make lint` (uses `golangci-lint`)

## Project Structure

- `cmd/forge/`: Main entry point and command definitions.
- `internal/`: Core logic for commands.
- `templates/`: Scaffolding templates.
- `docs/`: Documentation.

## Code Style & Conventions

- **Go Version**: 1.24+
- **CLI Framework**: Varies, check `cmd` structure (likely `cobra` or `urfave/cli`).
- **Error Handling**: Return descriptive errors.

## Commands Reference

### Workspace Management

- `forge new [name]`: Creates a new workspace with standardized structure.
  - Examples: `forge new my-project`, `forge new my-project --github-org=mycompany`
- `forge setup`: Initializes tools and environment for the workspace.
- `forge switch [env]`: Switches between environments (e.g., local, dev, prod).
- `forge clean`: Cleans build artifacts and caches (`--cache`, `--deep` options).
- `forge sync`: Regenerates build files (Bazel) and workspace configurations.

### Development & Scaffolding

- `forge generate [type] [name]`: Scaffolds new components.
  - `forge generate service [name]`: Creates a new microservice.
  - `forge generate library [name]`: Creates a shared library.
  - `forge generate frontend [name]`: Creates a frontend application.
- `forge build`: Builds the project or specific targets.
- `forge test`: Runs tests across the workspace.
  - Aliases: `forge t`
- `forge run [service]`: Runs a specific service locally.
- `forge validate`: Validates workspace configuration and structure.

### DevOps & Deployment

- `forge deploy`: Deploys the project or services.
- `forge proto`: Manages Protocol Buffer generation and linting.

## Development

- Use `make deploy` to build and deploy changes to a local test workspace for verification.
- Ensure backward compatibility when modifying `forge new` or `generate` logic.
