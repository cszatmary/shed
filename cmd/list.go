package cmd

import (
	"context"
	"fmt"

	"github.com/getshiphub/shed/client"
	"github.com/spf13/cobra"
)

var listOpts struct {
	showUpdates bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List Go tools specified in shed.lock.",
	Long: `shed list prints a list of tools specified in shed.lock. Each tool will consist of the import path and the version.

The --upgrades or -u flag causes shed to list information about available upgrades for each tool.
If a newer version is found for a tool, shed will print it in brackets after the current version.

For example, 'shed list -u' might print:

	golang.org/x/tools/cmd/stringer v0.1.0 [v0.1.5]`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := newLogger()
		setwd(logger)
		shed := mustShed(client.WithLogger(logger))
		// TODO(@cszatmary): Handle SIGINT
		tools, err := shed.List(context.Background(), client.ListOptions{
			ShowUpdates: listOpts.showUpdates,
		})
		if err != nil {
			// TODO(@cszatmary): Improve error handling
			fatal.ExitErrf(err, "Failed to list tools")
		}
		for _, info := range tools {
			if info.LatestVersion != "" {
				fmt.Printf("%s %s [%s]\n", info.Tool.ImportPath, info.Tool.Version, info.LatestVersion)
				continue
			}
			fmt.Printf("%s %s\n", info.Tool.ImportPath, info.Tool.Version)
		}
	},
}

func init() {
	listCmd.Flags().BoolVarP(&listOpts.showUpdates, "updates", "u", false, "show latest available version for each tool")
	rootCmd.AddCommand(listCmd)
}
