package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// fileShape mirrors the on-disk layout. Values are read through TOML metadata
// rather than pointers so an explicitly written zero is distinguishable from an
// absent key.
type fileShape struct {
	Server struct {
		Addr string `toml:"addr"`
	} `toml:"server"`
	Log struct {
		Level string `toml:"level"`
	} `toml:"log"`
	Capture struct {
		MaxBodyBytes int `toml:"max_body_bytes"`
	} `toml:"capture"`
	Retention struct {
		RunEventDays int `toml:"run_event_days"`
	} `toml:"retention"`
}

// Load resolves the effective configuration from defaults, the file at path,
// and the environment. Command-line overrides are applied afterwards by the
// caller through Override, which knows which flags were actually set.
func Load(path string, lookupEnv func(string) (string, bool)) (Loaded, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	loaded := Loaded{
		Config:  Default(),
		Origins: make(map[string]Origin, len(fields)),
		Path:    path,
	}
	for _, f := range fields {
		loaded.Origins[f.key] = FromDefault
	}

	if err := applyFile(&loaded, path); err != nil {
		return Loaded{}, err
	}
	if err := applyEnv(&loaded, lookupEnv); err != nil {
		return Loaded{}, err
	}
	if err := loaded.Config.Validate(); err != nil {
		return Loaded{}, err
	}
	return loaded, nil
}

func applyFile(loaded *Loaded, path string) error {
	fingerprint, err := Fingerprint(path)
	if err != nil {
		return err
	}
	loaded.Fingerprint = fingerprint
	if fingerprint == AbsentFingerprint {
		// An absent file is not an error: an unconfigured Muxio must run.
		return nil
	}
	loaded.Exists = true

	var shape fileShape
	meta, err := toml.DecodeFile(path, &shape)
	if err != nil {
		var parseErr toml.ParseError
		if errors.As(err, &parseErr) {
			return fmt.Errorf("config %s is not valid TOML:\n%s",
				path, parseErr.ErrorWithPosition())
		}
		return fmt.Errorf("read config %s: %w", path, err)
	}

	// An unknown key is nearly always a typo. Ignoring it silently would leave
	// the user believing a setting took effect.
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		names := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			names = append(names, key.String())
		}
		return fmt.Errorf("config %s has unknown settings: %s\nknown settings are %s",
			path, strings.Join(names, ", "), strings.Join(Keys(), ", "))
	}

	for _, f := range fields {
		if !meta.IsDefined(f.section, f.name) {
			continue
		}
		if err := applyFileField(loaded, f, shape, meta); err != nil {
			return err
		}
	}
	return nil
}

func applyFileField(loaded *Loaded, f field, shape fileShape, meta toml.MetaData) error {
	var raw string
	switch f.key {
	case "server.addr":
		raw = shape.Server.Addr
	case "log.level":
		raw = shape.Log.Level
	case "capture.max_body_bytes":
		raw = fmt.Sprint(shape.Capture.MaxBodyBytes)
	case "retention.run_event_days":
		raw = fmt.Sprint(shape.Retention.RunEventDays)
	default:
		return fmt.Errorf("no file mapping for %s", f.key)
	}

	if err := f.set(&loaded.Config, raw); err != nil {
		return annotateLine(err, loaded.Path, f)
	}
	if err := f.validate(&loaded.Config); err != nil {
		return annotateLine(err, loaded.Path, f)
	}
	loaded.Origins[f.key] = FromFile
	return nil
}

// annotateLine attaches the file position so the user can jump straight to the
// offending line. The TOML decoder reports positions only for parse errors, so
// a value rejected by our own validation has to be located here.
func annotateLine(err error, path string, f field) error {
	var invalid *ValidationError
	if errors.As(err, &invalid) {
		invalid.Line = findKeyLine(path, f)
	}
	return err
}

// findKeyLine locates "name =" within the field's section. It returns 0 when
// the line cannot be found, which degrades to an error without a position
// rather than a wrong one.
func findKeyLine(path string, f field) int {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	section := ""
	for index, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			continue
		}
		if section != f.section {
			continue
		}
		key, _, found := strings.Cut(trimmed, "=")
		if found && strings.TrimSpace(key) == f.name {
			return index + 1
		}
	}
	return 0
}

func applyEnv(loaded *Loaded, lookupEnv func(string) (string, bool)) error {
	for _, f := range fields {
		raw, ok := lookupEnv(f.env)
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		if err := f.set(&loaded.Config, strings.TrimSpace(raw)); err != nil {
			return fmt.Errorf("%s: %w", f.env, err)
		}
		if err := f.validate(&loaded.Config); err != nil {
			return fmt.Errorf("%s: %w", f.env, err)
		}
		loaded.Origins[f.key] = FromEnv
	}
	return nil
}

// Override applies a command-line value to one field and records the origin.
// Callers pass only flags the user actually set, so an unset flag never
// shadows the file or the environment.
func (l *Loaded) Override(key, value string) error {
	updated, err := Set(l.Config, key, value)
	if err != nil {
		return err
	}
	l.Config = updated
	l.Origins[key] = FromFlag
	return nil
}
