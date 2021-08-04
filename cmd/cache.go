package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCacheCommand(c *container) *cobra.Command {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage cached tools.",
		Long:  `shed cache manages the cache that contains installed tools.`,
	}

	cacheCleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Cleans the shed cache.",
		Long: `Cleans the shed cache by removing all installed tools.
This is useful for removing any stale tools that are no longer needed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.shed.CleanCache()
		},
	}

	cacheDirCmd := &cobra.Command{
		Use:   "dir",
		Short: "Prints the path to the shed cache directory.",
		Long:  `Prints the absolute path to the root shed cache directory where tools are installed.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(c.shed.CacheDir())
		},
	}

	cacheCmd.AddCommand(cacheCleanCmd)
	cacheCmd.AddCommand(cacheDirCmd)
	return cacheCmd
}
