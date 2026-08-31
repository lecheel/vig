package version

import "fmt"

// Build-time variables injected via -ldflags:
//
//	-X github.com/firstrow/wig/version.Version=<git tag>
//	-X github.com/firstrow/wig/version.CommitHash=<short sha>
//	-X github.com/firstrow/wig/version.Dirty=<true|false>
//
// They default to "" / "unknown" / "false" when the binary is built
// without the ldflags (e.g. plain `go build`), so the editor still
// runs and reports a sane value instead of panicking.
var (
	Version    = ""
	CommitHash = "unknown"
	Dirty      = "false"
)

// String returns a human-readable version string.
// If a git tag is present at the build commit, it takes precedence
// and is shown alongside the short commit hash.
//
// Examples:
//
//	wig 1.0 (abc1234)
//	wig 1.0-dirty (abc1234)
//	wig abc1234
//	wig abc1234-dirty
func String() string {
	var versionPart string
	if Version != "" {
		versionPart = Version
	} else {
		versionPart = CommitHash
	}

	var dirtyPart string
	if Dirty == "true" {
		dirtyPart = "-dirty"
	}

	if Version != "" {
		return fmt.Sprintf("wig %s (%s%s)", versionPart, CommitHash, dirtyPart)
	}
	return fmt.Sprintf("wig %s%s", versionPart, dirtyPart)
}
