package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/narasux/goblog/pkg/common/ctxkey"
	"github.com/narasux/goblog/pkg/model"
)

// GetLoginUser 从 Context 获取当前登录用户
func GetLoginUser(c *gin.Context) *model.User {
	if user, exists := c.Get(ctxkey.LoginUser); exists && user != nil {
		return user.(*model.User)
	}
	return nil
}
