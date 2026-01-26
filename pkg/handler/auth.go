package handler

import (
	"errors"
	"github.com/narasux/goblog/pkg/common/ctxkey"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/logging"
	"github.com/narasux/goblog/pkg/model"
	"github.com/narasux/goblog/pkg/service/oauth"
	"github.com/narasux/goblog/pkg/service/session"
	"github.com/narasux/goblog/pkg/utils/ginx"
	"github.com/narasux/goblog/pkg/utils/uuid"
)

const (
	// OAuth state cookie 名称
	oauthStateCookieName = "github_oauth_state"
	// OAuth state cookie 有效期（10 分钟）
	oauthStateCookieMaxAge = 600
	// OAuth redirect cookie 名称
	oauthRedirectCookieName = "github_oauth_redirect"
)

// GitHubLogin 发起 GitHub OAuth 登录
// GET /auth/github/login
func GitHubLogin(c *gin.Context) {
	// 检查 GitHub OAuth 配置
	if envs.GithubClientID == "" || envs.GithubClientSecret == "" {
		logging.GetSystemLogger().Error("GitHub OAuth not configured")
		c.Redirect(http.StatusFound, "/?error=oauth_not_configured")
		return
	}

	// 生成随机 state 防止 CSRF
	state := uuid.GenUUID4()

	// 允许跨站回调时发送 cookie
	c.SetSameSite(http.SameSiteLaxMode)
	// 将 state 存入 Cookie（短期有效）
	c.SetCookie(
		oauthStateCookieName,
		state,
		oauthStateCookieMaxAge,
		"/",
		"",
		envs.DomainScheme == "https",
		true,
	)

	// 保存来源页面，登录后跳转回去
	if referer := c.Query("redirect"); referer != "" {
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(oauthRedirectCookieName, referer, oauthStateCookieMaxAge, "/", "", envs.DomainScheme == "https", true)
	}

	// 重定向到 GitHub 授权页面
	authorizeURL := oauth.GetAuthorizeURL(state)
	c.Redirect(http.StatusFound, authorizeURL)
}

// GitHubCallback GitHub OAuth 回调处理
// GET /auth/github/callback
func GitHubCallback(c *gin.Context) {
	logger := logging.GetSystemLogger()
	ctx := c.Request.Context()

	// 获取 code 和 state
	code := c.Query("code")
	state := c.Query("state")

	// 检查是否有错误
	if errMsg := c.Query("error"); errMsg != "" {
		errDesc := c.Query("error_description")
		logger.Errorf("GitHub OAuth error: %s - %s", errMsg, errDesc)
		c.Redirect(http.StatusFound, "/?error=oauth_denied")
		return
	}

	// 验证 state 参数
	savedState, err := c.Cookie(oauthStateCookieName)
	if err != nil {
		logger.Errorf("OAuth state cookie not found: %v, expected state: %s", err, state)
		c.Redirect(http.StatusFound, "/?error=invalid_state_not_found")
		return
	}
	if savedState != state {
		logger.Errorf("OAuth state mismatch: cookie=%s, expected=%s", savedState, state)
		c.Redirect(http.StatusFound, "/?error=invalid_state_mismatch")
		return
	}

	// 清除 state cookie（必须使用相同的 Secure 属性才能正确清除）
	c.SetCookie(oauthStateCookieName, "", -1, "/", "", envs.DomainScheme == "https", true)

	// 用 code 换取 access_token
	accessToken, err := oauth.ExchangeToken(ctx, code)
	if err != nil {
		logger.Errorf("Failed to exchange token: %s", err.Error())
		c.Redirect(http.StatusFound, "/?error=token_exchange_failed")
		return
	}

	// 获取 GitHub 用户信息
	githubUser, err := oauth.GetUserInfo(ctx, accessToken)
	if err != nil {
		logger.Errorf("Failed to get GitHub user info: %s", err.Error())
		c.Redirect(http.StatusFound, "/?error=user_info_failed")
		return
	}

	// 创建或更新本地用户
	db := database.Client(ctx)
	var user model.User
	err = db.Where("github_id = ?", githubUser.ID).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新用户
		user = model.User{
			GithubID:    githubUser.ID,
			Username:    githubUser.Login,
			Email:       githubUser.Email,
			AvatarURL:   githubUser.AvatarURL,
			AccessToken: accessToken,
		}
		if err = db.Create(&user).Error; err != nil {
			logger.Errorf("Failed to create user: %s", err.Error())
			c.Redirect(http.StatusFound, "/?error=create_user_failed")
			return
		}
	} else if err != nil {
		logger.Errorf("Failed to query user: %s", err.Error())
		c.Redirect(http.StatusFound, "/?error=query_user_failed")
		return
	} else {
		// 更新用户信息
		updates := map[string]any{
			"username":     githubUser.Login,
			"email":        githubUser.Email,
			"avatar_url":   githubUser.AvatarURL,
			"access_token": accessToken,
		}
		if err = db.Model(&user).Updates(updates).Error; err != nil {
			logger.Errorf("Failed to update user: %s", err.Error())
			// 更新失败不影响登录
		}
	}

	// 创建 Session
	sess, err := session.Create(ctx, user.ID, ginx.GetClientIP(c), c.Request.UserAgent())
	if err != nil {
		logger.Errorf("Failed to create session: %s", err.Error())
		c.Redirect(http.StatusFound, "/?error=create_session_failed")
		return
	}

	// 设置 Session Cookie（使用签名后的值）
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		envs.SessionCookieName,
		session.GenerateSessionCookie(sess.ID),
		envs.SessionMaxAge,
		"/",
		"",
		envs.DomainScheme == "https",
		true,
	)

	// 获取重定向地址
	redirectURL := "/"
	if savedRedirect, err := c.Cookie(oauthRedirectCookieName); err == nil && savedRedirect != "" {
		redirectURL = savedRedirect
		c.SetCookie(oauthRedirectCookieName, "", -1, "/", "", envs.DomainScheme == "https", true)
	}

	// 重定向到原页面
	c.Redirect(http.StatusFound, redirectURL)
}

// Logout 登出
// POST /auth/logout
func Logout(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取 Session Cookie 值
	sessionCookie, err := c.Cookie(envs.SessionCookieName)
	if err == nil && sessionCookie != "" {
		// 解析签名获取真实的 Session ID
		sessionID, parseErr := session.ParseSessionCookie(sessionCookie)
		if parseErr == nil && sessionID != "" {
			// 删除 Session
			if err = session.Delete(ctx, sessionID); err != nil {
				logging.GetSystemLogger().Errorf("Failed to delete session: %s", err.Error())
			}
		}
	}

	// 清除 Cookie
	c.SetCookie(envs.SessionCookieName, "", -1, "/", "", false, true)

	// 检查是否是 API 请求
	if c.GetHeader("Accept") == "application/json" {
		ginx.SetResp(c, http.StatusNoContent, nil)
		return
	}

	// 重定向到首页
	c.Redirect(http.StatusFound, "/")
}

// GetCurrentUser 获取当前登录用户信息
// GET /apis/user
func GetCurrentUser(c *gin.Context) {
	user, exists := c.Get(ctxkey.LoginUser)
	if !exists || user == nil {
		ginx.SetErrResp(c, http.StatusUnauthorized, "not logged in")
		return
	}

	u := user.(*model.User)
	ginx.SetResp(c, http.StatusOK, map[string]any{
		"id":        u.ID,
		"username":  u.Username,
		"email":     u.Email,
		"avatarURL": u.AvatarURL,
	})
}
