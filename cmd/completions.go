package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCompletionsCommand() *cobra.Command {
	return &cobra.Command{
		Use:       "completions <shell>",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh"},
		Short:     "Generate shell completions.",
		Long: `shed completions generates a shell completion script and outputs it to standard output.
Supported shells are: bash, zsh.

For example to generate and use bash completions:

	shed completions bash > /usr/local/etc/bash_completion.d/shed.bash
	source /usr/local/etc/bash_completion.d/shed.bash`,
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			var err error
			switch shell {
			case "bash":
				err = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				err = cmd.Root().GenZshCompletion(os.Stdout)
			default:
				return &exitError{
					code: 1,
					msg: fmt.Sprintf(
						"Invalid shell value %q. Run 'shed completions --help' to see supported shells.",
						shell,
					),
				}
			}
			if err != nil {
				return fmt.Errorf("failed to generate %s completions: %w", shell, err)
			}
			return nil
		},
	}
}
