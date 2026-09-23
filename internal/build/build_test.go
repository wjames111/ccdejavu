package build

import "testing"

// TestEffectiveVersion mutates the package-level Version var that -ldflags
// would otherwise set, so it can't run in parallel with itself.
//
//nolint:paralleltest
func TestEffectiveVersion(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "v1.2.3"
	if got, want := effectiveVersion(), "v1.2.3"; got != want {
		t.Errorf("effectiveVersion() = %q, want %q", got, want)
	}

	// With the "dev" sentinel, it falls back to debug.ReadBuildInfo. Under
	// `go test` that reports Main.Version as "(devel)", so it stays "dev".
	Version = "dev"
	if got, want := effectiveVersion(), "dev"; got != want {
		t.Errorf("effectiveVersion() = %q, want %q", got, want)
	}
}
