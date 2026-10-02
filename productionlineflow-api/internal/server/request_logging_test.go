package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"

	"productionlineflow-api/internal/logging"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggingRecordsRequestAndSessionIDs(t *testing.T) {
	var output bytes.Buffer
	previousLogger := logging.Default()
	logging.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { logging.SetDefault(previousLogger) })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(requestLogging())
	router.GET("/session", func(c *gin.Context) {
		setRequestSessionID(c, "session-456")
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/session", nil)
	request.Header.Set(requestIDHeader, "request-123")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Header().Get(requestIDHeader) != "request-123" {
		t.Fatalf("request ID response header mismatch: %q", response.Header().Get(requestIDHeader))
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("decode access log: %v", err)
	}
	if record["request_id"] != "request-123" || record["session_id"] != "session-456" {
		t.Fatalf("access log missing correlation IDs: %#v", record)
	}
}
