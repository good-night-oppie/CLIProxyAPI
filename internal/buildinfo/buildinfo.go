// Package buildinfo exposes compile-time metadata shared across the server.
package buildinfo

// ReleaseVersion identifies this fork's release independently of upstream.
const ReleaseVersion = "v7.3.8-oppie.20261023"

// BackportNote describes selected fixes, not upstream release equivalence.
const BackportNote = "Selected fixes backported from CLIProxyAPI v8 (through v8.0.35); not a full v8 upgrade."

// The following variables are overridden via ldflags during release builds.
// Defaults cover local development builds.
var (
	// Version is the semantic version or git describe output of the binary.
	Version = ReleaseVersion

	// Commit is the git commit SHA baked into the binary.
	Commit = "none"

	// BuildDate records when the binary was built in UTC.
	BuildDate = "unknown"
)
