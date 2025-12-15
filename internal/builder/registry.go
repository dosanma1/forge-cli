package builder

import "fmt"

// Registry of available builders
var builders = map[string]func() Builder{
	"@forge/bazel:build":   func() Builder { return NewBazelBuilder() },
	"@forge/angular:build": func() Builder { return NewAngularBuilder() },
}

// GetBuilder returns a builder instance by name
func GetBuilder(name string) (Builder, error) {
	factory, ok := builders[name]
	if !ok {
		return nil, fmt.Errorf("unknown builder: %s", name)
	}
	return factory(), nil
}

// RegisterBuilder adds a new builder to the registry
func RegisterBuilder(name string, factory func() Builder) {
	builders[name] = factory
}

// ListBuilders returns all registered builder names
func ListBuilders() []string {
	names := make([]string, 0, len(builders))
	for name := range builders {
		names = append(names, name)
	}
	return names
}
