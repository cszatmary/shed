package cmd

import (
	"fmt"

	"github.com/cszatmary/shed/internal/spinner"
	"github.com/cszatmary/shed/tool"
	"github.com/spf13/cobra"
)

func newGetCommand(c *container) *cobra.Command {
	return &cobra.Command{
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

Examples:

Install the latest version of a tool:

	shed get golang.org/x/tools/cmd/stringer

Install a specific version of a tool:

	shed get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0

Install all tools specified in shed.lock:

	shed get

Uninstall a tool:

	shed get golang.org/x/tools/cmd/stringer@none`,
		RunE: func(cmd *cobra.Command, args []string) error {
			installSet, err := c.shed.Get(args...)
			if err != nil {
				return fmt.Errorf("unable to determine list of tools to install: %w", err)
			}

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
}
