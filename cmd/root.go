package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/cszatmary/shed/client"
	"github.com/cszatmary/shed/errors"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

// Set by goreleaser when release build is created.
var version string

// Execute runs the shed CLI.
func Execute() {
	var c container
	rootCmd := newRootCommand(&c)

	// Listen of SIGINT to do a graceful abort
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	abort := make(chan os.Signal, 1)
	signal.Notify(abort, os.Interrupt)
	go func() {
		<-abort
		cancel()
	}()

	// Check that go is installed with the minimum required version
	output, err := exec.CommandContext(ctx, "go", "version").Output()
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "\nOperation cancelled")
		os.Exit(130)
	}
	if err != nil {
		c.exitf(err, "Failed to check Go version. Make sure Go 1.11 or later is installed and in your PATH.")
	}
	regex := regexp.MustCompile(`go?([0-9]+(?:\.[0-9]+)?(?:\.[0-9]+)?)`)
	matches := regex.FindSubmatch(output)
	if len(matches) != 2 {
		c.exitf(nil, "Unexpected go version format %s, unable to parse", output)
	}
	goVersion := string(matches[1])
	// The semver package requires strings to be prefixed with 'v' to be considered valid
	if semver.Compare("v"+goVersion, "v1.11") == -1 {
		c.exitf(nil, "shed requires a minimum Go version of 1.11 to run, current version is %s", goVersion)
	}

	cmd, err := rootCmd.ExecuteContextC(ctx)
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "\nOperation cancelled")
		os.Exit(130)
	}
	if ee, ok := err.(*exitError); ok {
		fmt.Fprintln(os.Stderr, ee.msg)
		os.Exit(ee.code)
	}
	if rootErr := errors.Root(err); rootErr != nil {
		// Determine a message to show the user to offer help/suggestions.
		var msg string
		switch rootErr.Kind {
		case errors.Invalid:
			msg = fmt.Sprintf(
				"Please address the issue and retry the operation.\nRun '%s --help' for details on command usage.",
				cmd.CommandPath(),
			)
		case errors.NotInstalled:
			msg = "Run 'shed get' to install the tool(s)."
		case errors.BadState:
			msg = "Run 'shed get' to have shed resolve the issue and then try again."
		case errors.Internal:
			msg = `This is likely a bug. Try running the command again with the '--verbose' flag for more details.
If the issue persists, consider reporting it at https://github.com/cszatmary/shed/issues.`
		case errors.IO:
			msg = fmt.Sprintf(
				`Ensure you have read and write access to the current directory and %s.
Also try re-running the command with the '--verbose' flag for more details.`,
				c.shed.CacheDir(),
			)
		case errors.Go:
			msg = "Check that your version of Go works and you are able to run commands like 'go get' and 'go build'."
		default:
			msg = `Try running the command again with the '--verbose' flag for more details.
If the issue persists, consider reporting it at https://github.com/cszatmary/shed/issues.`
		}
		c.exitf(err, msg)
	}
	if err != nil {
		c.exitf(err, "")
	}
}

// container stores all the dependencies that can be used by commands.
type container struct {
	logger *logrus.Logger
	shed   *client.Shed
	isaTTY bool
	opts   struct {
		verbose      bool
		progressMode string
		lockfilePath string
	}
}

// exitf prints the given message to stderr then exits the program.
// It supports printf like formatting. If err is not nil it is also printed.
func (c *container) exitf(err error, format string, a ...interface{}) {
	if err != nil {
		if c.opts.verbose {
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
	}
	// If an error was just printed and a message is going to be printed,
	// add an extra newline inbetween them
	if err != nil && format != "" {
		fmt.Fprintln(os.Stderr)
	}
	if format != "" {
		fmt.Fprintf(os.Stderr, format, a...)
		if !strings.HasSuffix(format, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}
	os.Exit(1)
}

// exitError is used to signal that shed should exit with a given code and message.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string {
	return e.msg
}

func newRootCommand(c *container) *cobra.Command {
	// Set version if built from source
	if version == "" {
		version = "source"
		if info, available := debug.ReadBuildInfo(); available {
			version = info.Main.Version
		}
	}

	rootCmd := &cobra.Command{
		Use:     "shed",
		Version: version,
		Short:   "shed is a CLI for easily managing Go tool dependencies.",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		// cobra prints errors returned from RunE by default. Disable that since we handle errors ourselves.
		SilenceErrors: true,
		// cobra prints command usage by default if RunE returns an error.
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var isaTTY bool
			switch c.opts.progressMode {
			case "on":
				isaTTY = true
			case "off":
				isaTTY = false
			case "auto":
				isaTTY = isatty.IsTerminal(os.Stderr.Fd())
			default:
				return fmt.Errorf("invalid progress flag value '%s', valid values are 'on', 'off' or 'auto'", c.opts.progressMode)
			}

			logger := logrus.New()
			if c.opts.verbose {
				logger.SetLevel(logrus.DebugLevel)
			}
			logger.SetFormatter(&logrus.TextFormatter{
				DisableTimestamp: true,
				// Need to force colours since the decision of whether or not to use colour
				// is made lazily the first time a log is written, and Out may be changed
				// to a spinner before then.
				ForceColors: isaTTY,
			})

			// Find the nearest shed lockfile if it exists
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("unable to get current working directory: %w", err)
			}
			lfp := client.ResolveLockfilePath(cwd)
			logger.Debugf("Found lockfile: %s", lfp)
			shed, err := client.NewShed(client.WithLogger(logger), client.WithLockfilePath(lfp))
			if err != nil {
				return fmt.Errorf("failed to setup shed: %w", err)
			}

			// Set dependencies so commands can use them
			c.logger = logger
			c.shed = shed
			c.isaTTY = isaTTY
			c.opts.lockfilePath = lfp
			return nil
		},
	}

	rootCmd.AddCommand(
		newCacheCommand(c),
		newCompletionsCommand(),
		newGetCommand(c),
		newInitCommand(c),
		newListCommand(c),
		newRunCommand(c),
	)

	rootCmd.PersistentFlags().BoolVar(&c.opts.verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&c.opts.progressMode, "progress", "auto", "sets if a progress spinner should be used, valid values: on, off, auto")
	return rootCmd
}
