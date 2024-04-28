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

	// ========== 以下 MyGo 配置有助于你的网站出现在 Google、Baidu 的搜索结果中 ==========

	// GoogleSiteVerificationCode Google 网站所有权验证码（HTML 标签验证方式）
	// 访问 https://search.google.com/search-console 添加资源，类型选网址前缀，
	// 验证方式选 其他 -> HTML 标签，即可获取 Code（content 内容），仅用于验证所有权，不敏感
	GoogleSiteVerificationCode = envx.Get("GOOGLE_SITE_VERIFICATION_CODE", "")

	// BaiduSiteVerificationCode Baidu 网站所有权验证码（HTML 标签验证方式）
	// 访问 https://ziyuan.baidu.com/site/index#/ 添加网站，走到第三步，
	// 验证方式选 HTML 标签即可获取 Code（content 内容），仅用于验证所有权，不敏感
	BaiduSiteVerificationCode = envx.Get("BAIDU_SITE_VERIFICATION_CODE", "")
)
