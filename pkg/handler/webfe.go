package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"
	"github.com/samber/lo"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/logging"
	"github.com/narasux/goblog/pkg/model"
	"github.com/narasux/goblog/pkg/storage"
	"github.com/narasux/goblog/pkg/utils/ginx"
)

// GetHomePage 获取主页
func GetHomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", map[string]string{
		"googleSiteVerificationCode": envs.GoogleSiteVerificationCode,
		"baiduSiteVerificationCode":  envs.BaiduSiteVerificationCode,
	})
}

// ListArticles 获取文章列表
func ListArticles(c *gin.Context) {
	articles := storage.BlogData.Articles
	if category := c.Query("category"); category != "" {
		articles = articles.FilterByCategory(category)
	}
	if tag := c.Query("tag"); tag != "" {
		articles = articles.FilterByTag(tag)
	}

	db := database.Client(c.Request.Context())

	// 统计各个文章的点赞数量
	type Result struct {
		ArticleID string
		Count     int64
	}
	var results []Result

	// 忽略查询失败
	db.Model(&model.ViewRecord{}).Select("article_id, count(*) as count").Group("article_id").Find(&results)
	viewCntMap := lo.SliceToMap(results, func(item Result) (string, int64) {
		return item.ArticleID, item.Count
	})

	db.Model(&model.LikeRecord{}).Select("article_id, count(*) as count").Group("article_id").Find(&results)
	likeCntMap := lo.SliceToMap(results, func(item Result) (string, int64) {
		return item.ArticleID, item.Count
	})

	c.HTML(http.StatusOK, "articles.html", map[string]any{
		"articles": articles, "viewCntMap": viewCntMap, "likeCntMap": likeCntMap,
	})
}

// RetrieveArticle 获取文章详情
func RetrieveArticle(c *gin.Context) {
	article := storage.BlogData.Articles.GetByID(c.Param("id"))
	if article == nil {
		Get404(c)
		return
	}

	clientIP := ginx.GetClientIP(c)
	articleID := c.Param("id")
	db := database.Client(c.Request.Context())

	// 添加文章访问记录（同一 IP 30 分钟内只统计一次）
	var count int64
	db.Model(&model.ViewRecord{}).Where(
		"ip = ? AND article_id = ? AND created_at >= ?",
		clientIP, articleID, time.Now().Add(-30*time.Minute),
	).Count(&count)

	if count == 0 {
		record := model.ViewRecord{
			IP:        clientIP,
			ArticleID: articleID,
			BaseModel: model.BaseModel{Creator: ginx.GetClientID(c)},
		}
		if err := db.Create(&record).Error; err != nil {
			// 记录失败不影响正常展示
			logging.GetSystemLogger().Errorf("failed to create view record: %s", err.Error())
		}
	}

	c.HTML(http.StatusOK, "article_detail.html", map[string]any{
		"article":         article,
		"mermaidRequired": strings.Contains(article.Content, "mermaid"),
	})
}

// PeriodicTable 软件设计元素周期表
func PeriodicTable(c *gin.Context) {
	logger := logging.GetSystemLogger()

	content, err := os.ReadFile(filepath.Join(envs.BlogDataBaseDir, "periodic_table.json"))
	if err != nil {
		// 加载不到文件，也没必要报错，就提示功能开发中 :D
		c.HTML(http.StatusOK, "coming_soon.html", nil)
		// 打印错误日志
		logger.Errorf("failed to load periodic table: %s", err.Error())
		return
	}

	var periodicTable model.ElementPeriodicTable
	if err = json.Unmarshal(content, &periodicTable); err != nil {
		// 加载不到文件，也没必要报错，就提示功能开发中 :D
		c.HTML(http.StatusOK, "coming_soon.html", nil)
		// 打印错误日志
		logger.Errorf("failed to unmarshal periodic table: %s", err.Error())
		return
	}

	c.HTML(http.StatusOK, "periodic_table.html", periodicTable)
}

// GetRSS 获取 RSS
func GetRSS(c *gin.Context) {
	feed := &feeds.Feed{
		Title:       "Schnee's Blog",
		Link:        &feeds.Link{Href: fmt.Sprintf("%s://%s/articles", envs.DomainScheme, envs.Domain)},
		Description: "discussion about technology, thoughts and life",
		Author:      &feeds.Author{Name: "Schnee", Email: envs.ContactEmail},
		Updated:     time.Now(),
	}
	for _, article := range storage.BlogData.Articles {
		updatedAt, _ := time.ParseInLocation(time.DateOnly, article.UpdatedAt, time.Local)
		feed.Items = append(feed.Items, &feeds.Item{
			Id:          article.ID,
			Title:       article.Title,
			Link:        &feeds.Link{Href: fmt.Sprintf("%s://%s/articles/%s", envs.DomainScheme, envs.Domain, article.ID)},
			Description: article.Desc,
			Author:      &feeds.Author{Name: "Schnee", Email: envs.ContactEmail},
			Created:     updatedAt,
			Updated:     updatedAt,
		})
	}
	atom, _ := feed.ToAtom()

	// 不直接使用 c.XML() 以避免被包装 <string></string>
	c.Writer.Header().Set("Content-Type", "application/xml; charset=utf-8")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(atom))
}
