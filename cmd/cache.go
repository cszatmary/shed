package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage cached tools.",
	Long: `shed cache manages the cache that contains installed tools.

'shed cache dir' can be used to print the path to the shed cache.
'shed cache clean' can be used to clean the cache and remove all tools.`,
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Cleans the shed cache.",
	Long: `Cleans the shed cache by removing all installed tools.
This is useful for removing any stale tools that are no longer needed.`,
	Run: func(cmd *cobra.Command, args []string) {
		shed := mustShed()
		if err := shed.CleanCache(); err != nil {
			fatal.ExitErrf(err, "Failed to clean cache directory")
		}
	},
}

var cacheDirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Prints the path to the shed cache directory.",
	Long:  `Prints the absolute path to the root shed cache directory where tools are installed.`,
	Run: func(cmd *cobra.Command, args []string) {
		shed := mustShed()
		fmt.Println(shed.CacheDir())
	},
}

func init() {
	cacheCmd.AddCommand(cacheCleanCmd)
	cacheCmd.AddCommand(cacheDirCmd)
	rootCmd.AddCommand(cacheCmd)
}
