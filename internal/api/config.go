package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ConteMan/muxio/internal/config"
)

// maxConfigBody bounds a configuration upload. The file is a few dozen lines;
// anything larger is a mistake or an attack.
const maxConfigBody = 1 << 20

// Additional error codes used by the write path.
const (
	CodePreconditionRequired = "precondition_required"
	CodePreconditionFailed   = "precondition_failed"
	CodePayloadTooLarge      = "payload_too_large"
)

type configUpdate struct {
	Settings []configSettingInput `json:"settings"`
}

type configSettingInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (s *server) getConfig(w http.ResponseWriter, r *http.Request) {
	loaded, ok := s.currentConfig(w, r)
	if !ok {
		return
	}
	w.Header().Set("ETag", quoteETag(loaded.Fingerprint))
	writeJSON(w, http.StatusOK, newConfigView(loaded))
}

func (s *server) putConfig(w http.ResponseWriter, r *http.Request) {
	if s.saveConfig == nil {
		internalError(w, s.logFailure(r), errNoConfigWriter)
		return
	}

	expected, ok := parsePrecondition(w, r)
	if !ok {
		return
	}

	update, ok := decodeConfigUpdate(w, r)
	if !ok {
		return
	}

	desired, err := buildConfig(update.Settings)
	if err != nil {
		var invalid *config.ValidationError
		if errors.As(err, &invalid) {
			invalidArgument(w, invalid.Key, invalid.Error())
			return
		}
		var missing *missingSettingError
		if errors.As(err, &missing) {
			invalidArgument(w, missing.Key, missing.Error())
			return
		}
		invalidArgument(w, "settings", err.Error())
		return
	}

	if err := s.saveConfig(desired, expected); err != nil {
		if errors.Is(err, config.ErrConflict) {
			s.writeConflict(w, r)
			return
		}
		var invalid *config.ValidationError
		if errors.As(err, &invalid) {
			invalidArgument(w, invalid.Key, invalid.Error())
			return
		}
		internalError(w, s.logFailure(r), err)
		return
	}

	// Reload so the response reports the real effective values: a setting
	// overridden by the environment keeps its overridden value even though the
	// file now says otherwise.
	loaded, ok := s.currentConfig(w, r)
	if !ok {
		return
	}
	w.Header().Set("ETag", quoteETag(loaded.Fingerprint))
	writeJSON(w, http.StatusOK, newConfigView(loaded))
}

// writeConflict reports a stale precondition together with the current version,
// so a client can show what it is now based against.
func (s *server) writeConflict(w http.ResponseWriter, r *http.Request) {
	if loaded, err := s.loadConfig(); err == nil {
		w.Header().Set("ETag", quoteETag(loaded.Fingerprint))
	} else {
		s.logFailure(r)(err)
	}
	writeError(w, http.StatusPreconditionFailed, CodePreconditionFailed,
		"the configuration changed since it was read; reload and reapply your edit", "")
}

func (s *server) currentConfig(w http.ResponseWriter, r *http.Request) (config.Loaded, bool) {
	if s.loadConfig == nil {
		internalError(w, s.logFailure(r), errNoConfigLoader)
		return config.Loaded{}, false
	}
	loaded, err := s.loadConfig()
	if err != nil {
		internalError(w, s.logFailure(r), err)
		return config.Loaded{}, false
	}
	return loaded, true
}

// parsePrecondition requires a concrete If-Match value.
func parsePrecondition(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		writeError(w, http.StatusPreconditionRequired, CodePreconditionRequired,
			"If-Match is required; read the configuration first and send back its ETag", "")
		return "", false
	}
	if raw == "*" {
		// "*" only asserts that the resource exists, which is exactly the
		// unconditional write this endpoint exists to prevent.
		invalidArgument(w, "If-Match",
			`If-Match: * is not accepted; send the ETag the edit was based on`)
		return "", false
	}
	return unquoteETag(raw), true
}

func decodeConfigUpdate(w http.ResponseWriter, r *http.Request) (configUpdate, bool) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxConfigBody))
	decoder.DisallowUnknownFields()

	var update configUpdate
	if err := decoder.Decode(&update); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, CodePayloadTooLarge,
				fmt.Sprintf("the request body may not exceed %d bytes", maxConfigBody), "")
			return configUpdate{}, false
		}
		invalidArgument(w, "", "the request body is not valid JSON: "+err.Error())
		return configUpdate{}, false
	}
	return update, true
}

// missingSettingError reports an omitted setting. Omission is an error rather
// than a reset: a client that forgot a field must not silently revert it.
type missingSettingError struct {
	Key string
}

func (e *missingSettingError) Error() string {
	return "setting " + e.Key + " is missing; a write must include every setting"
}

// buildConfig turns a full set of submitted settings into a configuration.
func buildConfig(settings []configSettingInput) (config.Config, error) {
	known := make(map[string]bool, len(config.Keys()))
	for _, key := range config.Keys() {
		known[key] = true
	}

	provided := make(map[string]string, len(settings))
	for _, setting := range settings {
		if !known[setting.Key] {
			return config.Config{}, fmt.Errorf("unknown setting %q, known settings are %s",
				setting.Key, strings.Join(config.Keys(), ", "))
		}
		if _, duplicate := provided[setting.Key]; duplicate {
			return config.Config{}, fmt.Errorf("setting %q appears more than once", setting.Key)
		}
		provided[setting.Key] = setting.Value
	}

	result := config.Default()
	for _, key := range config.Keys() {
		value, ok := provided[key]
		if !ok {
			return config.Config{}, &missingSettingError{Key: key}
		}
		updated, err := config.Set(result, key, value)
		if err != nil {
			return config.Config{}, err
		}
		result = updated
	}
	return result, nil
}

func newConfigView(loaded config.Loaded) configView {
	settings := make([]configSettingView, 0, len(config.Keys()))
	for _, key := range config.Keys() {
		value, err := config.Get(loaded.Config, key)
		if err != nil {
			// Keys() is the source of the loop, so this cannot happen; report
			// the key rather than silently dropping it if it ever does.
			value = ""
		}
		settings = append(settings, configSettingView{
			Key:    key,
			Value:  value,
			Origin: string(loaded.Origin(key)),
		})
	}
	return configView{
		Path:     loaded.Path,
		Exists:   loaded.Exists,
		Settings: settings,
	}
}

func quoteETag(fingerprint string) string {
	return `"` + fingerprint + `"`
}

func unquoteETag(raw string) string {
	return strings.Trim(raw, `"`)
}
