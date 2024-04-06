package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "goblog",
	Short: "goblog is narasux's blog.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("welcome to use goblog, use `goblog -h` for help")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
