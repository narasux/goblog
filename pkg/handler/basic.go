package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Get404(c *gin.Context) {
	c.HTML(http.StatusOK, "404.html", nil)
}

func GetRobotsTxt(c *gin.Context) {
	c.String(http.StatusOK, "User-agent: *\nAllow: /\nAllow: /articles/\n\nDisallow: /static/")
}
