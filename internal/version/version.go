// Package version carries the build identity shared by every binary in the
// toolkit.
//
// It exists so the linker has one target to stamp. When each command declared
// its own `main.Version`, `-X main.Version=…` reached whichever binary was being
// built and silently missed the others — and `-X main.BuildTime=…` missed all of
// them, because two of the three called the field BuildDate.
package version

// Values are overridden at build time via:
//
//	-X github.com/tradik/wpexporter/internal/version.Version=v1.8.3
//
// The defaults are what a `go install` build reports, so they are kept in step
// with the VERSION file rather than left at a placeholder.
var (
	Version   = "1.8.3"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// String renders the full build identity for a --version banner.
func String() string {
	if GitCommit == "unknown" {
		return Version
	}

	return Version + " (" + GitCommit + ", built " + BuildTime + ")"
}
