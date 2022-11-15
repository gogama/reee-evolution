package version

import (
	"os"
	"path/filepath"
	"runtime/debug"
)

// OfCmd returns a version string for the currently executing command.
func OfCmd() string {
	name := filepath.Base(os.Args[0])
	version := "(unknown)"
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		version = buildInfo.Main.Version
	}
	return name + " " + version
}
