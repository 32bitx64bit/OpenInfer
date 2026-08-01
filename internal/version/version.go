// Package version is the single source of truth for the OpenInfer Studio
// release version. Packaging scripts and CMake read VERSION from this
// directory; Go embeds it. Optional Commit/Date are set via -ldflags at
// release build time for updater diagnostics.
package version

import (
	_ "embed"
	"fmt"
	"runtime"
	"strings"
)

//go:embed VERSION
var raw string

// Commit and Date may be overwritten at link time:
//
//	-ldflags "-X github.com/openinfer/openinfer-studio/internal/version.Commit=abc123 \
//	          -X github.com/openinfer/openinfer-studio/internal/version.Date=2026-08-01"
var (
	Commit = "dev"
	Date   = "unknown"
)

// Version returns the semver release string (e.g. "0.1.0").
func Version() string {
	return strings.TrimSpace(raw)
}

// UserAgent is a short product/version token for outbound HTTP.
func UserAgent() string {
	return "OpenInferStudio/" + Version()
}

// Summary is a one-line build identity for logs and --version.
func Summary() string {
	return fmt.Sprintf("OpenInfer Studio %s (%s/%s commit=%s built=%s)",
		Version(), runtime.GOOS, runtime.GOARCH, Commit, Date)
}

// Info is the JSON-friendly payload exposed by GET /api/v1/status.
func Info() map[string]any {
	return map[string]any{
		"version": Version(),
		"commit":  Commit,
		"date":    Date,
		"goos":    runtime.GOOS,
		"goarch":  runtime.GOARCH,
	}
}
