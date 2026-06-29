package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Version is the adev version. It defaults to "dev" and is overridden at release
// build time via -ldflags "-X agentic-developer/internal/cli.Version=vX.Y.Z".
var Version = "dev"

// resolveVersion returns the best-known version: the ldflags value when set,
// otherwise the module version from the build info (populated when adev is
// installed with `go install ...@vX.Y.Z`), falling back to "dev".
func resolveVersion() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return Version
}

// printVersion writes the version and platform to stdout.
func printVersion() {
	fmt.Printf("adev %s (%s/%s)\n", resolveVersion(), runtime.GOOS, runtime.GOARCH)
}
