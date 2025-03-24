package cmd

import (
	"context"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/logging"
	"github.com/narasux/goblog/pkg/router"
	"github.com/narasux/goblog/pkg/storage"
)

var webServerCmd = &cobra.Command{
	Use:   "webserver",
	Short: "webserver start http server.",
	Run: func(cmd *cobra.Command, args []string) {
		logging.InitLogger()
		storage.InitBlogData()
		database.InitDBClient(context.Background())

		color.Green("Starting server at http://0.0.0.0:%s/", envs.ServerPort)
		router.InitRouter()
	},
}

func init() {
	rootCmd.AddCommand(webServerCmd)
}
