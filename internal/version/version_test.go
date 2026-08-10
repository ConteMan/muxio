package version

import "testing"

func TestStringDevBuild(t *testing.T) {
	restore := swap("dev", "none", "unknown")
	defer restore()

	if got := String(); got != "muxio dev" {
		t.Fatalf("String() = %q", got)
	}
}

func TestStringReleaseBuild(t *testing.T) {
	restore := swap("v0.1.0", "abc1234", "2026-08-10T00:00:00Z")
	defer restore()

	want := "muxio v0.1.0 (abc1234, 2026-08-10T00:00:00Z)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// swap overrides the linker-injected build identity and returns a restore func.
func swap(version, commit, date string) func() {
	previous := [3]string{Version, Commit, Date}
	Version, Commit, Date = version, commit, date
	return func() {
		Version, Commit, Date = previous[0], previous[1], previous[2]
	}
}
