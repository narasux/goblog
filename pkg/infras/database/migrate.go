package database

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/narasux/goblog/pkg/logging"
)

// 迁移文件 ID（无需过度严格，只要确保大概格式对即可）
var migrationIDRegex = regexp.MustCompile("^20[0-9]{6}_[0-9]{6}$")

// GenMigrationID 生成新的 Migration ID
func GenMigrationID() string {
	return time.Now().Format("20060102_150405")
}

const (
	migrationTableName    = "gorm_migrations"
	migrationIDColumnName = "id"
	migrationIDColumnSize = 15
)

// migration 数据库表结构
type gormMigration struct {
	ID string `gorm:"primaryKey"`
}

// Version 从 migrations 表中获取数据库版本
func Version(ctx context.Context) (string, error) {
	var mig gormMigration

	// 检查表是否存在，不存在则直接返回
	if !Client(ctx).Migrator().HasTable(migrationTableName) {
		return "", nil
	}

	if err := Client(ctx).
		Table(migrationTableName).
		Not(fmt.Sprintf("%s = ?", migrationIDColumnName), "SCHEMA_INIT").
		Order(fmt.Sprintf("%s desc", migrationIDColumnName)).
		First(&mig).Error; err != nil {
		return "", err
	}
	return mig.ID, nil
}

// RunMigrate 根据模型对数据库执行迁移到指定版本，传入空字符串表示迁移到最新版本
func RunMigrate(ctx context.Context, migrationID string) error {
	opts := gormigrate.Options{
		TableName:    migrationTableName,
		IDColumnName: migrationIDColumnName,
		IDColumnSize: migrationIDColumnSize,
	}
	m := gormigrate.New(Client(ctx), &opts, getMigrationSet().values())

	logger := logging.GetSystemLogger()

	curVersion, err := Version(ctx)
	if err != nil {
		return errors.Wrap(err, "get current database version")
	}
	logger.Infof("current database version: %s", curVersion)

	// 无法获取当前 DB 版本 或 未指定迁移版本，则默认迁移到最新版本
	if curVersion == "" || migrationID == "" {
		logger.Info("migrate to latest version")
		return m.Migrate()
	}

	if curVersion > migrationID {
		logger.Warnf("rollback to version: %s", migrationID)
		return m.RollbackTo(migrationID)
	}
	logger.Infof("migrate to version: %s", migrationID)
	return m.MigrateTo(migrationID)
}

// 迁移集
type migrationSet struct {
	mapping map[string]*gormigrate.Migration
}

func (ms *migrationSet) register(migration *gormigrate.Migration) error {
	if !migrationIDRegex.MatchString(migration.ID) {
		return errors.Errorf(
			"Invalid migration ID: %s. Do not modify the ID generated by the make-migration command!",
			migration.ID,
		)
	}
	if _, ok := ms.mapping[migration.ID]; ok {
		return errors.Errorf(
			"Migration %s is already registered. Please confirm if another migration shares the same ID?",
			migration.ID,
		)
	}
	ms.mapping[migration.ID] = migration
	return nil
}

func (ms *migrationSet) values() []*gormigrate.Migration {
	// 按照 ID（生成时间）进行升序排序
	migrations := lo.Values(ms.mapping)
	slices.SortFunc(migrations, func(x, y *gormigrate.Migration) int {
		return strings.Compare(x.ID, y.ID)
	})
	return migrations
}
