// Package build carries build-time metadata injected via -ldflags.
package build

import "runtime"

var (
	Version = "0.0.0-dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)

func GoVersion() string { return runtime.Version() }
