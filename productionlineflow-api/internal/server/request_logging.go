package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"productionlineflow-api/internal/logging"

	"github.com/gin-gonic/gin"
)

const (
	requestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
	errorCodeKey    = "error_code"
)

// requestLogging emits one structured access record for every request, including
// requests rejected by authentication middleware. It deliberately omits headers,
// query values, and bodies because they can contain credentials or tokens.
func requestLogging() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" || len(requestID) > 128 {
			var raw [16]byte
			if _, err := rand.Read(raw[:]); err != nil {
				requestID = time.Now().UTC().Format("20060102T150405.000000000")
			} else {
				requestID = hex.EncodeToString(raw[:])
			}
		}
		c.Set(requestIDKey, requestID)
		c.Header(requestIDHeader, requestID)
		requestContext := logging.WithRequestID(c.Request.Context(), requestID)
		requestContext = logging.WithSessionID(requestContext, "anonymous")
		c.Request = c.Request.WithContext(requestContext)

		started := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		attrs := []any{
			"method", c.Request.Method,
			"route", route,
			"status", c.Writer.Status(),
			"duration_ms", float64(time.Since(started).Microseconds()) / 1000,
		}
		if code, exists := c.Get(errorCodeKey); exists {
			attrs = append(attrs, "error_code", code)
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "error_count", len(c.Errors))
		}
		logger := logging.FromContext(c.Request.Context())
		switch status := c.Writer.Status(); {
		case status >= http.StatusInternalServerError:
			logger.Error("http request", attrs...)
		case status >= http.StatusBadRequest:
			logger.Warn("http request", attrs...)
		default:
			logger.Info("http request", attrs...)
		}
	}
}

func logInternalError(c *gin.Context, operation string, err error) {
	if err == nil {
		return
	}
	logging.FromContext(c.Request.Context()).Error("request operation failed",
		"method", c.Request.Method,
		"route", c.FullPath(),
		"operation", operation,
		"error", err,
	)
}

func setRequestSessionID(c *gin.Context, sessionID string) {
	c.Request = c.Request.WithContext(logging.WithSessionID(c.Request.Context(), sessionID))
}
