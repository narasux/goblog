package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Get404 获取 404 页面
func Get404(c *gin.Context) {
	c.HTML(http.StatusOK, "404.html", nil)
}

// GetRobotsTxt 获取 robots.txt
func GetRobotsTxt(c *gin.Context) {
	c.String(http.StatusOK, "User-agent: *\nAllow: /\nAllow: /articles/\n\nDisallow: /static/")
}
