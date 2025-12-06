package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "Forge CLI - Production-ready microservice scaffolding",
	Long: `Forge is a powerful CLI tool for creating and managing production-ready microservices.
It provides standardized patterns for Go services with built-in observability, authentication, and more.

Built with ❤️ following industry best practices.`,
	Version: "1.0.0",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Commands are registered in their respective files via init()
	// This avoids duplicate command registration
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(validateCmd)
}
