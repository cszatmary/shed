package cmd

import (
	"github.com/TouchBistro/goutils/fatal"
	"github.com/cszatmary/shed/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// TODO(@cszatmary): Set version on build
var version = "0.0.0-volatile"

type rootOptions struct {
	verbose bool
}

var (
	rootOpts rootOptions
	shed     *client.Shed
	logger   = logrus.New()
)

var rootCmd = &cobra.Command{
	Use:     "shed",
	Version: version,
	Short:   "shed is a CLI for easily managing Go tool dependencies.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fatal.ShowStackTraces(rootOpts.verbose)
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
			fatal.ExitErr(err, "Failed to setup shed")
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&rootOpts.verbose, "verbose", false, "enable verbose logging")
}

// Execute runs the shed CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fatal.ExitErr(err, "Failed executing command.")
	}
}

// Root returns the root CLI command.
func Root() *cobra.Command {
	return rootCmd
}
