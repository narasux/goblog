package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/logging"
)

var (
	db         *gorm.DB
	dbInitOnce sync.Once
)

const (
	// string 类型字段的默认长度
	defaultStringSize = 256
	// 默认批量创建数量
	defaultBatchSize = 100
	// 默认最大空闲连接
	defaultMaxIdleConns = 20
	// 默认最大连接数
	defaultMaxOpenConns = 100
)

// Client 获取数据库客户端
func Client(ctx context.Context) *gorm.DB {
	if db == nil {
		log.Fatal("database client not init")
	}
	// 设置上下文目的：让 slogGorm 记录日志时带上 Request ID
	return db.WithContext(ctx)
}

// InitDBClient 初始化数据库客户端
func InitDBClient(ctx context.Context) {
	if db != nil {
		return
	}
	dbInitOnce.Do(func() {
		dbInfo := fmt.Sprintf("mysql %s:%s/%s", envs.MysqlHost, envs.MysqlPort, envs.MysqlDatabase)

		var err error
		if db, err = newClient(ctx); err != nil {
			log.Fatalf("failed to connect database %s: %s", dbInfo, err)
		} else {
			logging.GetSystemLogger().Infof("database: %s connected", dbInfo)
		}
	})
}

// 初始化 DB Client
func newClient(ctx context.Context) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=true",
		envs.MysqlUser,
		envs.MysqlPassword,
		envs.MysqlHost,
		envs.MysqlPort,
		envs.MysqlDatabase,
		envs.MysqlCharSet,
	)

	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(defaultMaxIdleConns)
	sqlDB.SetMaxOpenConns(defaultMaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	cCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 检查 DB 是否可用
	if err = sqlDB.PingContext(cCtx); err != nil {
		return nil, err
	}

	mysqlCfg := mysql.Config{
		DSN:                       dsn,
		DefaultStringSize:         defaultStringSize,
		SkipInitializeWithVersion: false,
	}

	gormCfg := &gorm.Config{
		ConnPool: sqlDB,
		// 禁用默认事务（需要手动管理）
		SkipDefaultTransaction: true,
		// 缓存预编译语句
		PrepareStmt: true,
		// Mysql 本身即不支持嵌套事务
		DisableNestedTransaction: true,
		// 批量操作数量
		CreateBatchSize: defaultBatchSize,
		// 数据库迁移时，忽略外键约束
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	client, err := gorm.Open(mysql.New(mysqlCfg), gormCfg)
	if err != nil {
		return nil, err
	}

	return client, nil
}

var (
	migSet         *migrationSet
	migSetInitOnce sync.Once
)

// 初始化数据库迁移集
func getMigrationSet() *migrationSet {
	migSetInitOnce.Do(func() {
		migSet = &migrationSet{
			mapping: map[string]*gormigrate.Migration{},
		}
	})
	return migSet
}

// RegisterMigration 注册迁移文件
func RegisterMigration(m *gormigrate.Migration) {
	if err := getMigrationSet().register(m); err != nil {
		log.Fatalf("failed to register migration: %s", err)
	}
}
