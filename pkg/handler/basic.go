package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Get404(c *gin.Context) {
	c.HTML(http.StatusOK, "404.html", nil)
}
