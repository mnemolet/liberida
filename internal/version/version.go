package version

import (
	"fmt"
	"runtime"
	"strings"
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
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Version: %s\n", Version))
	sb.WriteString(fmt.Sprintf("Commit: %s\n", Commit))
	sb.WriteString(fmt.Sprintf("Built: %s\n", Date))
	sb.WriteString(fmt.Sprintf("Built by: %s\n", BuiltBy))
	sb.WriteString(fmt.Sprintf("Go version: %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH))

	return sb.String()
}

// Short returns a short version string
func Short() string {
	if Version == "dev" {
		return "dev"
	}
	return fmt.Sprintf("v%s", Version)
}
