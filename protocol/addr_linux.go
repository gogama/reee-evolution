//go:build !windows
// +build !windows

package protocol

// DefaultNetAddr returns the default network and address for
// communication with the daemon.
func DefaultNetAddr() (network, address string) {
	return "unix", "/tmp/reee_" + DefaultTCPPort + ".sock"
}
