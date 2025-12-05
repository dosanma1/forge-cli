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
	deployEnv       string
	deployMode      string
	deployTarget    string
	deployVerbose   bool
	deployTail      bool
	deployPort      int
	deploySkipBuild bool
	deployDryRun    bool
	deployServices  string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy services using Skaffold with Bazel",
	Long: `Deploy your services to GKE, Kubernetes, or Google Cloud Run.

Skaffold with Bazel builder automatically detects changed services via 'bazel query'
and only rebuilds/deploys those services (incremental deployment).

Deployment targets are configured in forge.json under 'environments[].target'.
Supported targets: gke, kubernetes, cloudrun

Environments are configured in forge.json under 'environments'.
Default environments: local, dev, staging, prod (fully customizable)

Modes (--mode):
  dev     - Continuous development with hot reload (default, K8s/GKE only)
  run     - One-time deployment
  debug   - Deploy with debugging enabled (K8s/GKE only)

Examples:
  forge deploy                              # Deploy to local (dev mode)
  forge deploy --env=dev                    # Deploy to dev environment
  forge deploy --env=prod --mode=run        # One-time prod deploy
  forge deploy --skip-build                 # Deploy pre-built images
  forge deploy --dry-run                    # Show what would be deployed
  forge deploy --services=api,worker        # Deploy specific services only`,
	RunE: runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVarP(&deployEnv, "env", "e", "local", "Environment to deploy to (defined in forge.json)")
	deployCmd.Flags().StringVarP(&deployTarget, "target", "r", "", "Deployment target (gke|kubernetes|cloudrun, defaults to forge.json config)")
	deployCmd.Flags().StringVarP(&deployMode, "mode", "m", "dev", "Deployment mode (dev|run|debug)")
	deployCmd.Flags().BoolVarP(&deployVerbose, "verbose", "v", false, "Show verbose output")
	deployCmd.Flags().BoolVarP(&deployTail, "tail", "t", false, "Stream logs after deployment")
	deployCmd.Flags().IntVarP(&deployPort, "port", "p", 0, "Port forward (0 = use defaults)")
	deployCmd.Flags().BoolVar(&deploySkipBuild, "skip-build", false, "Skip build phase, deploy existing images")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show what would be deployed without executing")
	deployCmd.Flags().StringVar(&deployServices, "services", "", "Deploy specific services only (comma-separated)")
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

	// Determine deployment target (flag overrides config, default to kubernetes)
	target := deployTarget
	if target == "" {
		target = envConfig.Target
	}
	if target == "" {
		target = "kubernetes" // default
	}

	// Validate target (support gke as alias for kubernetes with GKE-specific features)
	validTargets := map[string]bool{"gke": true, "kubernetes": true, "cloudrun": true}
	if !validTargets[target] {
		return fmt.Errorf("invalid target: %s (must be gke, kubernetes, or cloudrun)", target)
	}

	// Cloud Run doesn't support dev and debug modes (except local simulation)
	if target == "cloudrun" && deployEnv != "local" && (deployMode == "dev" || deployMode == "debug") {
		return fmt.Errorf("Cloud Run only supports mode=run (continuous dev/debug modes not supported)")
	}

	// Dry-run mode
	if deployDryRun {
		return showDeploymentPlan(workspaceRoot, config, envConfig, target)
	}

	// Route to appropriate deployment function
	switch target {
	case "gke", "kubernetes":
		return deployToKubernetes(workspaceRoot, config, envConfig)
	case "cloudrun":
		return deployToCloudRun(workspaceRoot, config, envConfig)
	default:
		return fmt.Errorf("unsupported target: %s", target)
	}
}

