package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/narasux/goblog/pkg/common/ctxkey"
	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/model"
)

// InteractionDisabledMsg 交互功能关闭时的提示文案
const InteractionDisabledMsg = "交互功能暂未开放"

// IsInteractionEnabled 是否启用交互功能（登录与评论，由 INTERACTION_ENABLED 控制）
func IsInteractionEnabled() bool {
	return envs.InteractionEnabled
}

// GetLoginUser 从 Context 获取当前登录用户
func GetLoginUser(c *gin.Context) *model.User {
	if user, exists := c.Get(ctxkey.LoginUser); exists && user != nil {
		return user.(*model.User)
	}
	return nil
}
