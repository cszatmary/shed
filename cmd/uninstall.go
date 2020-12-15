package cmd

import (
	"github.com/TouchBistro/goutils/spinner"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall [tools...]",
	Args:  cobra.MinimumNArgs(1),
	Short: "Uninstall Go tools.",
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