func deployToKubernetes(workspaceRoot string, config *workspace.Config, envConfig workspace.EnvironmentConfig) error {
	// Detect if this is GKE
	isGKE := envConfig.Target == "gke" || (config.Infrastructure != nil && config.Infrastructure.GKE != nil)

	// For local deployment, ensure kind cluster and infrastructure are ready
	if deployEnv == "local" {
		if err := setupLocalInfrastructure(workspaceRoot, config); err != nil {
			return fmt.Errorf("failed to setup local infrastructure: %w", err)
		}
	}

	// For GKE, ensure cluster credentials are configured
	if isGKE && deployEnv != "local" {
		if err := setupGKECredentials(config, envConfig); err != nil {
			return fmt.Errorf("failed to setup GKE credentials: %w", err)
		}
	}

	// Check if Skaffold is installed
	if _, err := exec.LookPath("skaffold"); err != nil {
		return fmt.Errorf("skaffold not found. Install with: curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 && sudo install skaffold /usr/local/bin/")
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

	// Skip build phase if requested (use pre-built images)
	if deploySkipBuild {
		skaffoldArgs = append(skaffoldArgs, "--skip-build")
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

	// Deploy specific services only
	if deployServices != "" {
		services := strings.Split(deployServices, ",")
		for _, svc := range services {
			skaffoldArgs = append(skaffoldArgs, fmt.Sprintf("--build-image=%s", strings.TrimSpace(svc)))
		}
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

	targetName := "Kubernetes"
	if isGKE {
		targetName = "GKE"
	}

	fmt.Printf("%s Deploying to %s on %s [%s]...\n", emoji, targetName, envDesc, modeDesc[deployMode])
	if deployVerbose {
		fmt.Printf("  ℹ️  Skaffold will use Bazel to detect changed services automatically\n")
		fmt.Printf("  Running: skaffold %s\n", strings.Join(skaffoldArgs, " "))
	} else {
		fmt.Printf("  ℹ️  Bazel detecting changes...\n")
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

// deployToCloudRun deploys services to Google Cloud Run
func deployToCloudRun(workspaceRoot string, config *workspace.Config, envConfig workspace.EnvironmentConfig) error {
	// Check for local simulation mode
	if deployEnv == "local" {
		return deployCloudRunLocally(workspaceRoot, config, envConfig)
	}

	// Check if gcloud is installed
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("gcloud CLI not found. Install from: https://cloud.google.com/sdk/docs/install")
	}

	// Determine GCP project and region
	gcpProject := ""
	if config.Workspace.GCP != nil {
		gcpProject = config.Workspace.GCP.ProjectID
	}
	if gcpProject == "" {
		return fmt.Errorf("GCP project not configured in forge.json (workspace.gcp.projectId)")
	}

	region := envConfig.Region
	if region == "" && config.Infrastructure != nil && config.Infrastructure.CloudRun != nil {
		region = config.Infrastructure.CloudRun.Region
	}
	if region == "" {
		region = "us-central1" // default region
	}

	fmt.Printf("☁️  Deploying to Cloud Run [%s]...\n", deployEnv)
	fmt.Printf("   Project: %s\n", gcpProject)
	fmt.Printf("   Region: %s\n", region)

	// Warn for production
	if strings.Contains(strings.ToLower(deployEnv), "prod") {
		fmt.Printf("⚠️  Deploying to %s - please confirm\n", strings.ToUpper(deployEnv))
		if !confirmDeployment() {
			fmt.Println("❌ Deployment cancelled")
			return nil
		}
	}

	// Get list of services to deploy
	services := []string{}
	for name, project := range config.Projects {
		if project.Type == "go-service" || project.Type == "service" {
			services = append(services, name)
		}
	}

	if len(services) == 0 {
		return fmt.Errorf("no services found to deploy")
	}

	fmt.Printf("   Services: %s\n\n", strings.Join(services, ", "))

	// Deploy each service
	for _, serviceName := range services {
		project := config.Projects[serviceName]
		if err := deployServiceToCloudRun(workspaceRoot, serviceName, project.Root, gcpProject, region, envConfig); err != nil {
			return fmt.Errorf("failed to deploy service %s: %w", serviceName, err)
		}
	}

	fmt.Printf("\n✅ Deployed successfully to Cloud Run (%s)\n", deployEnv)
	return nil
}

// deployServiceToCloudRun deploys a single service to Cloud Run
func deployServiceToCloudRun(workspaceRoot, serviceName, serviceRoot, gcpProject, region string, envConfig workspace.EnvironmentConfig) error {
	fmt.Printf("📦 Deploying %s...\n", serviceName)

	// Build image name
	registry := envConfig.Registry
	if registry == "" {
		registry = fmt.Sprintf("gcr.io/%s", gcpProject)
	}
	imageTag := fmt.Sprintf("%s/%s:%s-latest", registry, serviceName, deployEnv)

	// Build and push Docker image
	fmt.Printf("   Building image: %s\n", imageTag)

	dockerfilePath := fmt.Sprintf("%s/%s/Dockerfile", workspaceRoot, serviceRoot)
	buildCmd := exec.Command("docker", "build",
		"-t", imageTag,
		"-f", dockerfilePath,
		workspaceRoot, // context is workspace root
	)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Push image
	fmt.Printf("   Pushing image to registry...\n")
	pushCmd := exec.Command("docker", "push", imageTag)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %w", err)
	}

	// Deploy to Cloud Run
	fmt.Printf("   Deploying to Cloud Run...\n")

	deployArgs := []string{
		"run", "deploy", serviceName,
		"--image", imageTag,
		"--platform", "managed",
		"--region", region,
		"--project", gcpProject,
		"--allow-unauthenticated", // TODO: make this configurable
		"--port", "8080",
		"--memory", "512Mi",
		"--cpu", "1",
		"--max-instances", "10",
		"--min-instances", "0",
	}

	// Add environment variables if specified
	if envConfig.Variables != nil && len(envConfig.Variables) > 0 {
		for key, value := range envConfig.Variables {
			deployArgs = append(deployArgs, "--set-env-vars", fmt.Sprintf("%s=%s", key, value))
		}
	}

	deployCmd := exec.Command("gcloud", deployArgs...)
	deployCmd.Stdout = os.Stdout
	deployCmd.Stderr = os.Stderr
	if err := deployCmd.Run(); err != nil {
		return fmt.Errorf("gcloud run deploy failed: %w", err)
	}

	fmt.Printf("   ✅ %s deployed successfully\n\n", serviceName)
	return nil
}

// setupGKECredentials configures kubectl for GKE cluster access.
func setupGKECredentials(config *workspace.Config, envConfig workspace.EnvironmentConfig) error {
	// Check if gcloud is installed
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("gcloud CLI not found. Install from: https://cloud.google.com/sdk/docs/install")
	}

	// Determine GKE configuration
	var projectID, clusterName, region string

	// Try from infrastructure.gke
	if config.Infrastructure != nil && config.Infrastructure.GKE != nil {
		projectID = config.Infrastructure.GKE.ProjectID
		clusterName = config.Infrastructure.GKE.ClusterName
		region = config.Infrastructure.GKE.Region
	}

	// Override with environment-specific values if provided
	if envConfig.Cluster != "" {
		clusterName = envConfig.Cluster
	}
	if envConfig.Region != "" {
		region = envConfig.Region
	}

	// Fallback to workspace.gcp
	if projectID == "" && config.Workspace.GCP != nil {
		projectID = config.Workspace.GCP.ProjectID
	}

	// Validate required fields
	if projectID == "" {
		return fmt.Errorf("GCP project ID not configured. Add to infrastructure.gke.projectId or workspace.gcp.projectId in forge.json")
	}
	if clusterName == "" {
		return fmt.Errorf("GKE cluster name not configured. Add to infrastructure.gke.clusterName in forge.json")
	}
	if region == "" {
		region = "us-central1" // default region
	}

	if deployVerbose {
		fmt.Printf("  ℹ️  Configuring GKE credentials: %s/%s (%s)\n", projectID, clusterName, region)
	}

	// Get GKE credentials (use --region for regional clusters)
	cmd := exec.Command("gcloud", "container", "clusters", "get-credentials", clusterName,
		"--region", region,
		"--project", projectID,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to get GKE credentials: %w", err)
	}

	return nil
}

// showDeploymentPlan shows what would be deployed without actually deploying (dry-run mode).
func showDeploymentPlan(workspaceRoot string, config *workspace.Config, envConfig workspace.EnvironmentConfig, target string) error {
	fmt.Println("🔍 Deployment Plan (dry-run mode)")
	fmt.Println("═══════════════════════════════════")
	fmt.Printf("Environment:    %s\n", deployEnv)
	fmt.Printf("Target:         %s\n", target)
	fmt.Printf("Mode:           %s\n", deployMode)
	fmt.Printf("Namespace:      %s\n", envConfig.Namespace)

	if envConfig.Registry != "" {
		fmt.Printf("Registry:       %s\n", envConfig.Registry)
	}

	fmt.Println("\n📦 Services that would be deployed:")

	// Use Bazel query to find services that have changed
	queryCmd := exec.Command("bazel", "query", "kind('.*_image', //backend/services/...)")
	queryCmd.Dir = workspaceRoot
	output, err := queryCmd.Output()
	if err != nil {
		// Fallback: list all services from config
		for name, project := range config.Projects {
			if project.Type == "go-service" {
				fmt.Printf("  • %s\n", name)
			}
		}
	} else {
		// Parse Bazel query output
		targets := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, target := range targets {
			if target != "" {
				serviceName := extractServiceNameFromTarget(target)
				if serviceName != "" {
					fmt.Printf("  • %s\n", serviceName)
				}
			}
		}
	}

	fmt.Println("\n✓ Use 'forge deploy' without --dry-run to proceed")
	return nil
}

// extractServiceNameFromTarget extracts service name from Bazel target.
// Example: //backend/services/api-server:image -> api-server
func extractServiceNameFromTarget(target string) string {
	parts := strings.Split(target, "/")
	for i, part := range parts {
		if part == "services" && i+1 < len(parts) {
			serviceName := parts[i+1]
			serviceName = strings.Split(serviceName, ":")[0]
			return serviceName
		}
	}
	return ""
}

// deployCloudRunLocally simulates Cloud Run deployment locally using Docker
func deployCloudRunLocally(workspaceRoot string, config *workspace.Config, envConfig workspace.EnvironmentConfig) error {
	fmt.Println("🏠 Deploying locally (Cloud Run simulation)...")
	fmt.Println("   ℹ️  Running services in Docker with Cloud Run environment")

	// Get list of services to deploy
	services := []string{}
	for name, project := range config.Projects {
		if project.Type == workspace.ProjectTypeGoService {
			services = append(services, name)
		}
	}

	if len(services) == 0 {
		return fmt.Errorf("no services found to deploy")
	}

	fmt.Printf("\n📦 Building and running services: %s\n\n", strings.Join(services, ", "))

	// Auto-assign ports starting from 8080
	nextPort := 8080
	servicesPorts := make(map[string]int)

	for _, serviceName := range services {
		project := config.Projects[serviceName]
		if project.Port > 0 {
			servicesPorts[serviceName] = project.Port
		} else {
			servicesPorts[serviceName] = nextPort
			nextPort++
		}
	}

	// Deploy each service with its assigned port
	for _, serviceName := range services {
		port := servicesPorts[serviceName]
		if err := runServiceLocallyAsCloudRun(workspaceRoot, serviceName, port, config); err != nil {
			return fmt.Errorf("failed to run service %s: %w", serviceName, err)
		}
	}

	fmt.Printf("\n✅ Services running locally (Cloud Run simulation)\n")
	fmt.Println("\nℹ️  Useful commands:")
	fmt.Println("   docker ps                     # View running containers")
	fmt.Println("   docker logs -f <container>    # Stream logs")
	fmt.Println("   docker stop <container>       # Stop service")

	return nil
}

// runServiceLocallyAsCloudRun runs a single service locally with Cloud Run environment
func runServiceLocallyAsCloudRun(workspaceRoot, serviceName string, port int, config *workspace.Config) error {
	fmt.Printf("🚀 Starting %s...\n", serviceName)

	// Build image using Bazel for linux/amd64 (Cloud Run target)
	fmt.Printf("   Building image with Bazel (linux/amd64)...\n")
	buildCmd := exec.Command("bazel", "run",
		"--platforms=@rules_go//go/toolchain:linux_amd64",
		fmt.Sprintf("//backend/services/%s/cmd/server:image.tar", serviceName))
	buildCmd.Dir = workspaceRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}

	// Stop and remove existing container
	stopCmd := exec.Command("docker", "stop", serviceName)
	stopCmd.Run() // Ignore errors if container doesn't exist
	rmCmd := exec.Command("docker", "rm", serviceName)
	rmCmd.Run() // Ignore errors if container doesn't exist

	// Run container with Cloud Run environment variables
	fmt.Printf("   Running on port %d with Cloud Run environment...\n", port)
	runArgs := []string{
		"run",
		"-d",                  // Detached mode
		"--name", serviceName, // Container name
		"-p", fmt.Sprintf("%d:%d", port, port), // Port mapping
		"-e", fmt.Sprintf("PORT=%d", port), // Cloud Run PORT env var
		"-e", "K_SERVICE=" + serviceName, // Cloud Run service name
		"-e", "K_REVISION=" + serviceName + "-001", // Cloud Run revision
		"-e", "K_CONFIGURATION=" + serviceName, // Cloud Run configuration
		"--restart", "unless-stopped", // Auto-restart
		fmt.Sprintf("%s:latest", serviceName), // Image name
	}

	runCmd := exec.Command("docker", runArgs...)
	runCmd.Dir = workspaceRoot
	if deployVerbose {
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
	}
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	fmt.Printf("   ✅ %s running at http://localhost:%d\n", serviceName, port)
	return nil
}
