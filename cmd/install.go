package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"github.com/getshiphub/shed/client"
	"github.com/getshiphub/shed/internal/spinner"
	"github.com/getshiphub/shed/tool"
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
	Long: `shed install installs the given tools plus all tools specified in a shed.lock file.
After installing the tools, shed will either update the existing shed.lock file in the current directory,
or create a new shed.lock if one does not exist. The shed.lock file is responsible for keeping track of
what tools are installed and their verion. This allows shed to always reinstall the same tools.

Each tool provided must be the full import path to the package containing the main executable.
The format is identical to what would be passed to 'go get'. Tools may specify a version by prefixing it with
an '@', just like with 'go get' in module-aware mode. If no version is provided, the latest version will be installed.

If no tools are provided, then shed will simply install all tools in the lockfile.

By default shed prevents installing a tool if a different version of the same tool already exists in the shed.lock file.
If you wish to update the tool, the '-u' or '--update' flag can be used. This will cause shed to install the new version
of the tool specified and update the shed.lock file.

Examples:

Install the latest version of a tool:

	shed install golang.org/x/tools/cmd/stringer

Install a specific version of a tool:

	shed install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0

Install all tools specified in shed.lock:

	shed install`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := newLogger()
		setwd(logger)
		shed := mustShed(client.WithLogger(logger))
		installSet, err := shed.Install(installOpts.allowUpdates, args...)
		if err != nil {
			fatal.ExitErrf(err, "Failed to determine list of tools to install")
		}

		s := spinner.NewTTY(spinner.Options{
			Message:         "Installing tools",
			Count:           installSet.Len(),
			PersistMessages: rootOpts.verbose,
		})
		logger.Out = s
		ch := make(chan tool.Tool, installSet.Len())
		installSet.Notify(ch)
		go func() {
			for range ch {
				s.Inc()
			}
		}()

		// Listen of SIGINT to do a graceful abort
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		abort := make(chan os.Signal, 1)
		signal.Notify(abort, os.Interrupt)
		go func() {
			<-abort
			cancel()
		}()

		s.Start()
		err = installSet.Apply(ctx)
		s.Stop()
		close(ch)
		logger.Out = os.Stderr

		if errors.Is(err, context.Canceled) {
			logger.Info("Install aborted")
			return
		}
		if err != nil {
			fatal.ExitErrf(err, "Failed to install tools")
		}
		logger.Info("Finished installing tools")
	},
}

func init() {
	installCmd.Flags().BoolVarP(&installOpts.allowUpdates, "update", "u", false, "allow updating already installed tools")
	rootCmd.AddCommand(installCmd)
}
