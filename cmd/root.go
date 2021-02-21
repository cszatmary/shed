package cmd

import (
	"os"

	"github.com/getshiphub/shed/client"
	"github.com/getshiphub/shed/internal/util"
	"github.com/mattn/go-isatty"
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
	fatal    = util.Fatal{}
)

var rootCmd = &cobra.Command{
	Use:     "shed",
	Version: version,
	Short:   "shed is a CLI for easily managing Go tool dependencies.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fatal.ShowErrorDetail = rootOpts.verbose
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

func mustShed(opts ...client.Option) *client.Shed {
	shed, err := client.NewShed(opts...)
	if err != nil {
		fatal.ExitErrf(err, "Failed to setup shed")
	}
	return shed
}

func newLogger() *logrus.Logger {
	level := logrus.InfoLevel
	if rootOpts.verbose {
		level = logrus.DebugLevel
	}
	return &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			DisableTimestamp: true,
			// Need to force colours since the decision of whether or not to use colour
			// is made lazily the first time a log is written, and Out may be changed
			// to a spinner before then.
			ForceColors: isatty.IsTerminal(os.Stderr.Fd()),
		},
		Hooks: make(logrus.LevelHooks),
		Level: level,
	}
}
