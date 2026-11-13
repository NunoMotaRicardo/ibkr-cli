// Package version holds build metadata (set via -ldflags) and the provider
// compatibility line. Provider values are mirrored in specs/provider/compatibility.md.
package version

import "fmt"

// Set by goreleaser / `go build -ldflags "-X ..."`.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Provider compatibility: the IBKR connector version this CLI was last verified against.
const (
	ProviderName     = "IBKR MCP connector"
	ProviderVersion  = "1.2.1"
	ProviderVerified = "2026-11-12"
)

// Line is the one-line build description.
func Line() string { return fmt.Sprintf("ibkr %s (commit %s, built %s)", Version, Commit, Date) }

// Compat is the provider compatibility line.
func Compat() string {
	return fmt.Sprintf("compatible with %s %s (verified %s)", ProviderName, ProviderVersion, ProviderVerified)
}
