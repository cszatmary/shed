package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/getshiphub/shed/client"
	"github.com/getshiphub/shed/lockfile"
	"github.com/spf13/cobra"
)

func newInitCommand(c *container) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Args:  cobra.NoArgs,
		Short: "Generate a lockfile in the current directory.",
		Long: `shed init initializes shed in the current directory by creating a lockfile.
In most cases this isn't necessary as shed will automatically create a lockfile when shed get is run.

In some situations however, it may be desirable to explicitly create the lockfile. One reason for this is to
setup shed in a subdirectory of a project. shed will automatically check parent directories for lockfiles.
If you wish to have shed get update a lockfile in a subdirectory instead of a parent directory,
you can use shed init to create a new lockfile.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.OpenFile(client.LockfileName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if errors.Is(err, os.ErrExist) {
				c.logger.Infof("%s already exists", client.LockfileName)
				return nil
			}
			if err != nil {
				return fmt.Errorf("failed to create lockfile: %w", err)
			}
			defer f.Close()

			var lf lockfile.Lockfile
			if _, err := lf.WriteTo(f); err != nil {
				return fmt.Errorf("failed to write lockfile: %w", err)
			}
			c.logger.Infof("Created %s", client.LockfileName)
			return nil
		},
	}
}
