package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/narasux/goblog/pkg/utils/ginx"
	"github.com/narasux/goblog/pkg/utils/uuid"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(ginx.RequestIDHeaderKey)

		if requestID == "" || len(requestID) != 32 {
			requestID = uuid.GenUUID4()
		}
		ginx.SetRequestID(c, requestID)
		c.Writer.Header().Set(ginx.RequestIDHeaderKey, requestID)

		c.Next()
	}
}
