// Package logging builds the structured logger written to stderr.
//
// This is the debug-detail layer. Anything that must survive the process and be
// queryable belongs in run events instead; see docs/design/architecture.md.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// LevelEnv overrides the default log level.
const LevelEnv = "MUXIO_LOG_LEVEL"

// New builds a JSON logger at the given level.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// ParseLevel maps a name to a level. An empty name yields info.
func ParseLevel(name string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q, want debug, info, warn or error", name)
	}
}
