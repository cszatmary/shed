package cmd

import (
	"fmt"
	"os"

	"github.com/TouchBistro/goutils/fatal"
	"github.com/spf13/cobra"
)

var completionsCmd = &cobra.Command{
	Use:       "completions <shell>",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh"},
	Short:     "Generate shell completions.",
	Long: `shed completions generates a shell completion script and outputs it to standard output.
Supported shells are: bash, zsh.

For example to generate and use bash completions:

	shed completions bash > /usr/local/etc/bash_completion.d/shed.bash
	source /usr/local/etc/bash_completion.d/shed.bash`,
	Run: func(cmd *cobra.Command, args []string) {
		shell := args[0]
		var err error
		switch shell {
		case "bash":
			err = rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			err = rootCmd.GenZshCompletion(os.Stdout)
		default:
			err = fmt.Errorf("invalid shell value %q, run 'shed completions --help' to see supported shells", shell)
		}
		if err != nil {
			fatal.ExitErrf(err, "Failed to generate %s completions", shell)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionsCmd)
}
