package cmd

import (
	"os"

	"github.com/getshiphub/shed/client"
	"github.com/getshiphub/shed/internal/util"
	"github.com/getshiphub/shed/lockfile"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Args:  cobra.NoArgs,
	Short: "Generate a lockfile in the current directory.",
	Long: `shed init initializes shed in the current directory by creating a lockfile.
In most cases this isn't necessary as shed will automatically create a lockfile when shed get is run.

In some situations however, it may be desirable to explicitly create the lockfile. One reason for this is to
setup shed in a subdirectory of a project. shed will automatically check parent directories for lockfiles.
If you wish to have shed get update a lockfile in a subdirectory instead of a parent directory,
you can use shed init to create a new lockfile.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := newLogger()
		if util.FileOrDirExists(client.LockfileName) {
			logger.Infof("%s already exists", client.LockfileName)
			return
		}

		f, err := os.OpenFile(client.LockfileName, os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			fatal.ExitErrf(err, "Failed to create file %s", client.LockfileName)
		}
		defer f.Close()
		var lf lockfile.Lockfile
		if _, err = lf.WriteTo(f); err != nil {
			fatal.ExitErrf(err, "Failed to write lockfile")
		}
		logger.Infof("Created %s", client.LockfileName)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
