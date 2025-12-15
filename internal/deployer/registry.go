package deployer

import "fmt"

// skaffoldSupport defines which builder+deployer combinations support Skaffold
var skaffoldSupport = map[string]map[string]bool{
	"@forge/helm:deploy": {
		"@forge/bazel:build":   true,
		"@forge/docker:build":  true,
		"@forge/angular:build": false,
		"@forge/go:build":      false,
	},
	"@forge/kubectl:deploy": {
		"@forge/bazel:build":  true,
		"@forge/docker:build": true,
	},
	"@forge/cloudrun:deploy": {
		"@forge/bazel:build":  true,
		"@forge/docker:build": true,
	},
	"@forge/firebase:deploy": {
		// Firebase never uses Skaffold
		"@forge/bazel:build":   false,
		"@forge/angular:build": false,
	},
}

// Registry of available deployers
var deployers = map[string]func() Deployer{
	"@forge/firebase:deploy": func() Deployer { return NewFirebaseDeployer() },
	"@forge/helm:deploy":     func() Deployer { return NewHelmDeployer() },
}

// GetDeployer returns a deployer instance by name
func GetDeployer(name string) (Deployer, error) {
	factory, ok := deployers[name]
	if !ok {
		return nil, fmt.Errorf("unknown deployer: %s", name)
	}
	return factory(), nil
}

// RegisterDeployer adds a new deployer to the registry
func RegisterDeployer(name string, factory func() Deployer) {
	deployers[name] = factory
}

// ListDeployers returns all registered deployer names
func ListDeployers() []string {
	names := make([]string, 0, len(deployers))
	for name := range deployers {
		names = append(names, name)
	}
	return names
}

// CanUseSkaffold checks if a builder+deployer combination supports Skaffold
func CanUseSkaffold(deployerName, builderName string) bool {
	if builders, ok := skaffoldSupport[deployerName]; ok {
		if supported, exists := builders[builderName]; exists {
			return supported
		}
	}
	return false
}
