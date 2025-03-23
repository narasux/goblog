package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/model"
	"github.com/narasux/goblog/pkg/utils/ginx"
)

// LikeArticle 点赞文章
func LikeArticle(c *gin.Context) {
	clientIP := ginx.GetClientIP(c)
	articleID := c.Param("id")
	db := database.Client(c.Request.Context())

	// 添加文章点赞记录（同一 IP 30 分钟内只统计一次）
	var count int64
	db.Model(&model.LikeRecord{}).Where(
		"ip = ? AND article_id = ? AND created_at >= ?",
		clientIP, articleID, time.Now().Add(-30*time.Minute),
	).Count(&count)

	if count != 0 {
		ginx.SetResp(c, http.StatusNoContent, nil)
		return
	}

	record := model.LikeRecord{
		IP:        clientIP,
		ArticleID: articleID,
		BaseModel: model.BaseModel{Creator: ginx.GetClientID(c)},
	}
	if err := db.Create(&record).Error; err != nil {
		ginx.SetErrResp(c, http.StatusInternalServerError, err.Error())
		return
	}
	ginx.SetResp(c, http.StatusNoContent, nil)
}
