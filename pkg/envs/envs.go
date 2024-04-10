package envs

import (
	"path/filepath"

	"github.com/narasux/goblog/pkg/common/runmode"
	"github.com/narasux/goblog/pkg/utils/envx"
	"github.com/narasux/goblog/pkg/utils/pathx"
)

// 以下变量值可通过环境变量指定
var (
	// Domain 服务域名
	Domain = envx.Get("DOMAIN", "www.narasux.cn")

	// DomainScheme 服务域名协议
	DomainScheme = envx.Get("DOMAIN_SCHEME", "https")

	// ServerPort web 服务启用端口
	ServerPort = envx.Get("SERVER_PORT", "8080")

	// GinRunMode web 服务运行模式
	GinRunMode = envx.Get("GIN_RUN_MODE", runmode.Release)

	// TmplFileBaseDir
	TmplFileBaseDir = envx.Get("TMPL_FILE_BASE_DIR", filepath.Join(pathx.GetCurPKGPath(), "../../templates"))

	// StaticFileBaseDir
	StaticFileBaseDir = envx.Get("STATIC_FILE_BASE_DIR", filepath.Join(pathx.GetCurPKGPath(), "../../static"))

	// BlogDataBaseDir 博客文章内容存放目录
	BlogDataBaseDir = envx.Get("BLOG_DATA_BASE_DIR", filepath.Join(pathx.GetCurPKGPath(), "../../data"))

	// LogFileBaseDir 日志存放目录
	LogFileBaseDir = envx.Get("LOG_FILE_BASE_DIR", filepath.Join(pathx.GetCurPKGPath(), "../../logs"))

	// LogLevel 日志等级（panic/fatal/error/warn/info/debug/trace）
	LogLevel = envx.Get("LOG_LEVEL", "warn")

	// ContactEmail 联系邮箱
	ContactEmail = envx.Get("CONTACT_EMAIL", "suzh9@mail2.sysu.edu.cn")

	// RealClientIPHeaderKey Header 中真实客户端 IP 键（适用于类似 Nginx 转发的情况）为空则使用默认的 ClientIP
	RealClientIPHeaderKey = envx.Get("REAL_CLIENT_IP_HEADER_KEY", "")
)
