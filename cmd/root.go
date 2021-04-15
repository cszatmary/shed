package cmd

import (
	"os"
	"path/filepath"

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

// setwd finds the nearest shed lockfile in either the current directory
// or parent directories and changes the current working directory.
func setwd(logger *logrus.Logger) {
	cwd, err := os.Getwd()
	if err != nil {
		fatal.ExitErrf(err, "Failed to get current working directory")
	}
	lfp := client.ResolveLockfilePath(cwd)
	if lfp == "" {
		return
	}

	logger.Debugf("Found lockfile: %s", lfp)
	dir := filepath.Dir(lfp)
	if err := os.Chdir(dir); err != nil {
		fatal.ExitErrf(err, "Failed to change current working directory to %s", dir)
	}
	logger.Debugf("Changed current working directory to %s", dir)
}
