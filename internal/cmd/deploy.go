package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	deployEnv          string
	deployMode         string
	deployTarget       string
	deployVerbose      bool
	deployTail         bool
	deployPort         int
	deploySkipBuild    bool
	deployDryRun       bool
	deployServices     string
	deployFrontendOnly bool
	deployServicesOnly bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy services using the architect pattern (under development)",
	Long: `The deploy command is being redesigned to work with the new architect pattern.

The old deployment system using environments and infrastructure configuration
has been replaced with a more flexible architect-based approach.

This command will be available again soon with improved functionality.`,
	RunE: runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVarP(&deployEnv, "env", "e", "local", "Environment to deploy to")
	deployCmd.Flags().StringVarP(&deployTarget, "target", "r", "", "Deployment target")
	deployCmd.Flags().StringVarP(&deployMode, "mode", "m", "dev", "Deployment mode")
	deployCmd.Flags().BoolVarP(&deployVerbose, "verbose", "v", false, "Show verbose output")
	deployCmd.Flags().BoolVarP(&deployTail, "tail", "t", false, "Stream logs after deployment")
	deployCmd.Flags().IntVarP(&deployPort, "port", "p", 0, "Port forward")
	deployCmd.Flags().BoolVar(&deploySkipBuild, "skip-build", false, "Skip build phase")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show deployment plan")
	deployCmd.Flags().StringVar(&deployServices, "services", "", "Deploy specific services")
	deployCmd.Flags().BoolVar(&deployFrontendOnly, "frontend-only", false, "Deploy only frontend")
	deployCmd.Flags().BoolVar(&deployServicesOnly, "services-only", false, "Deploy only backend")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("deploy command is being redesigned for the new architect pattern and is temporarily unavailable")
}
