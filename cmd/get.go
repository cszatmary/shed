package cmd

import (
	"fmt"

	"github.com/cszatmary/shed/client"
	"github.com/cszatmary/shed/internal/spinner"
	"github.com/cszatmary/shed/tool"
	"github.com/spf13/cobra"
)

func newGetCommand(c *container) *cobra.Command {
	var getOpts struct {
		update      bool
		concurrency int
	}

	getCmd := &cobra.Command{
		Use:   "get [tools...]",
		Args:  cobra.ArbitraryArgs,
		Short: "Install Go tools.",
		Long: `shed get installs the given tools plus all tools specified in a shed.lock file.
After installing the tools, shed will either update the existing shed.lock file in the current directory,
or create a new shed.lock if one does not exist. The shed.lock file is responsible for keeping track of
what tools are installed and their verion. This allows shed to always reinstall the same tools.

Each tool provided must be the full import path to the package containing the main executable.
The format is identical to what would be passed to 'go get'. Tools may specify a version by suffixing it with
an '@', just like with 'go get' in module-aware mode. If no version is provided, the latest version will be installed.

Tools can be uninstalled by using the special '@none' version suffix.

If no tools are provided, then shed will simply install all tools in the lockfile.

The '-u, --update' flag instructs get to update the provided tools to use newer minor or patch releases when available.
If no tools are provided, all tools in the lockfile will be updated. When this flag is used, tools are not allowed
to have a version suffix.

Examples:

Install the latest version of a tool:

	shed get golang.org/x/tools/cmd/stringer

Install a specific version of a tool:

	shed get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0

Install all tools specified in shed.lock:

	shed get

Uninstall a tool:

	shed get golang.org/x/tools/cmd/stringer@none

Update a specific tool to the latest minor or patch version:

	shed get -u golang.org/x/tools/cmd/stringer

Update all tools in the lockfile to their latest minor or patch version:

	shed get -u`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if getOpts.concurrency < 0 {
				return &exitError{
					code: 1,
					msg:  "Concurrency value must be a positive integer.",
					err:  fmt.Errorf(`invalid value %d for concurrency flag`, getOpts.concurrency),
				}
			}

			installSet, err := c.shed.Get(client.GetOptions{
				ToolNames: args,
				Update:    getOpts.update,
			})
			if err != nil {
				return fmt.Errorf("unable to determine list of tools to install: %w", err)
			}
			installSet.Concurrency = uint(getOpts.concurrency)

			s := spinner.NewTTY(spinner.TTYOptions{
				Options: spinner.Options{
					Message:         "Installing tools",
					Count:           installSet.Len(),
					PersistMessages: c.opts.verbose,
				},
				IsaTTY: c.isaTTY,
			})
			prevOut := c.logger.Out
			c.logger.Out = s

			ch := make(chan tool.Tool, installSet.Len())
			installSet.Notify(ch)
			go func() {
				for range ch {
					s.Inc()
				}
			}()

			s.Start()
			err = installSet.Apply(cmd.Context())
			s.Stop()
			close(ch)
			c.logger.Out = prevOut

			if err != nil {
				return fmt.Errorf("failed to install tools: %w", err)
			}
			c.logger.Info("Finished installing tools")
			return nil
		},
	}

	getCmd.Flags().BoolVarP(&getOpts.update, "update", "u", false, "update tools to their latest minor or patch version")
	getCmd.Flags().IntVarP(&getOpts.concurrency, "concurrency", "c", 0, "amount of tasks to run concurrently (default: number of CPUs)")
	return getCmd
}
