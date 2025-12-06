# test-project

A Forge workspace for building production-ready microservices.

## Getting Started

### Prerequisites

- Go 1.23+
- Node.js 20+
- Bazel 7+
- Docker

### Project Structure

```
test-project/
├── forge.json          # Workspace configuration
├── backend/            # Backend services
│   └── services/       # Microservices
├── frontend/           # Frontend applications
│   └── projects/       # Angular projects
├── infra/              # Infrastructure
│   ├── helm/           # Kubernetes Helm charts
│   └── cloudrun/       # Cloud Run configurations
├── shared/             # Shared libraries
└── docs/               # Documentation
```

### Commands

```bash
# Generate a new Go service
forge generate service <service-name>

# Generate a new Angular application
forge generate frontend <app-name>

# Add a handler to a service
forge add handler <service-name> <endpoint>

# Add middleware to a service
forge add middleware <service-name> <middleware-type>
```

## Documentation

See [docs/](./docs/) for detailed documentation.
