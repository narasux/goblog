package cmd

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/logging"
	// load migration package to register migrations
	_ "github.com/narasux/goblog/pkg/migration"
	"github.com/narasux/goblog/pkg/version"
)

// NewMigrateCmd ...
func NewMigrateCmd() *cobra.Command {
	var migrationID string

	migrateCmd := cobra.Command{
		Use:   "migrate",
		Short: "Apply migrations to the database tables.",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			logging.InitLogger()
			database.InitDBClient(ctx)

			if err := database.RunMigrate(ctx, migrationID); err != nil {
				log.Fatalf("failed to run migrate: %s", err)
			}
			dbVersion, err := database.Version(ctx)
			if err != nil {
				log.Fatalf("failed to get database version: %s", err)
			}
			logging.GetSystemLogger().Infof("migrate success %s\nDatabaseVersion: %s", version.GetVersion(), dbVersion)
		},
	}

	migrateCmd.Flags().StringVar(&migrationID, "migration", "", "migration to apply, blank means latest version")

	return &migrateCmd
}

func init() {
	rootCmd.AddCommand(NewMigrateCmd())
}
