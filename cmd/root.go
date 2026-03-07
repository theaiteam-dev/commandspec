// Package cmd contains the cobra commands for the swagger-jack CLI tool.
package cmd

import (
	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags.
var version = "dev"

// NewRootCmd constructs and returns the root cobra command for swagger-jack.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "swaggerjack",
		Short: "Generate a Go CLI from an OpenAPI 3.x spec",
		Long: `Swagger Jack reads an OpenAPI 3.x spec and produces a complete,
buildable Go CLI project using Cobra.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newValidateCmd())
	root.AddCommand(newInitCmd())

	return root
}

// Execute runs the root command. This is called by main.go.
func Execute() error {
	return NewRootCmd().Execute()
}
