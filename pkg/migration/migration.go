// Package migration stores all database migrations
package migration

import (
	"github.com/narasux/goblog/pkg/logging"
)

// 打印执行迁移的日志
func logApplying(migrationID string) {
	logging.GetSystemLogger().Infof("Applying migration %s", migrationID)
}

// 打印回滚迁移的日志
func logRollingBack(migrationID string) {
	logging.GetSystemLogger().Infof("Rolling back migration %s", migrationID)
}
