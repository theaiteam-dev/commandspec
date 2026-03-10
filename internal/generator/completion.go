package generator

import "fmt"

// completionTemplate is the Go source for the generated cmd/completion.go file.
// It produces a completion subcommand for bash, zsh, fish, and powershell.
const completionSrc = `package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh|fish|powershell]",
	Short:     "Generate shell completion scripts",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		switch args[0] {
		case "bash":
			return root.GenBashCompletion(os.Stdout)
		case "zsh":
			return root.GenZshCompletion(os.Stdout)
		case "fish":
			return root.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return root.GenPowerShellCompletion(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell %q: must be one of bash, zsh, fish, powershell", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
`

// GenerateCompletion returns Go source code for the generated project's
// cmd/completion.go file. The cliName parameter is accepted for future use.
func GenerateCompletion(cliName string) (string, error) {
	if cliName == "" {
		return "", fmt.Errorf("cliName must not be empty")
	}
	return validateGoSource(completionSrc)
}
