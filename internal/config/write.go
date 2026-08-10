package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// filePerm keeps configuration readable by its owner only.
const filePerm os.FileMode = 0o600

// ErrConflict reports that the file changed between reading and writing.
var ErrConflict = errors.New("config file changed since it was read")

// ErrExists reports an attempt to create a configuration that already exists.
var ErrExists = errors.New("config file already exists")

// Render produces the complete file. Writing is always a full re-render rather
// than an in-place edit, which is what lets the comments below survive every
// write. A user's own comments and key ordering do not survive; ADR-005
// requires saying so plainly rather than implying otherwise.
func Render(c Config) string {
	var out strings.Builder
	out.WriteString("# Muxio configuration\n")
	out.WriteString("# Rewritten in full on every change: these comments are generated,\n")
	out.WriteString("# and any comments you add here will be lost the next time Muxio writes.\n")
	out.WriteString("# Credentials never belong in this file (see ADR-005).\n")

	section := ""
	for _, f := range fields {
		if f.section != section {
			section = f.section
			out.WriteString("\n[" + section + "]\n")
		}
		for _, line := range strings.Split(f.comment, "\n") {
			out.WriteString("# " + line + "\n")
		}
		out.WriteString(f.name + " = " + renderValue(&c, f) + "\n")
	}
	return out.String()
}

func renderValue(c *Config, f field) string {
	raw := f.get(c)
	if f.numeric {
		return raw
	}
	return strconv.Quote(raw)
}

// Write renders the configuration and replaces the file atomically.
//
// When expected is non-zero it must match the file's current modification time;
// otherwise the write is refused. This is what stops a settings UI from
// overwriting an edit someone made by hand in the meantime.
func Write(path string, c Config, expected time.Time) error {
	if err := c.Validate(); err != nil {
		return err
	}

	if !expected.IsZero() {
		current, err := TouchedAt(path)
		if err != nil {
			return err
		}
		if !current.Equal(expected) {
			return fmt.Errorf("%w: read at %s, now %s",
				ErrConflict,
				expected.UTC().Format(time.RFC3339),
				current.UTC().Format(time.RFC3339))
		}
	}

	return replaceFile(path, Render(c))
}

// Create writes a fresh configuration and refuses to clobber an existing one.
func Create(path string, c Config) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s", ErrExists, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat config %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return err
	}
	return replaceFile(path, Render(c))
}

// replaceFile writes content through a temporary file in the same directory and
// renames it into place, so a crash mid-write cannot leave a truncated config.
func replaceFile(path, content string) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempName := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}()

	if err := temp.Chmod(filePerm); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := temp.WriteString(content); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("flush config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace config %s: %w", path, err)
	}
	return nil
}
