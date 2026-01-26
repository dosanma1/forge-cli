package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GoServiceBuilder generates Go microservice code from forge.json
type GoServiceBuilder struct{}

// NewGoServiceBuilder creates a new Go service builder
func NewGoServiceBuilder() *GoServiceBuilder {
	return &GoServiceBuilder{}
}

// Name returns the builder identifier
func (b *GoServiceBuilder) Name() string {
	return "go-service"
}

// Description returns a human-readable description
func (b *GoServiceBuilder) Description() string {
	return "Generates Go microservice code following Clean Architecture patterns"
}

// Parse parses the forge.json for Go service generation
func (b *GoServiceBuilder) Parse(ctx context.Context, opts ParseOptions) (*ParseResult, error) {
	var forgeJSON []byte
	var err error

	if opts.ForgeJSON != nil {
		forgeJSON = opts.ForgeJSON
	} else {
		forgeJSONPath := filepath.Join(opts.ProjectDir, "forge.json")
		forgeJSON, err = os.ReadFile(forgeJSONPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read forge.json: %w", err)
		}
	}

	var raw struct {
		Name     string                 `json:"name"`
		Type     string                 `json:"type"`
		Nodes    []Node                 `json:"nodes"`
		Edges    []Edge                 `json:"edges"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	if err := json.Unmarshal(forgeJSON, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse forge.json: %w", err)
	}

	return &ParseResult{
		ProjectName: raw.Name,
		ProjectType: raw.Type,
		Nodes:       raw.Nodes,
		Edges:       raw.Edges,
		Metadata:    raw.Metadata,
	}, nil
}

// Generate produces Go code from the parsed result
func (b *GoServiceBuilder) Generate(ctx context.Context, opts GenerateOptions) error {
	if opts.ParseResult == nil {
		return fmt.Errorf("ParseResult is required")
	}

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = opts.ProjectDir
	}

	progress := func(pct int, msg string) {
		if opts.ProgressFunc != nil {
			opts.ProgressFunc(pct, msg)
		}
	}

	progress(0, "Starting code generation...")

	// Group nodes by type
	entities := make([]Node, 0)
	restEndpoints := make([]Node, 0)
	grpcServices := make([]Node, 0)
	natsProducers := make([]Node, 0)
	natsConsumers := make([]Node, 0)

	for _, node := range opts.ParseResult.Nodes {
		switch node.Type {
		case "entity":
			entities = append(entities, node)
		case "rest-endpoint":
			restEndpoints = append(restEndpoints, node)
		case "grpc-service":
			grpcServices = append(grpcServices, node)
		case "nats-producer":
			natsProducers = append(natsProducers, node)
		case "nats-consumer":
			natsConsumers = append(natsConsumers, node)
		}
	}

	totalSteps := len(entities) + len(restEndpoints) + len(grpcServices) + len(natsProducers) + len(natsConsumers) + 2 // +2 for module.go and types.go
	currentStep := 0

	// Generate entity files
	for _, entity := range entities {
		currentStep++
		progress(currentStep*100/totalSteps, fmt.Sprintf("Generating entity: %s", entity.Data["name"]))

		if !opts.DryRun {
			if err := b.generateEntity(ctx, outputDir, entity, opts.ParseResult.Edges); err != nil {
				return fmt.Errorf("failed to generate entity %s: %w", entity.Data["name"], err)
			}
		}
	}

	// Generate REST transport files
	for _, endpoint := range restEndpoints {
		currentStep++
		progress(currentStep*100/totalSteps, fmt.Sprintf("Generating REST endpoint: %s", endpoint.Data["basePath"]))

		if !opts.DryRun {
			if err := b.generateRESTTransport(ctx, outputDir, endpoint, entities, opts.ParseResult.Edges); err != nil {
				return fmt.Errorf("failed to generate REST endpoint: %w", err)
			}
		}
	}

	// Generate gRPC service files
	for _, service := range grpcServices {
		currentStep++
		progress(currentStep*100/totalSteps, fmt.Sprintf("Generating gRPC service: %s", service.Data["name"]))

		if !opts.DryRun {
			if err := b.generateGRPCService(ctx, outputDir, service, entities, opts.ParseResult.Edges); err != nil {
				return fmt.Errorf("failed to generate gRPC service: %w", err)
			}
		}
	}

	// Generate NATS producer files
	for _, producer := range natsProducers {
		currentStep++
		progress(currentStep*100/totalSteps, fmt.Sprintf("Generating NATS producer: %s", producer.Data["subject"]))

		if !opts.DryRun {
			if err := b.generateNATSProducer(ctx, outputDir, producer); err != nil {
				return fmt.Errorf("failed to generate NATS producer: %w", err)
			}
		}
	}

	// Generate NATS consumer files
	for _, consumer := range natsConsumers {
		currentStep++
		progress(currentStep*100/totalSteps, fmt.Sprintf("Generating NATS consumer: %s", consumer.Data["subject"]))

		if !opts.DryRun {
			if err := b.generateNATSConsumer(ctx, outputDir, consumer); err != nil {
				return fmt.Errorf("failed to generate NATS consumer: %w", err)
			}
		}
	}

	// Generate module.go
	currentStep++
	progress(currentStep*100/totalSteps, "Generating module.go")
	if !opts.DryRun {
		if err := b.generateModule(ctx, outputDir, opts.ParseResult); err != nil {
			return fmt.Errorf("failed to generate module.go: %w", err)
		}
	}

	// Generate types.go
	currentStep++
	progress(currentStep*100/totalSteps, "Generating types.go")
	if !opts.DryRun {
		if err := b.generateTypes(ctx, outputDir, opts.ParseResult); err != nil {
			return fmt.Errorf("failed to generate types.go: %w", err)
		}
	}

	progress(100, "Code generation complete!")
	return nil
}

// Validate checks if the configuration is valid
func (b *GoServiceBuilder) Validate(ctx context.Context, opts ValidateOptions) error {
	if opts.ParseResult == nil {
		return fmt.Errorf("ParseResult is required")
	}

	var errors []ValidationError

	// Validate entities
	for _, node := range opts.ParseResult.Nodes {
		if node.Type == "entity" {
			if node.Data["name"] == nil || node.Data["name"] == "" {
				errors = append(errors, ValidationError{
					NodeID:  node.ID,
					Field:   "name",
					Message: "Entity name is required",
					Severe:  true,
				})
			}

			fields, ok := node.Data["fields"].([]interface{})
			if !ok || len(fields) == 0 {
				errors = append(errors, ValidationError{
					NodeID:  node.ID,
					Field:   "fields",
					Message: "Entity must have at least one field",
					Severe:  true,
				})
			}
		}

		if node.Type == "rest-endpoint" {
			if node.Data["basePath"] == nil || node.Data["basePath"] == "" {
				errors = append(errors, ValidationError{
					NodeID:  node.ID,
					Field:   "basePath",
					Message: "REST endpoint base path is required",
					Severe:  true,
				})
			}

			// Check that REST endpoint is connected to an entity
			hasEntityConnection := false
			for _, edge := range opts.ParseResult.Edges {
				if edge.Target == node.ID {
					for _, n := range opts.ParseResult.Nodes {
						if n.ID == edge.Source && n.Type == "entity" {
							hasEntityConnection = true
							break
						}
					}
				}
			}
			if !hasEntityConnection {
				errors = append(errors, ValidationError{
					NodeID:  node.ID,
					Message: "REST endpoint must be connected to an entity",
					Severe:  true,
				})
			}
		}
	}

	if len(errors) > 0 {
		return &ValidationResult{
			Valid:  false,
			Errors: errors,
		}
	}

	return nil
}

// ValidationResult implements the error interface
func (v *ValidationResult) Error() string {
	if len(v.Errors) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %d errors", len(v.Errors))
}

// Placeholder implementations for code generation
// These will be expanded with actual template-based generation

func (b *GoServiceBuilder) generateEntity(ctx context.Context, outputDir string, entity Node, edges []Edge) error {
	// TODO: Implement entity code generation using templates
	return nil
}

func (b *GoServiceBuilder) generateRESTTransport(ctx context.Context, outputDir string, endpoint Node, entities []Node, edges []Edge) error {
	// TODO: Implement REST transport code generation using templates
	return nil
}

func (b *GoServiceBuilder) generateGRPCService(ctx context.Context, outputDir string, service Node, entities []Node, edges []Edge) error {
	// TODO: Implement gRPC service code generation using templates
	return nil
}

func (b *GoServiceBuilder) generateNATSProducer(ctx context.Context, outputDir string, producer Node) error {
	// TODO: Implement NATS producer code generation using templates
	return nil
}

func (b *GoServiceBuilder) generateNATSConsumer(ctx context.Context, outputDir string, consumer Node) error {
	// TODO: Implement NATS consumer code generation using templates
	return nil
}

func (b *GoServiceBuilder) generateModule(ctx context.Context, outputDir string, result *ParseResult) error {
	// TODO: Implement module.go generation using templates
	return nil
}

func (b *GoServiceBuilder) generateTypes(ctx context.Context, outputDir string, result *ParseResult) error {
	// TODO: Implement types.go generation using templates
	return nil
}

func init() {
	// Register the Go service builder
	Register(NewGoServiceBuilder())
}
