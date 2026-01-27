package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

var studioCmd = &cobra.Command{
	Use:   "studio",
	Short: "Start Forge Studio development environment",
	Long: `Starts the Forge Studio (Angular) and the API service concurrently for development.
It assumes the project structure:
- studio/ (Angular application)
- api/ (Go API service)

The command will:
1. Start the API service on port 8080 (default).
2. Start the Studio development server on port 4200 (default).
`,
	RunE: runStudio,
}

func init() {
	rootCmd.AddCommand(studioCmd)
}

func runStudio(cmd *cobra.Command, args []string) error {
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("not in a forge workspace: %w", err)
	}

	fmt.Println("🚀 Starting Forge Studio...")

	// 1. Start API Service
	apiCmd := exec.Command("go", "run", "cmd/server/main.go")
	apiCmd.Dir = filepath.Join(workspaceRoot, "api")
	apiCmd.Stdout = os.Stdout
	apiCmd.Stderr = os.Stderr
	apiCmd.Env = append(os.Environ(), "PORT=3002")

	// 2. Start Studio (Angular)
	// Detect if using Nx monorepo structure
	var studioCmdExec *exec.Cmd
	nxStudioPath := filepath.Join(workspaceRoot, "apps", "studio")
	nxConfigPath := filepath.Join(workspaceRoot, "nx.json")

	if _, err := os.Stat(nxStudioPath); err == nil {
		if _, err := os.Stat(nxConfigPath); err == nil {
			// Nx monorepo detected
			studioCmdExec = exec.Command("npx", "nx", "serve", "studio")
			studioCmdExec.Dir = workspaceRoot
		}
	}

	// Fallback to legacy structure
	if studioCmdExec == nil {
		studioCmdExec = exec.Command("npm", "run", "start")
		studioCmdExec.Dir = filepath.Join(workspaceRoot, "studio")
	}

	studioCmdExec.Stdout = os.Stdout
	studioCmdExec.Stderr = os.Stderr

	// Start API
	fmt.Println("📡 Starting API Service...")
	if err := apiCmd.Start(); err != nil {
		return fmt.Errorf("failed to start API: %w", err)
	}

	// Start Studio
	fmt.Println("🎨 Starting Studio Frontend...")
	if err := studioCmdExec.Start(); err != nil {
		// Try to kill API if frontend fails to start
		apiCmd.Process.Kill()
		return fmt.Errorf("failed to start Studio: %w", err)
	}

	// Wait for interrupt signal to stop both
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutting down...")
		apiCmd.Process.Signal(syscall.SIGTERM)
		studioCmdExec.Process.Signal(syscall.SIGTERM)
	}()

	// Wait for processes
	go func() {
		apiCmd.Wait()
		fmt.Println("📡 API stopped.")
		sigChan <- syscall.SIGTERM
	}()

	err = studioCmdExec.Wait()
	fmt.Println("🎨 Studio stopped.")

	return err
}
