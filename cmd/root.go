package cmd

import (
	"github.com/TouchBistro/goutils/fatal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// TODO(@cszatmary): Set version on build
const version = "0.0.0-volatile"

type rootOptions struct {
	verbose bool
}

var rootOpts rootOptions

var rootCmd = &cobra.Command{
	Use:     "shed",
	Version: version,
	Short:   "shed is a CLI for easily managing Go tool dependencies.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if rootOpts.verbose {
			log.SetLevel(log.DebugLevel)
		}
		fatal.ShowStackTraces(rootOpts.verbose)
		// TODO(@cszatmary): We should use a custom formatter that's way simpler for non-verbose
		// Current one is too noisy
		log.SetFormatter(&log.TextFormatter{
			DisableTimestamp: true,
		})
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&rootOpts.verbose, "verbose", false, "enable verbose logging")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fatal.ExitErr(err, "Failed executing command.")
	}
}
