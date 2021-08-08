package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cszatmary/shed/lockfile"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newRunCommand(c *container) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run <tool> [args...]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Run installed tools.",
		Long: `shed run runs an installed tool passing all arguments to it.

The tool name can either be the full import path or the binary name if it is unique.
All arguments after the tool name will be passed to the tool as is, even if they are flags.
That is, 'shed run --verbose foo' is treated differently than 'shed run foo --verbose'.
The first treats the '--verbose' flag as belonging to shed, the second treats it as belonging
to the 'foo' tool.

For example to run the stringer tool you can either run:

	shed run stringer -type=Pill

Or:

	shed run golang.org/x/tools/cmd/stringer -type=Pill`,
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]
			binPath, err := c.shed.ToolPath(toolName)
			// Handle special cases that are specific to run as they would be difficult for the global error handler to deal with.
			if errors.Is(err, lockfile.ErrNotFound) {
				return &exitError{
					code: 1,
					msg:  fmt.Sprintf("No tool named %s installed. Run 'shed get' first to install the tool.", toolName),
				}
			}
			if errors.Is(err, lockfile.ErrMultipleTools) {
				return &exitError{
					code: 1,
					msg:  fmt.Sprintf("Multiple tools named %s found. Specify the full import path of the tool in order to run it.", toolName),
				}
			}
			if err != nil {
				return err
			}
			c.logger.WithFields(logrus.Fields{
				"tool": toolName,
				"path": binPath,
			}).Debugf("Found path for tool")

			ec := exec.Command(binPath, args[1:]...)
			ec.Dir = filepath.Dir(c.opts.lockfilePath)
			ec.Stdout = os.Stdout
			ec.Stderr = os.Stderr
			ec.Stdin = os.Stdin
			if err := ec.Run(); err != nil {
				code := ec.ProcessState.ExitCode()
				if code != -1 {
					os.Exit(code)
				}
				os.Exit(1)
			}
			return nil
		},
	}

	// Stop parsing flags after first non-flag arg so we can pass them to the command being run
	runCmd.Flags().SetInterspersed(false)
	return runCmd
}
