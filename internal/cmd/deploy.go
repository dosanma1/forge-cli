package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dosanma1/forge-cli/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	deployEnv     string
	deployMode    string
	deployVerbose bool
	deployTail    bool
	deployPort    int
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy services using Skaffold",
	Long: `Deploy your services to Kubernetes using Skaffold.

Skaffold handles:
- Building Docker images
- Pushing to registry
- Deploying with Helm charts
- Port forwarding and log streaming

Environments are configured in forge.json under 'environments'.
Default environments: local, dev, staging, prod (fully customizable)

Modes (--mode):
  dev     - Continuous development with hot reload (default)
  run     - One-time deployment
  debug   - Deploy with debugging enabled

Examples:
  forge deploy                           # Dev mode on local
  forge deploy --env=dev                 # Deploy to dev
  forge deploy --env=prod --mode=run     # One-time prod deploy
  forge deploy --env=custom-env          # Deploy to custom environment
  forge deploy --tail                    # Stream logs after deploy
  forge deploy --port=8080               # Custom port forward`,
	RunE: runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVarP(&deployEnv, "env", "e", "local", "Environment to deploy to (defined in forge.json)")
	deployCmd.Flags().StringVarP(&deployMode, "mode", "m", "dev", "Deployment mode (dev|run|debug)")
	deployCmd.Flags().BoolVarP(&deployVerbose, "verbose", "v", false, "Show verbose output")
	deployCmd.Flags().BoolVarP(&deployTail, "tail", "t", false, "Stream logs after deployment")
	deployCmd.Flags().IntVarP(&deployPort, "port", "p", 0, "Port forward (0 = use defaults)")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	// Validate mode
	validModes := map[string]bool{"dev": true, "run": true, "debug": true}
	if !validModes[deployMode] {
		return fmt.Errorf("invalid mode: %s (must be dev, run, or debug)", deployMode)
	}

	// Get workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		return fmt.Errorf("not in a forge workspace: %w", err)
	}

	// Load workspace config
	config, err := workspace.LoadConfig(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace config: %w", err)
	}

	// Validate environment exists in config
	envConfig, envExists := config.Environments[deployEnv]
	if !envExists {
		// List available environments
		availableEnvs := make([]string, 0, len(config.Environments))
		for envName := range config.Environments {
			availableEnvs = append(availableEnvs, envName)
		}
		if len(availableEnvs) == 0 {
			return fmt.Errorf("no environments configured in forge.json. Add environments to deploy")
		}
		return fmt.Errorf("environment %q not found. Available: %s", deployEnv, strings.Join(availableEnvs, ", "))
	}

	// For local deployment, ensure kind cluster and infrastructure are ready
	if deployEnv == "local" {
		if err := setupLocalInfrastructure(workspaceRoot, config); err != nil {
			return fmt.Errorf("failed to setup local infrastructure: %w", err)
		}
	}

	// Check if Skaffold is installed
	if _, err := exec.LookPath("skaffold"); err != nil {
		return fmt.Errorf("skaffold not found. Install with: curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-darwin-amd64 && sudo install skaffold /usr/local/bin/")
	}

	// Build Skaffold command
	skaffoldArgs := []string{}

	switch deployMode {
	case "dev":
		skaffoldArgs = append(skaffoldArgs, "dev")
	case "run":
		skaffoldArgs = append(skaffoldArgs, "run")
	case "debug":
		skaffoldArgs = append(skaffoldArgs, "debug")
	}

	// Add profile for environment (use custom profile if specified, otherwise use env name)
	profile := deployEnv
	if envConfig.Profile != "" {
		profile = envConfig.Profile
	}
	if profile != "" && profile != "local" {
		skaffoldArgs = append(skaffoldArgs, fmt.Sprintf("--profile=%s", profile))
	}

	// Add namespace if specified
	if envConfig.Namespace != "" {
		skaffoldArgs = append(skaffoldArgs, fmt.Sprintf("--namespace=%s", envConfig.Namespace))
	}

	// Add tail flag if requested
	if deployTail {
		skaffoldArgs = append(skaffoldArgs, "--tail")
	}

	// Add port forward if specified
	if deployPort > 0 {
		skaffoldArgs = append(skaffoldArgs, fmt.Sprintf("--port-forward=%d", deployPort))
	}

	// Add verbose flag
	if deployVerbose {
		skaffoldArgs = append(skaffoldArgs, "--verbosity=debug")
	}

	// Show user-friendly message
	envEmoji := map[string]string{
		"local":   "🏠",
		"dev":     "🔧",
		"staging": "🧪",
		"prod":    "🚀",
	}
	emoji := envEmoji[deployEnv]
	if emoji == "" {
		emoji = "🚀" // Default emoji for custom environments
	}

	modeDesc := map[string]string{
		"dev":   "continuous development",
		"run":   "one-time deployment",
		"debug": "debug mode",
	}

	envDesc := deployEnv
	if envConfig.Description != "" {
		envDesc = fmt.Sprintf("%s (%s)", deployEnv, envConfig.Description)
	}

	fmt.Printf("%s Deploying to %s [%s]...\n", emoji, envDesc, modeDesc[deployMode])
	if deployVerbose {
		fmt.Printf("  Running: skaffold %s\n", strings.Join(skaffoldArgs, " "))
	}

	// Warn for production or any environment with "prod" in the name
	if (strings.Contains(strings.ToLower(deployEnv), "prod") || strings.Contains(strings.ToLower(deployEnv), "production")) && deployMode == "run" {
		fmt.Printf("⚠️  Deploying to %s - please confirm\n", strings.ToUpper(deployEnv))
		if !confirmDeployment() {
			fmt.Println("❌ Deployment cancelled")
			return nil
		}
	}

	// Execute Skaffold
	skaffoldCmd := exec.Command("skaffold", skaffoldArgs...)
	skaffoldCmd.Dir = workspaceRoot
	skaffoldCmd.Stdout = os.Stdout
	skaffoldCmd.Stderr = os.Stderr
	skaffoldCmd.Stdin = os.Stdin

	if err := skaffoldCmd.Run(); err != nil {
		return fmt.Errorf("❌ Deployment failed: %w", err)
	}

	// Success message depends on mode
	if deployMode == "dev" {
		fmt.Printf("\n✅ Development session started on %s\n", deployEnv)
		fmt.Println("   Press Ctrl+C to stop")
	} else {
		fmt.Printf("\n✅ Deployed successfully to %s\n", deployEnv)
	}

	// Show useful commands
	if deployEnv == "local" {
		fmt.Println("\nℹ️  Useful commands:")
		fmt.Println("   kubectl get pods              # View running pods")
		fmt.Println("   kubectl logs -f <pod-name>    # Stream logs")
		fmt.Println("   kubectl port-forward <pod> 8080:8080  # Port forward")
	}

	_ = config // Use config if needed for registry info

	return nil
}

