package cmd

import (
	"github.com/TouchBistro/goutils/spinner"
	"github.com/cszatmary/shed/api"
	"github.com/spf13/cobra"
)

type installOptions struct {
	allowUpdates bool
}

var installOpts installOptions

var installCmd = &cobra.Command{
	Use:   "install [tools...]",
	Args:  cobra.ArbitraryArgs,
	Short: "Install Go tools.",
	Run: func(cmd *cobra.Command, args []string) {
		successCh := make(chan string)
		failedCh := make(chan error)

		go func() {
			err := api.Install(installOpts.allowUpdates, args...)
			if err != nil {
				failedCh <- err
				return
			}

			successCh <- "Finished installing tools"
		}()

		spinner.SpinnerWait(successCh, failedCh, "%s", "Failed to install tools", 1)
	},
}

func init() {
	installCmd.Flags().BoolVarP(&installOpts.allowUpdates, "update", "u", false, "allow updating already installed tools")
	rootCmd.AddCommand(installCmd)
}
