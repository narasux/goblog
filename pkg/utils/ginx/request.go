package ginx

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"

	"github.com/TencentBlueKing/gopkg/conv"
	"github.com/pkg/errors"
)

// RequestIDHeaderKey ...
const RequestIDHeaderKey = "X-Request-ID"

// ErrNilRequestBody ...
var ErrNilRequestBody = errors.New("request Body is nil")

// ReadRequestBody will return the body in []byte, without change the origin body
func ReadRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, ErrNilRequestBody
	}

	body, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, err
}

// BasicAuthAuthorizationHeader ...
func BasicAuthAuthorizationHeader(user, password string) string {
	base := user + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString(conv.StringToBytes(base))
}
