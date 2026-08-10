// Package paths resolves where Muxio keeps its configuration and data.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// HomeEnv overrides every resolved path. It exists for tests and portable runs.
const HomeEnv = "MUXIO_HOME"

// dirPerm keeps the data directory readable by its owner only. Captured content
// is personal data, so it must not be world readable.
const dirPerm os.FileMode = 0o700

// DatabaseFile is the SQLite database name inside the resolved home directory.
const DatabaseFile = "muxio.db"

// ConfigFileName is the configuration file name.
const ConfigFileName = "config.toml"

// Home returns the directory holding Muxio's data. It does not create anything.
func Home() (string, error) {
	if override := os.Getenv(HomeEnv); override != "" {
		absolute, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", HomeEnv, err)
		}
		return absolute, nil
	}

	base, err := userDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "muxio"), nil
}

// Database returns the SQLite path. It does not create the file or its parent.
func Database() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DatabaseFile), nil
}

// ConfigFile returns the configuration path. It does not create the file.
//
// Configuration lives in the platform config directory while data lives in the
// data directory, but MUXIO_HOME collapses both into one place so a portable or
// test installation stays self-contained.
func ConfigFile() (string, error) {
	if override := os.Getenv(HomeEnv); override != "" {
		home, err := Home()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ConfigFileName), nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(base, "muxio", ConfigFileName), nil
}

// EnsureConfigDir creates the directory holding the configuration file.
func EnsureConfigDir() (string, error) {
	configPath, err := ConfigFile()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	return dir, nil
}

// EnsureHome creates the data directory with owner-only permissions.
func EnsureHome() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(home, dirPerm); err != nil {
		return "", fmt.Errorf("create data directory: %w", err)
	}
	return home, nil
}

// userDataDir reports the platform directory for application data. The standard
// library only exposes config and cache directories, and on Linux those are not
// where a database belongs.
func userDataDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return dir, nil
		}
		return "", errors.New("neither MUXIO_HOME nor LOCALAPPDATA is set")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return dir, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}
