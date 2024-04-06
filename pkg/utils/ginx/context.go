package ginx

import "github.com/gin-gonic/gin"

const (
	// RequestIDKey ...
	RequestIDKey = "requestID"
	// ClientIDKey ...
	ClientIDKey = "clientID"
	// ErrorKey ...
	ErrorKey = "error"
)

// GetRequestID ...
func GetRequestID(c *gin.Context) string {
	return c.GetString(RequestIDKey)
}

// SetRequestID ...
func SetRequestID(c *gin.Context, requestID string) {
	c.Set(RequestIDKey, requestID)
}

// GetClientID ...
func GetClientID(c *gin.Context) string {
	return c.GetString(ClientIDKey)
}

// SetClientID ...
func SetClientID(c *gin.Context, clientID string) {
	c.Set(ClientIDKey, clientID)
}

// GetError ...
func GetError(c *gin.Context) (any, bool) {
	return c.Get(ErrorKey)
}

// SetError ...
func SetError(c *gin.Context, err error) {
	c.Set(ErrorKey, err)
}
