// Package migration stores all database migrations
package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/model"
)

func init() {
	// Do Not Edit Migration ID!
	migrationID := "20250322_123456"

	database.RegisterMigration(&gormigrate.Migration{
		ID: migrationID,
		Migrate: func(tx *gorm.DB) error {
			logApplying(migrationID)

			return tx.AutoMigrate(&model.ViewRecord{}, &model.LikeRecord{})
		},
		Rollback: func(tx *gorm.DB) error {
			logRollingBack(migrationID)

			return tx.Migrator().DropTable(&model.ViewRecord{}, &model.LikeRecord{})
		},
	})
}
