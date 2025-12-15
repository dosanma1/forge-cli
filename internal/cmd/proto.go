package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dosanma1/forge-cli/internal/ui"
	"github.com/spf13/cobra"
)

var protoCmd = &cobra.Command{
	Use:   "proto",
	Short: "Compile protocol buffers",
	Long: `Compile protocol buffers using buf or protoc.

This command will:
- Scan for proto/ directories in services
- Detect buf.yaml or use protoc
- Compile .proto files to Go/TypeScript
- Generate gRPC stubs

Examples:
  forge proto
  forge proto --tool=buf
  forge proto --tool=protoc`,
	RunE: runProto,
}

var protoTool string

func init() {
	rootCmd.AddCommand(protoCmd)
	protoCmd.Flags().StringVar(&protoTool, "tool", "auto", "Protobuf tool to use: auto, buf, or protoc")
}

func runProto(cmd *cobra.Command, args []string) error {

	// Find proto directories
	protoDirs, err := findProtoDirs(".")
	if err != nil {
		return fmt.Errorf("failed to scan for proto directories: %w", err)
	}

	if len(protoDirs) == 0 {
		fmt.Println("No proto/ directories found")
		fmt.Println("\nCreate a proto/ directory in your service with .proto files")
		return nil
	}

	fmt.Printf("Found %d proto director%s:\n", len(protoDirs), pluralize(len(protoDirs), "y", "ies"))
	for _, dir := range protoDirs {
		fmt.Printf("  • %s\n", dir)
	}
	fmt.Println()

	// Determine tool to use
	tool := protoTool
	if tool == "auto" {
		tool, err = detectProtoTool(protoDirs)
		if err != nil {
			return err
		}
	}

	fmt.Printf("Using tool: %s\n\n", tool)

	// Compile each directory
	for _, dir := range protoDirs {
		fmt.Printf("Compiling %s...\n", dir)

		var compileErr error
		switch tool {
		case "buf":
			compileErr = compileBuf(dir)
		case "protoc":
			compileErr = compileProtoc(dir)
		default:
			return fmt.Errorf("unknown tool: %s", tool)
		}

		if compileErr != nil {
			fmt.Printf("✗ Failed: %v\n", compileErr)
			return compileErr
		}

		fmt.Println("✔ Success")
		fmt.Println()
	}

	fmt.Println("✔ All proto files compiled successfully.")
	return nil
}

func findProtoDirs(root string) ([]string, error) {
	var protoDirs []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories, node_modules, vendor, etc.
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "dist" || name == "bazel-" {
				return filepath.SkipDir
			}

			// Check if this is a proto directory
			if name == "proto" {
				protoDirs = append(protoDirs, path)
				return filepath.SkipDir
			}
		}

		return nil
	})

	return protoDirs, err
}

func detectProtoTool(protoDirs []string) (string, error) {
	// Check if buf is installed and buf.yaml exists
	if _, err := exec.LookPath("buf"); err == nil {
		for _, dir := range protoDirs {
			bufYaml := filepath.Join(dir, "buf.yaml")
			if _, err := os.Stat(bufYaml); err == nil {
				return "buf", nil
			}
		}
	}

	// Check if protoc is installed
	if _, err := exec.LookPath("protoc"); err == nil {
		return "protoc", nil
	}

	return "", fmt.Errorf("no protobuf compiler found. Install buf (https://buf.build) or protoc (https://grpc.io/docs/protoc-installation/)")
}

func compileBuf(protoDir string) error {
	// Check for buf.yaml
	bufYaml := filepath.Join(protoDir, "buf.yaml")
	if _, err := os.Stat(bufYaml); os.IsNotExist(err) {
		return fmt.Errorf("buf.yaml not found in %s", protoDir)
	}

	// Run buf generate
	cmd := exec.Command("buf", "generate")
	cmd.Dir = protoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func compileProtoc(protoDir string) error {
	// Find all .proto files
	var protoFiles []string
	err := filepath.Walk(protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".proto") {
			relPath, _ := filepath.Rel(protoDir, path)
			protoFiles = append(protoFiles, relPath)
		}
		return nil
	})

	if err != nil {
		return err
	}

	if len(protoFiles) == 0 {
		return fmt.Errorf("no .proto files found in %s", protoDir)
	}

	// Determine output languages
	fmt.Println("\nSelect output languages:")

	var languages []string
	genGo, err := ui.AskConfirm("Generate Go code?", true)
	if err != nil {
		return err
	}
	if genGo {
		languages = append(languages, "Go")
	}

	genTS, err := ui.AskConfirm("Generate TypeScript code?", false)
	if err != nil {
		return err
	}
	if genTS {
		languages = append(languages, "TypeScript")
	}

	genPython, err := ui.AskConfirm("Generate Python code?", false)
	if err != nil {
		return err
	}
	if genPython {
		languages = append(languages, "Python")
	}

	if len(languages) == 0 {
		return fmt.Errorf("no languages selected")
	}

	// Build protoc command
	args := []string{"--proto_path=" + protoDir}

	for _, lang := range languages {
		switch lang {
		case "Go":
			args = append(args, "--go_out=.", "--go-grpc_out=.")
		case "TypeScript":
			args = append(args, "--ts_out=.")
		case "Python":
			args = append(args, "--python_out=.", "--grpc_python_out=.")
		}
	}

	args = append(args, protoFiles...)

	cmd := exec.Command("protoc", args...)
	cmd.Dir = protoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