// setupLocalInfrastructure ensures kind cluster, ingress controller, and API gateway are ready
func setupLocalInfrastructure(workspaceRoot string, config *workspace.Config) error {
	clusterName := fmt.Sprintf("kind-%s", config.Workspace.Name)

	fmt.Println("🔧 Step 1/5: Ensuring Kubernetes cluster exists...")

	// Check if kind cluster exists
	checkCmd := exec.Command("kind", "get", "clusters")
	output, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check kind clusters: %w", err)
	}

	clusterExists := strings.Contains(string(output), config.Workspace.Name)

	if !clusterExists {
		fmt.Println("   Creating new kind cluster...")
		kindConfigPath := fmt.Sprintf("%s/infra/kind-config.yaml", workspaceRoot)
		createCmd := exec.Command("kind", "create", "cluster",
			"--name", config.Workspace.Name,
			"--config", kindConfigPath)
		createCmd.Stdout = os.Stdout
		createCmd.Stderr = os.Stderr
		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("failed to create kind cluster: %w", err)
		}
	} else {
		fmt.Println("   ✓ Cluster already exists, skipping creation")
	}

	fmt.Println("🔧 Step 2/5: Setting kubectl context...")
	contextCmd := exec.Command("kubectl", "config", "use-context", clusterName)
	if err := contextCmd.Run(); err != nil {
		return fmt.Errorf("failed to set kubectl context: %w", err)
	}
	fmt.Printf("   ✅ kubectl context set to %s\n", clusterName)

	fmt.Println("🔧 Step 3/5: Creating namespace...")
	namespace := config.Workspace.Name
	nsCmd := exec.Command("kubectl", "create", "namespace", namespace,
		"--dry-run=client", "-o", "yaml")
	applyCmd := exec.Command("kubectl", "apply", "-f", "-")
	applyCmd.Stdin, _ = nsCmd.StdoutPipe()
	if err := applyCmd.Start(); err != nil {
		return fmt.Errorf("failed to start kubectl apply: %w", err)
	}
	if err := nsCmd.Run(); err != nil {
		return fmt.Errorf("failed to create namespace yaml: %w", err)
	}
	if err := applyCmd.Wait(); err != nil {
		return fmt.Errorf("failed to apply namespace: %w", err)
	}
	fmt.Printf("   ✅ Namespace '%s' created or already exists\n", namespace)

	fmt.Println("🔧 Step 4/5: Ensuring ingress controller...")
	if err := setupIngressController(); err != nil {
		return fmt.Errorf("failed to setup ingress controller: %w", err)
	}
	fmt.Println("   ✅ Ingress controller ready")

	fmt.Println("🔧 Step 5/5: Deploying API Gateway...")
	if err := deployAPIGateway(workspaceRoot); err != nil {
		return fmt.Errorf("failed to deploy API gateway: %w", err)
	}
	fmt.Println("   ✅ API Gateway ready")

	return nil
}

