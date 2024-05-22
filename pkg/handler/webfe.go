package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/storage"
)

func GetHomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", map[string]string{
		"googleSiteVerificationCode": envs.GoogleSiteVerificationCode,
		"baiduSiteVerificationCode":  envs.BaiduSiteVerificationCode,
	})
}

func ListArticles(c *gin.Context) {
	articles := storage.BlogData.Articles
	if category := c.Query("category"); category != "" {
		articles = articles.FilterByCategory(category)
	}
	if tag := c.Query("tag"); tag != "" {
		articles = articles.FilterByTag(tag)
	}
	// TODO 支持分页，需要对应调整前端页面
	c.HTML(http.StatusOK, "articles.html", articles)
}

func RetrieveArticle(c *gin.Context) {
	article := storage.BlogData.Articles.GetByID(c.Param("id"))
	if article == nil {
		Get404(c)
		return
	}
	c.HTML(http.StatusOK, "article_detail.html", map[string]any{
		"article":         article,
		"mermaidRequired": strings.Contains(article.Content, "mermaid"),
	})
}

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
