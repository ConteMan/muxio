package version

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns the human-readable build identity.
func String() string {
	if Version == "dev" {
		return "muxio dev"
	}
	return fmt.Sprintf("muxio %s (%s, %s)", Version, Commit, Date)
}
