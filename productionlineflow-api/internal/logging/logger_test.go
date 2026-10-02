package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestLoggerConfigurationAndRequestContext(t *testing.T) {
	var output bytes.Buffer
	logger, err := newLogger("warn", "json", &output)
	if err != nil {
		t.Fatal(err)
	}
	if logger.Handler().Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("expected info logs to be disabled")
	}
	if !logger.Handler().Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("expected warning logs to be enabled")
	}

	previous := Default()
	SetDefault(logger)
	t.Cleanup(func() { SetDefault(previous) })

	ctx := WithRequestID(context.Background(), "request-123")
	ctx = WithSessionID(ctx, "session-456")
	FromContext(ctx).Warn("request failed")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	if record["request_id"] != "request-123" || record["session_id"] != "session-456" {
		t.Fatalf("missing request context fields: %#v", record)
	}
}

func TestNewLoggerRejectsUnsupportedConfiguration(t *testing.T) {
	for _, test := range []struct {
		name   string
		level  string
		format string
	}{
		{name: "level", level: "verbose", format: "json"},
		{name: "format", level: "info", format: "yaml"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.level, test.format); err == nil {
				t.Fatal("expected unsupported logger configuration to fail")
			}
		})
	}
}
