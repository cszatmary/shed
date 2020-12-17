package cmd

import (
	"errors"
	"os"
	"os/exec"

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
In order to pass flags to the tool, you must preceed them with '--'. This signifies to
shed that these flags are meant to be treated as arguments for the tool, and not flags for shed.

For example to run the stringer tool you can either run:

	shed run stringer -- -type=Pill

Or:

	shed run golang.org/x/tools/cmd/stringer -- -type=Pill`,
	Run: func(cmd *cobra.Command, args []string) {
		toolName := args[0]
		binPath, err := shed.ToolPath(toolName)
		if errors.Is(err, lockfile.ErrNotFound) {
			fatal.Exitf("No tool named %s installed. Run 'shed install' first to install the tool.", toolName)
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
	rootCmd.AddCommand(runCmd)
}
