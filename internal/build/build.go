// Package build carries version metadata injected at link time.
package build

import "runtime/debug"

// Version and Date are set via -ldflags by Taskfile.yml and GoReleaser.
var (
	Version = "dev"
	Date    = "unknown"
)

// String renders the version for `ccdejavu --version`.
func String() string { return effectiveVersion() + " (" + Date + ")" }

// effectiveVersion falls back to the module version recorded by `go install`
// when Version wasn't set via -ldflags, so `go install ...@latest` reports
// something more useful than "dev".
func effectiveVersion() string {
	if Version != "dev" {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return Version
	}
	return info.Main.Version
}
