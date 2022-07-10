package version

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "unknown"
)

// String returns a human readable version string.
func String() string {
	return fmt.Sprintf("boring-registry %s (%s) (build date: %s)", Version, Commit, Date)
}
