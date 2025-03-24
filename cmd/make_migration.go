package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/logging"
)

var migrationTmpl = `
// Package migration stores all database migrations
package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/narasux/goblog/pkg/infras/database"
)


func init() {
	// Do Not Edit Migration ID!
	migrationID := "{{ .id }}"

	database.RegisterMigration(&gormigrate.Migration{
		ID: migrationID,
		Migrate: func(tx *gorm.DB) error {
			logApplying(migrationID)

			// TODO implement migrate code
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			logRollingBack(migrationID)

			// TODO implement rollback code
			return nil
		},
	})
}
`

var makeMigrationCmd = &cobra.Command{
	Use:   "make-migration",
	Short: "Generate an empty migration file.",
	Run: func(cmd *cobra.Command, args []string) {
		logging.InitLogger()
		logger := logging.GetSystemLogger()

		migrationID := database.GenMigrationID()

		// 文件
		fileName := fmt.Sprintf("%s.go", migrationID)
		filePath := path.Join(envs.BaseDir, "pkg/migration", fileName)
		file, err := os.Create(filePath)
		if err != nil {
			logger.Fatalf("failed to create migration file with path: %s, err: %s", filePath, err)
		}
		defer file.Close()

		// 模板
		tmpl, err := template.New("migration").Parse(strings.TrimLeft(migrationTmpl, "\n"))
		if err != nil {
			logger.Fatal("failed to initialize migration template")
		}
		if err = tmpl.Execute(file, map[string]string{"id": migrationID}); err != nil {
			logger.Fatal("failed to render migration file from template")
		}

		logger.Infof(
			"migration file %s generated, you must edit it and "+
				"implement the migration logic and then run `migrate` to apply",
			fileName,
		)
	},
}

func init() {
	rootCmd.AddCommand(makeMigrationCmd)
}
