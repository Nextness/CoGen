// Package logging owns the process-wide structured logger used by the binary.
// The minimum level is a compile-time constant (MinLevel). Changing the
// threshold requires rebuilding the binary. There is no runtime level setter.
package logging

import (
	"log/slog"
	"os"
)

// MinLevel is the compile-time minimum log level for every package logger.
// Change this constant and rebuild the binary to raise or lower the threshold.
const MinLevel = slog.LevelInfo

var processLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: MinLevel,
}))

// init installs the package's process-wide runtime configuration.
func init() {
	// This also makes direct slog.Info/Debug calls use the same configuration.
	slog.SetDefault(processLog)
}

// Logger returns a logger backed by the shared process handler.
func Logger(component string) *slog.Logger {
	if component == "" {
		return processLog
	}
	return processLog.With("component", component)
}
