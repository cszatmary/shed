package cmd

import (
	"os"

	"github.com/getshiphub/shed/client"
	"github.com/getshiphub/shed/internal/spinner"
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
		s := spinner.NewTTY(spinner.Options{
			Message:         "Uninstalling tools",
			PersistMessages: rootOpts.verbose,
		})
		logger := newLogger()
		logger.Out = s
		setwd(logger)
		shed := mustShed(client.WithLogger(logger))
		s.Start()

		err := shed.Uninstall(args...)
		s.Stop()
		if err != nil {
			fatal.ExitErrf(err, "Failed to uninstall tools")
		}
		logger.Out = os.Stderr
		logger.Info("Finished uninstalling tools")
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
