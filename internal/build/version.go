// Package build provides build-time metadata injected via -ldflags.
package build

// Version is set at build time via:
//
//	-ldflags "-X github.com/seandheath/koboctl/internal/build.Version=v1.2.3"
//
// Falls back to "dev" when built without ldflags (e.g. go run, go test).
var Version = "dev"
