package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHomeOverrideWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnv, home)

	resolved, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if resolved != home {
		t.Fatalf("Home() = %q, want %q", resolved, home)
	}

	databasePath, err := Database()
	if err != nil {
		t.Fatalf("Database: %v", err)
	}
	if want := filepath.Join(home, DatabaseFile); databasePath != want {
		t.Fatalf("Database() = %q, want %q", databasePath, want)
	}
}

func TestHomeOverrideIsMadeAbsolute(t *testing.T) {
	t.Setenv(HomeEnv, "relative-home")

	resolved, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("Home() = %q, want an absolute path", resolved)
	}
}

func TestResolvingDoesNotCreateAnything(t *testing.T) {
	home := filepath.Join(t.TempDir(), "not-yet")
	t.Setenv(HomeEnv, home)

	databasePath, err := Database()
	if err != nil {
		t.Fatalf("Database: %v", err)
	}
	if _, err := os.Stat(databasePath); !os.IsNotExist(err) {
		t.Fatalf("Database() created the file or failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("Database() created the directory: %v", err)
	}
}

func TestEnsureHomeIsOwnerOnly(t *testing.T) {
	home := filepath.Join(t.TempDir(), "data")
	t.Setenv(HomeEnv, home)

	created, err := EnsureHome()
	if err != nil {
		t.Fatalf("EnsureHome: %v", err)
	}
	if created != home {
		t.Fatalf("EnsureHome() = %q, want %q", created, home)
	}

	// Calling twice must stay a no-op.
	if _, err := EnsureHome(); err != nil {
		t.Fatalf("second EnsureHome: %v", err)
	}

	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	info, err := os.Stat(home)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// Captured content is personal data and must not be group or world readable.
	if perm := info.Mode().Perm(); perm != dirPerm {
		t.Fatalf("permissions = %04o, want %04o", perm, dirPerm)
	}
}

func TestPlatformDefaultIsUsedWithoutOverride(t *testing.T) {
	t.Setenv(HomeEnv, "")
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_DATA_HOME", "/xdg-data")
	}

	resolved, err := Home()
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if filepath.Base(resolved) != "muxio" {
		t.Fatalf("Home() = %q, want a muxio directory", resolved)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("Home() = %q, want an absolute path", resolved)
	}
}

func TestConfigFileFollowsHomeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv(HomeEnv, home)

	configPath, err := ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	if want := filepath.Join(home, ConfigFileName); configPath != want {
		t.Fatalf("ConfigFile() = %q, want %q", configPath, want)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("ConfigFile created the file: %v", err)
	}
}

// Without an override, config and data live in separate platform directories.
func TestConfigFileUsesPlatformConfigDirectory(t *testing.T) {
	t.Setenv(HomeEnv, "")

	configPath, err := ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	if filepath.Base(configPath) != ConfigFileName {
		t.Fatalf("ConfigFile() = %q", configPath)
	}
	if !filepath.IsAbs(configPath) {
		t.Fatalf("ConfigFile() = %q, want an absolute path", configPath)
	}
}

func TestEnsureConfigDirIsOwnerOnly(t *testing.T) {
	home := filepath.Join(t.TempDir(), "cfg")
	t.Setenv(HomeEnv, home)

	dir, err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != dirPerm {
		t.Fatalf("permissions = %04o, want %04o", perm, dirPerm)
	}
}
