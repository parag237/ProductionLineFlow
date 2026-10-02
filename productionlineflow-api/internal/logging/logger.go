package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

type contextKey uint8

const (
	requestIDKey contextKey = iota
	sessionIDKey
)

var defaultLogger atomic.Pointer[slog.Logger]

func init() {
	defaultLogger.Store(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
}

func New(levelName, formatName string) (*slog.Logger, error) {
	return newLogger(levelName, formatName, os.Stdout)
}

func newLogger(levelName, formatName string, output io.Writer) (*slog.Logger, error) {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(levelName)) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported log level %q", levelName)
	}

	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(formatName)) {
	case "json":
		handler = slog.NewJSONHandler(output, options)
	case "text":
		handler = slog.NewTextHandler(output, options)
	default:
		return nil, fmt.Errorf("unsupported log format %q", formatName)
	}
	return slog.New(handler), nil
}

func SetDefault(logger *slog.Logger) {
	if logger == nil {
		panic("logging: cannot set a nil default logger")
	}
	defaultLogger.Store(logger)
}

func Default() *slog.Logger {
	return defaultLogger.Load()
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

func FromContext(ctx context.Context) *slog.Logger {
	attributes := make([]any, 0, 4)
	if requestID, ok := ctx.Value(requestIDKey).(string); ok && requestID != "" {
		attributes = append(attributes, "request_id", requestID)
	}
	if sessionID, ok := ctx.Value(sessionIDKey).(string); ok && sessionID != "" {
		attributes = append(attributes, "session_id", sessionID)
	}
	return Default().With(attributes...)
}