// setupIngressController ensures ingress-nginx is installed and ready
func setupIngressController() error {
	// Check if ingress-nginx namespace exists
	checkCmd := exec.Command("kubectl", "get", "namespace", "ingress-nginx")
	if err := checkCmd.Run(); err != nil {
		// Install ingress controller
		fmt.Println("   Installing ingress-nginx controller...")
		installCmd := exec.Command("kubectl", "apply", "-f",
			"https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/kind/deploy.yaml")
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install ingress controller: %w", err)
		}

		// Give it a moment for pods to be created
		fmt.Println("   Waiting for ingress controller pods to be created...")
		time.Sleep(5 * time.Second)
	}

	// Wait for ingress controller to be ready
	fmt.Println("   Waiting for ingress controller to be ready...")
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		waitCmd := exec.Command("kubectl", "wait",
			"--namespace=ingress-nginx",
			"--for=condition=ready",
			"pod",
			"--selector=app.kubernetes.io/component=controller",
			"--timeout=10s")
		if err := waitCmd.Run(); err == nil {
			return nil
		}
		if i < maxRetries-1 {
			fmt.Printf("   Retry %d/%d...\n", i+1, maxRetries)
			time.Sleep(2 * time.Second)
		}
	}

	return fmt.Errorf("timeout waiting for ingress controller after %d retries", maxRetries)
}

// deployAPIGateway deploys the API Gateway Helm chart
func deployAPIGateway(workspaceRoot string) error {
	apiGatewayDir := fmt.Sprintf("%s/infra/api-gateway", workspaceRoot)

	// Deploy using skaffold
	deployCmd := exec.Command("skaffold", "run", "--profile=local")
	deployCmd.Dir = apiGatewayDir
	deployCmd.Stdout = os.Stdout
	deployCmd.Stderr = os.Stderr
	if err := deployCmd.Run(); err != nil {
		return fmt.Errorf("failed to deploy API gateway: %w", err)
	}

	return nil
}

func confirmDeployment() bool {
	fmt.Print("Type 'yes' to continue: ")
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(response) == "yes"
}
