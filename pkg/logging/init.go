package logging

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/narasux/goblog/pkg/envs"
)

var initOnce sync.Once

// 注：虽然 zap 性能更好，不过 logrus 展示效果较好
// 作为一个博客网站，性能要求应该不会很高，先用 logrus 吧

// 访问日志
var accessLogger *logrus.Logger

// web 页面日志（Handler...)
var webLogger *logrus.Logger

// sql 日志（暂时用不上，等接入 DB 再说）
var sqlLogger *logrus.Logger

const (
	LogTypeSystem = "system"
	LogTypeAccess = "access"
	LogTypeWeb    = "web"
	LogTypeSql    = "sql"
)

func InitLogger() {
	initSystemLogger()

	initOnce.Do(func() {
		accessLogger = newJsonLogger(LogTypeAccess)
		webLogger = newJsonLogger(LogTypeWeb)
		sqlLogger = newJsonLogger(LogTypeSql)
	})
}

func GetSystemLogger() *logrus.Logger {
	return logrus.StandardLogger()
}

func GetAccessLogger() *logrus.Logger {
	if accessLogger == nil {
		return GetSystemLogger()
	}
	return accessLogger
}

func GetWebLogger() *logrus.Logger {
	if webLogger == nil {
		return GetSystemLogger()
	}
	return webLogger
}

func GetSqlLogger() *logrus.Logger {
	if sqlLogger == nil {
		return GetSystemLogger()
	}
	return sqlLogger
}

func initSystemLogger() {
	// 设置日志输出
	writer, err := getWriter(LogTypeSystem)
	if err != nil {
		panic(err)
	}
	logrus.SetOutput(writer)

	// 设置日志格式
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		FullTimestamp:   true,
		TimestampFormat: time.DateTime,
	})

	// 设置日志级别
	level, err := logrus.ParseLevel(envs.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
}

func newJsonLogger(logType string) *logrus.Logger {
	logger := logrus.New()
	// 设置日志输出
	writer, err := getWriter(logType)
	if err != nil {
		panic(err)
	}
	logger.SetOutput(writer)

	// 设置日志格式
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.DateTime,
		PrettyPrint:     false,
	})

	// 设置日志级别
	level, err := logrus.ParseLevel(envs.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	return logger
}
