package cmd

import (
	"fmt"

	"github.com/cszatmary/shed/client"
	"github.com/spf13/cobra"
)

func newListCommand(c *container) *cobra.Command {
	var listOpts struct {
		showUpdates bool
		concurrency int
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List Go tools specified in shed.lock.",
		Long: `shed list prints a list of tools specified in shed.lock. Each tool will consist of the import path and the version.

The '-u, --updates' flag causes shed to list information about available upgrades for each tool.
If a newer version is found for a tool, shed will print it in brackets after the current version.

For example, 'shed list -u' might print:

	golang.org/x/tools/cmd/stringer v0.1.0 [v0.1.5]`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listOpts.concurrency < 0 {
				return &exitError{
					code: 1,
					msg:  "Concurrency value must be a positive integer.",
					err:  fmt.Errorf(`invalid value %d for concurrency flag`, listOpts.concurrency),
				}
			}

			tools, err := c.shed.List(cmd.Context(), client.ListOptions{
				ShowUpdates: listOpts.showUpdates,
				Concurrency: uint(listOpts.concurrency),
			})
			if err != nil {
				return err
			}
			for _, info := range tools {
				if info.LatestVersion != "" {
					fmt.Printf("%s %s [%s]\n", info.Tool.ImportPath, info.Tool.Version, info.LatestVersion)
					continue
				}
				fmt.Printf("%s %s\n", info.Tool.ImportPath, info.Tool.Version)
			}
			return nil
		},
	}

	listCmd.Flags().BoolVarP(&listOpts.showUpdates, "updates", "u", false, "show latest available version for each tool")
	listCmd.Flags().IntVarP(&listOpts.concurrency, "concurrency", "c", 0, "amount of tasks to run concurrently (default: number of CPUs)")
	return listCmd
}
