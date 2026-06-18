// Package version provides build-time version information injected via ldflags.
//
// Build with:
//
//	go build -ldflags "-X github.com/Malong11-007/web-search-mcp/internal/version.Version=v0.2.1 -X .../version.Commit=$(git rev-parse --short HEAD) -X .../version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./cmd/web-search-mcp/
package version

import "fmt"

// These variables are set via -ldflags at build time. If unset, they default to
// "dev" values so development builds are clearly identifiable.
var (
	// Version is the semantic version tag (e.g. "v0.2.1"). Set from git tags.
	Version = "dev"

	// Commit is the short git SHA of the build. Set from $(git rev-parse --short HEAD).
	Commit = "unknown"

	// BuildDate is the UTC timestamp of the build. Set from $(date -u +%Y-%m-%dT%H:%M:%SZ).
	BuildDate = "unknown"
)

// String returns a human-readable version string.
func String() string {
	return fmt.Sprintf("web-search-mcp %s (commit %s, built %s)", Version, Commit, BuildDate)
}

// Short returns just the version number.
func Short() string {
	return Version
}
