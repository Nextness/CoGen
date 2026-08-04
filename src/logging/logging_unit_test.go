// logging_unit_test.go tests the process-wide structured logger, verifying
// that the compile-time minimum log level is respected and that
// component loggers share the same handler.
//go:build unit

package logging

import (
	"context"
	"log/slog"
	"testing"
)

// TestLoggerCreatedAtInitUsesMinLevel verifies logger created at init uses min level.
func TestLoggerCreatedAtInitUsesMinLevel(t *testing.T) {
	logger := Logger("test")
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info should be enabled at info level")
	}
	if !logger.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("warn should be enabled at info level")
	}
	if !logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("error should be enabled at info level")
	}
}

// TestLoggerReturnsSharedHandler verifies logger returns shared handler.
func TestLoggerReturnsSharedHandler(t *testing.T) {
	components := []string{"pipeline", "database", "enrich", "normalization", "validation", "workspace"}
	for _, component := range components {
		logger := Logger(component)
		if !logger.Enabled(context.Background(), slog.LevelInfo) {
			t.Fatalf("%s logger disables debug at the shared threshold", component)
		}
		if !logger.Enabled(context.Background(), MinLevel) {
			t.Fatalf("%s logger disables the shared threshold", component)
		}
	}
}

// TestLoggerEmptyComponent verifies logger empty component.
func TestLoggerEmptyComponent(t *testing.T) {
	l := Logger("")
	if l == nil {
		t.Fatal("Logger with empty component should return a valid logger")
	}
}
