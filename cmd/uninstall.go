package cmd

import (
	"github.com/cszatmary/shed/internal/spinner"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <tool> [tools...]",
	Args:  cobra.MinimumNArgs(1),
	Short: "Uninstall Go tools.",
	Long: `shed uninstall removes the given tools from the shed.lock file.
This does not remove the actual tool binaries. This shed uses a single shared cache
to install tool binaries and therefore, other projects could be using the same tool.
If you wish to remove tool binaries see 'shed cache clean'.

The tool name can either be the full import path or the binary name if it is unique.

For example to uninstall the 'golang.org/x/tools/cmd/stringer' tool:

	shed uninstall stringer

Or:

	shed uninstall golang.org/x/tools/cmd/stringer`,
	Run: func(cmd *cobra.Command, args []string) {
		successCh := make(chan string)
		failedCh := make(chan error)

		go func() {
			err := shed.Uninstall(args...)
			if err != nil {
				failedCh <- err
				return
			}

			successCh <- "Finished uninstalling tools"
		}()

		spinner.SpinnerWait(successCh, failedCh, "%s", "Failed to uninstall tools", 1)
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
