package cmd

import (
	"fmt"

	"github.com/getshiphub/shed/client"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List Go tools specified in shed.lock.",
	Long:  `shed list prints a list of tools specified in shed.lock. Each tool will consist of the import path and the version.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := newLogger()
		setwd(logger)
		shed := mustShed(client.WithLogger(logger))
		tools := shed.List()
		for _, t := range tools {
			fmt.Println(t)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
