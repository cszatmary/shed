package cmd

import (
	"errors"
	"os"
	"os/exec"

	"github.com/getshiphub/shed/client"
	"github.com/getshiphub/shed/lockfile"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
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
	Run: func(cmd *cobra.Command, args []string) {
		toolName := args[0]
		logger := newLogger()
		setwd(logger)
		shed := mustShed(client.WithLogger(logger))
		binPath, err := shed.ToolPath(toolName)
		if errors.Is(err, lockfile.ErrNotFound) {
			fatal.Exitf("No tool named %s installed. Run 'shed get' first to install the tool.", toolName)
		} else if errors.Is(err, lockfile.ErrMultipleTools) {
			fatal.Exitf("Multiple tools named %s found. Specify the full import path of the tool in order to run it.", toolName)
		} else if err != nil {
			fatal.ExitErrf(err, "Failed to run tool %s", toolName)
		}

		logger.WithFields(logrus.Fields{
			"tool": toolName,
			"path": binPath,
		}).Debugf("Found path for tool")

		c := exec.Command(binPath, args[1:]...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin
		if err := c.Run(); err != nil {
			code := c.ProcessState.ExitCode()
			if code != -1 {
				os.Exit(code)
			}
			os.Exit(1)
		}
	},
}

func init() {
	// Stop parsing flags after first non-flag arg
	// so we can pass them to the command being run
	runCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(runCmd)
}
