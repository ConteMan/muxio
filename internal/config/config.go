// Package config loads, validates and writes Muxio's configuration.
//
// The file is the single source of truth (ADR-005). Nothing here touches the
// database, and credentials never appear in this file.
package config

import (
	"fmt"
	"time"

	"github.com/ConteMan/muxio/internal/record"
)

// Defaults. A Muxio that has never been configured must run correctly.
const (
	DefaultAddr          = "127.0.0.1:8080"
	DefaultLogLevel      = "info"
	DefaultMaxBodyBytes  = record.DefaultMaxBodyBytes
	DefaultRunEventDays  = 30
	maxAllowedBodyBytes  = 64 << 20
	maxAllowedEventsDays = 3650
)

// Config is the whole of Muxio's settings.
type Config struct {
	Server    Server
	Log       Log
	Capture   Capture
	Retention Retention
}

// Server holds the local HTTP settings.
type Server struct {
	Addr string
}

// Log holds logging settings.
type Log struct {
	Level string
}

// Capture holds limits applied to captured records.
type Capture struct {
	MaxBodyBytes int
}

// Retention holds how long derived history is kept.
type Retention struct {
	RunEventDays int
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		Server:    Server{Addr: DefaultAddr},
		Log:       Log{Level: DefaultLogLevel},
		Capture:   Capture{MaxBodyBytes: DefaultMaxBodyBytes},
		Retention: Retention{RunEventDays: DefaultRunEventDays},
	}
}

// RunEventRetention converts the configured day count to a duration.
func (c Config) RunEventRetention() time.Duration {
	return time.Duration(c.Retention.RunEventDays) * 24 * time.Hour
}

// Origin records where an effective value came from. Without it, "why is this
// setting not taking effect" cannot be answered from the outside.
type Origin string

const (
	FromDefault Origin = "default"
	FromFile    Origin = "file"
	FromEnv     Origin = "env"
	FromFlag    Origin = "flag"
)

// Loaded is a configuration together with the provenance of each field.
type Loaded struct {
	Config Config
	// Origins maps a field key such as "server.addr" to where its value came from.
	Origins map[string]Origin
	// Path is the file consulted, whether or not it existed.
	Path string
	// Exists reports whether the file was present.
	Exists bool
	// Fingerprint identifies the bytes that were read. Writes pass it back to
	// detect a concurrent edit; see ADR-006.
	Fingerprint string
}

// Origin reports where a field's effective value came from.
func (l Loaded) Origin(key string) Origin {
	if origin, ok := l.Origins[key]; ok {
		return origin
	}
	return FromDefault
}

// ValidationError explains one rejected field.
type ValidationError struct {
	Key   string
	Value string
	Want  string
	Line  int
}

func (e *ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s = %s is invalid, %s", e.Line, e.Key, e.Value, e.Want)
	}
	return fmt.Sprintf("%s = %s is invalid, %s", e.Key, e.Value, e.Want)
}

// Validate checks every field and reports the first problem.
func (c Config) Validate() error {
	for _, f := range fields {
		if err := f.validate(&c); err != nil {
			return err
		}
	}
	return nil
}
