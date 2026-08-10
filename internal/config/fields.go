package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// field describes one setting once, so loading, validation, display, editing
// and rendering all agree on its key, environment variable and meaning.
type field struct {
	key     string
	env     string
	comment string
	section string
	name    string
	// numeric drives rendering: a numeric field is written bare, a string field
	// is quoted. Inferring this from the value would misrender a string that
	// happens to look like a number.
	numeric  bool
	get      func(*Config) string
	set      func(*Config, string) error
	validate func(*Config) error
}

var fields = []field{
	{
		key:     "server.addr",
		env:     "MUXIO_SERVER_ADDR",
		section: "server",
		name:    "addr",
		comment: "HTTP listen address. Must stay on loopback until authentication\nand a threat model exist (ADR-002).",
		get:     func(c *Config) string { return c.Server.Addr },
		set: func(c *Config, value string) error {
			c.Server.Addr = value
			return nil
		},
		validate: func(c *Config) error {
			if err := checkLoopback(c.Server.Addr); err != nil {
				return &ValidationError{
					Key: "server.addr", Value: strconv.Quote(c.Server.Addr),
					Want: err.Error(),
				}
			}
			return nil
		},
	},
	{
		key:     "log.level",
		env:     "MUXIO_LOG_LEVEL",
		section: "log",
		name:    "level",
		comment: "One of debug, info, warn, error.",
		get:     func(c *Config) string { return c.Log.Level },
		set: func(c *Config, value string) error {
			c.Log.Level = value
			return nil
		},
		validate: func(c *Config) error {
			switch c.Log.Level {
			case "debug", "info", "warn", "error":
				return nil
			default:
				return &ValidationError{
					Key: "log.level", Value: strconv.Quote(c.Log.Level),
					Want: "want debug, info, warn or error",
				}
			}
		},
	},
	{
		key:     "capture.max_body_bytes",
		env:     "MUXIO_CAPTURE_MAX_BODY_BYTES",
		section: "capture",
		name:    "max_body_bytes",
		numeric: true,
		comment: "Largest body accepted for a single capture. Bodies above this are\nrejected rather than truncated, so nothing is silently altered.",
		get:     func(c *Config) string { return strconv.Itoa(c.Capture.MaxBodyBytes) },
		set: func(c *Config, value string) error {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return &ValidationError{
					Key: "capture.max_body_bytes", Value: value, Want: "want a whole number",
				}
			}
			c.Capture.MaxBodyBytes = parsed
			return nil
		},
		validate: func(c *Config) error {
			if c.Capture.MaxBodyBytes <= 0 || c.Capture.MaxBodyBytes > maxAllowedBodyBytes {
				return &ValidationError{
					Key: "capture.max_body_bytes", Value: strconv.Itoa(c.Capture.MaxBodyBytes),
					Want: fmt.Sprintf("want between 1 and %d", maxAllowedBodyBytes),
				}
			}
			return nil
		},
	},
	{
		key:     "retention.run_event_days",
		env:     "MUXIO_RETENTION_RUN_EVENT_DAYS",
		section: "retention",
		name:    "run_event_days",
		numeric: true,
		comment: "How long run events are kept. Runs themselves are never purged:\nlosing events costs explainability, losing runs costs correctness.",
		get:     func(c *Config) string { return strconv.Itoa(c.Retention.RunEventDays) },
		set: func(c *Config, value string) error {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return &ValidationError{
					Key: "retention.run_event_days", Value: value, Want: "want a whole number",
				}
			}
			c.Retention.RunEventDays = parsed
			return nil
		},
		validate: func(c *Config) error {
			if c.Retention.RunEventDays <= 0 || c.Retention.RunEventDays > maxAllowedEventsDays {
				return &ValidationError{
					Key: "retention.run_event_days", Value: strconv.Itoa(c.Retention.RunEventDays),
					Want: fmt.Sprintf("want between 1 and %d", maxAllowedEventsDays),
				}
			}
			return nil
		},
	},
}

// Keys lists every settable field key in file order.
func Keys() []string {
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.key)
	}
	return keys
}

// Get reads one field by key.
func Get(c Config, key string) (string, error) {
	for _, f := range fields {
		if f.key == key {
			return f.get(&c), nil
		}
	}
	return "", unknownKey(key)
}

// Set writes one field by key and validates the result.
func Set(c Config, key, value string) (Config, error) {
	for _, f := range fields {
		if f.key != key {
			continue
		}
		if err := f.set(&c, value); err != nil {
			return Config{}, err
		}
		if err := f.validate(&c); err != nil {
			return Config{}, err
		}
		return c, nil
	}
	return Config{}, unknownKey(key)
}

func unknownKey(key string) error {
	return fmt.Errorf("unknown setting %q, known settings are %s",
		key, strings.Join(Keys(), ", "))
}

// checkLoopback mirrors the CLI's listen address rule so an address is rejected
// at configuration time rather than at startup.
func checkLoopback(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("want host:port")
	}
	if port == "" {
		return fmt.Errorf("want a port")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("want a loopback address such as 127.0.0.1")
	}
	return nil
}
