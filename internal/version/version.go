package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version of the build (set by ldflags)
	Version = "dev"

	// Commit is the git commit hash (set by ldflags)
	Commit = "none"

	// Date is the build date (set by ldflags)
	Date = "unknown"

	// BuiltBy is who built the binary (set by ldflags)
	BuiltBy = "unknown"
)

// Info returns the complete version information
func Info() string {
	return "Version: " + Version + "\n" +
		"Commit: " + Commit + "\n" +
		"Built: " + Date + "\n" +
		"Built by: " + BuiltBy + "\n" +
		"Go version: " + runtime.Version() + "\n" +
		"OS/Arch: " + runtime.GOOS + "/" + runtime.GOARCH + "\n"
}

// Short returns a short version string
func Short() string {
	if Version == "dev" {
		return "dev"
	}
	return fmt.Sprintf("v%s", Version)
}
