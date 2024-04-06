package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/narasux/goblog/pkg/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "version show goblog version info.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.GetVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
