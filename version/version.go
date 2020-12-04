package version

import "fmt"

var (
	Version string = "dev"
	Commit  string = "none"
	Date    string = "unknown"
	BuiltBy string = "unknown"
)

// String returns a human readable version string.
func String() string {
	return fmt.Sprintf("boring-registry %s (%s) (build date: %s)", Version, Commit, Date)
}
