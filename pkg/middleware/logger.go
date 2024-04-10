package middleware

import (
	"bytes"
	"time"

	"github.com/TencentBlueKing/gopkg/stringx"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/logging"
	"github.com/narasux/goblog/pkg/utils/ginx"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write ...
func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// 获取客户端 IP
func getClientIP(c *gin.Context) string {
	if envs.RealClientIPHeaderKey != "" {
		return c.GetHeader(envs.RealClientIPHeaderKey)
	}
	return c.ClientIP()
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		reqBody, respBody := "", ""
		if requestBody, err := ginx.ReadRequestBody(c.Request); err == nil {
			// NOTE: no truncation
			reqBody = string(requestBody)
		}

		writer := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = writer

		c.Next()

		// 检查错误信息，以手动设置的为主，否则检查 c.Errors
		errStr, hasErr := ginx.GetError(c)
		if !hasErr && len(c.Errors) > 0 {
			errStr = c.Errors.String()
			hasErr = true
		}

		// 统计请求耗时，单位为 ms，限制最小 1ms
		duration := time.Since(start)
		latency := float64(duration/time.Millisecond) + 1

		// 请求参数
		params := stringx.Truncate(c.Request.URL.RawQuery, 1024)

		// 如果没有错误信息，则不关注 respBody
		if hasErr {
			respBody = stringx.Truncate(writer.body.String(), 1024)
		}

		fields := logrus.Fields{
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"params":    params,
			"reqBody":   reqBody,
			"respBody":  respBody,
			"status":    c.Writer.Status(),
			"latency":   latency,
			"requestID": ginx.GetRequestID(c),
			"clientID":  ginx.GetClientID(c),
			"clientIP":  getClientIP(c),
			"error":     errStr,
		}

		logger := logging.GetAccessLogger()
		if hasErr {
			logger.WithFields(fields).Error("-")
		} else {
			logger.WithFields(fields).Info("-")
		}
	}
}
