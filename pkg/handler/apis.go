package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/narasux/goblog/pkg/common/auth"
	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/model"
	"github.com/narasux/goblog/pkg/service/comment"
	"github.com/narasux/goblog/pkg/utils/ginx"
	"github.com/narasux/goblog/pkg/utils/markdownx"
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

// GetArticleComments 获取文章的所有评论
func GetArticleComments(c *gin.Context) {
	articleID := c.Param("id")

	comments, err := comment.GetCommentsByArticleID(c.Request.Context(), articleID)
	if err != nil {
		ginx.SetErrResp(c, http.StatusInternalServerError, err.Error())
		return
	}

	ginx.SetResp(c, http.StatusOK, comments)
}

// CreateComment 创建新评论
func CreateComment(c *gin.Context) {
	if !auth.IsInteractionEnabled() {
		ginx.SetErrResp(c, http.StatusForbidden, auth.InteractionDisabledMsg)
		return
	}

	user := auth.GetLoginUser(c)
	if user == nil {
		ginx.SetErrResp(c, http.StatusUnauthorized, "未登录，请先登录")
		return
	}

	articleID := c.Param("id")

	var req struct {
		Content  string `json:"content" binding:"required,max=2000"`
		ParentID *int64 `json:"parentID"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.SetErrResp(c, http.StatusBadRequest, "评论内容不能为空且不能超过 2000 字符")
		return
	}

	// 检查评论内容是否为空
	if len(req.Content) == 0 {
		ginx.SetErrResp(c, http.StatusBadRequest, "评论内容不能为空")
		return
	}

	newComment, err := comment.CreateComment(c.Request.Context(), articleID, user.ID, req.Content, req.ParentID)
	if err != nil {
		ginx.SetErrResp(c, http.StatusInternalServerError, err.Error())
		return
	}

	ginx.SetResp(c, http.StatusCreated, newComment)
}

// UpdateComment 更新评论
func UpdateComment(c *gin.Context) {
	if !auth.IsInteractionEnabled() {
		ginx.SetErrResp(c, http.StatusForbidden, auth.InteractionDisabledMsg)
		return
	}

	user := auth.GetLoginUser(c)
	if user == nil {
		ginx.SetErrResp(c, http.StatusUnauthorized, "未登录，请先登录")
		return
	}

	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ginx.SetErrResp(c, http.StatusBadRequest, "无效的评论 ID")
		return
	}

	var req struct {
		Content string `json:"content" binding:"required,max=2000"`
	}

	if err = c.ShouldBindJSON(&req); err != nil {
		ginx.SetErrResp(c, http.StatusBadRequest, "评论内容不能为空且不能超过 2000 字符")
		return
	}

	// 检查评论内容是否为空
	if len(req.Content) == 0 {
		ginx.SetErrResp(c, http.StatusBadRequest, "评论内容不能为空")
		return
	}

	if err = comment.UpdateComment(c.Request.Context(), commentID, user.ID, req.Content); err != nil {
		if err == comment.ErrCommentNotFound {
			ginx.SetErrResp(c, http.StatusNotFound, "评论不存在")
			return
		}
		if err == comment.ErrCommentOwnerOnly {
			ginx.SetErrResp(c, http.StatusForbidden, "只能编辑自己的评论")
			return
		}
		ginx.SetErrResp(c, http.StatusInternalServerError, err.Error())
		return
	}

	ginx.SetResp(c, http.StatusOK, nil)
}

// DeleteComment 删除评论
func DeleteComment(c *gin.Context) {
	if !auth.IsInteractionEnabled() {
		ginx.SetErrResp(c, http.StatusForbidden, auth.InteractionDisabledMsg)
		return
	}

	user := auth.GetLoginUser(c)
	if user == nil {
		ginx.SetErrResp(c, http.StatusUnauthorized, "未登录，请先登录")
		return
	}

	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ginx.SetErrResp(c, http.StatusBadRequest, "无效的评论 ID")
		return
	}

	if err = comment.DeleteComment(c.Request.Context(), commentID, user.ID); err != nil {
		if err == comment.ErrCommentNotFound {
			ginx.SetErrResp(c, http.StatusNotFound, "评论不存在")
			return
		}
		if err == comment.ErrCommentOwnerOnly {
			ginx.SetErrResp(c, http.StatusForbidden, "只能删除自己的评论")
			return
		}
		ginx.SetErrResp(c, http.StatusInternalServerError, err.Error())
		return
	}

	ginx.SetResp(c, http.StatusNoContent, nil)
}

// MarkdownToHTML 将 Markdown 内容转换为 HTML
func MarkdownToHTML(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.SetErrResp(c, http.StatusBadRequest, "内容不能为空")
		return
	}

	html := markdownx.ToHTML([]byte(req.Content))

	ginx.SetResp(c, http.StatusOK, gin.H{"html": html})
}
