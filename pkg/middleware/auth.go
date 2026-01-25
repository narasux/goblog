package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/narasux/goblog/pkg/common/ctxkey"
	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/service/session"
)

// Auth 认证中间件（可选认证，不强制）
// 如果用户已登录，将用户信息存入 Context
// 如果用户未登录，也继续处理请求
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Cookie 获取 Session Cookie 值
		sessionCookie, err := c.Cookie(envs.SessionCookieName)
		if err != nil || sessionCookie == "" {
			c.Next()
			return
		}

		// 解析并验证签名
		sessionID, err := session.ParseSessionCookie(sessionCookie)
		if err != nil {
			// Cookie 签名无效，清除 Cookie
			c.SetCookie(envs.SessionCookieName, "", -1, "/", "", false, true)
			c.Next()
			return
		}

		// 验证 Session 有效性
		user, err := session.Validate(c.Request.Context(), sessionID)
		if err != nil {
			// Session 无效，清除 Cookie
			c.SetCookie(envs.SessionCookieName, "", -1, "/", "", false, true)
			c.Next()
			return
		}

		// 将用户信息存入 Context
		c.Set(ctxkey.LoginUser, user)
		c.Next()
	}
}
