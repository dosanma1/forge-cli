package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xeipuuv/gojsonschema"

	"github.com/dosanma1/forge-cli/internal/workspace"
)

//go:embed schemas/forge-config.v1.schema.json
var schemaFS embed.FS

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate forge.json configuration",
	Long: `Validates the forge.json configuration file against the JSON Schema.
This ensures your workspace configuration is correct and follows the expected structure.`,
	RunE: runValidate,
}

var (
	validateFix bool
)

func init() {
	validateCmd.Flags().BoolVar(&validateFix, "fix", false, "Attempt to auto-fix common issues")
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Find forge.json
	configPath := filepath.Join(cwd, "forge.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("forge.json not found in current directory")
	}

	fmt.Println("🔍 Validating forge.json...")

	// Load config
	config, err := workspace.LoadConfig(cwd)
	if err != nil {
		return fmt.Errorf("failed to load forge.json: %w", err)
	}

	// Load schema from embedded file
	schemaBytes, err := schemaFS.ReadFile("schemas/forge-config.v1.schema.json")
	if err != nil {
		return fmt.Errorf("failed to load JSON schema: %w", err)
	}

	// Create schema loader
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)

	// Read config file as JSON
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read forge.json: %w", err)
	}

	documentLoader := gojsonschema.NewBytesLoader(configBytes)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if result.Valid() {
		fmt.Println("✅ forge.json is valid!")

		// Additional semantic validations
		if err := validateSemantics(config); err != nil {
			fmt.Printf("\n⚠️  Semantic warning: %v\n", err)
			if validateFix {
				fmt.Println("🔧 Attempting to fix...")
				if err := fixSemanticIssues(config, cwd); err != nil {
					return fmt.Errorf("failed to fix issues: %w", err)
				}
				fmt.Println("✅ Issues fixed!")
			}
		}

		return nil
	}

	// Print validation errors
	fmt.Println("\n❌ Validation failed with the following errors:")
	fmt.Println()

	for i, desc := range result.Errors() {
		fmt.Printf("%d. %s\n", i+1, desc.String())
		fmt.Printf("   Field: %s\n", desc.Field())
		fmt.Printf("   Type: %s\n\n", desc.Type())
	}

	if validateFix {
		fmt.Println("🔧 Auto-fix is not yet supported for schema validation errors.")
		fmt.Println("Please manually correct the errors listed above.")
	}

	return fmt.Errorf("validation failed with %d errors", len(result.Errors()))
}

// validateSemantics performs additional semantic validation beyond schema
func validateSemantics(config *workspace.Config) error {
	// Semantic validation for architect pattern
	// Could add checks like:
	// - All deployers referenced are valid
	// - All builders referenced are valid
	// - Options match expected schemas for each builder/deployer

	// For now, just return nil as the JSON schema handles most validation
	return nil
}

// fixSemanticIssues attempts to auto-fix common semantic issues
func fixSemanticIssues(config *workspace.Config, workspaceDir string) error {
	// Auto-fix logic removed - per-project config should be explicit
	return nil
}

// formatJSON formats JSON with proper indentation
func formatJSON(data interface{}) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
