package ginx

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const (
	// MaxPageSize 单页最大数量
	MaxPageSize = 50
	// MinPageSize 单页最小数量
	MinPageSize = 10
	// MinPage 最小页码数
	MinPage = 1
)

// GetPageSizeFromQuery ...
func GetPageSizeFromQuery(c *gin.Context) int {
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	pageSize = lo.Min([]int{MaxPageSize, pageSize})
	pageSize = lo.Max([]int{MinPageSize, pageSize})
	return pageSize
}

// GetPageNumFromQuery ...
func GetPageNumFromQuery(c *gin.Context) int {
	pageNum, _ := strconv.Atoi(c.Query("page_num"))
	return lo.Max([]int{MinPage, pageNum})
}
