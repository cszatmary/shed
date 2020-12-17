package cmd

import (
	"github.com/getshiphub/shed/client"
	"github.com/getshiphub/shed/internal/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Set by goreleaser when release build is created.
var version string

type rootOptions struct {
	verbose bool
}

var (
	rootOpts rootOptions
	shed     *client.Shed
	logger   = logrus.StandardLogger()
	fatal    = util.Fatal{}
)

var rootCmd = &cobra.Command{
	Use:     "shed",
	Version: version,
	Short:   "shed is a CLI for easily managing Go tool dependencies.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fatal.ShowErrorDetail = rootOpts.verbose
		if rootOpts.verbose {
			logger.SetLevel(logrus.DebugLevel)
		}
		logger.SetFormatter(&logrus.TextFormatter{
			DisableTimestamp: true,
		})

		var err error
		shed, err = client.NewShed(client.Options{
			Logger: logger,
		})
		if err != nil {
			fatal.ExitErrf(err, "Failed to setup shed")
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&rootOpts.verbose, "verbose", false, "enable verbose logging")
}

// Execute runs the shed CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fatal.ExitErrf(err, "Failed executing command.")
	}
}
