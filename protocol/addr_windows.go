//go:build windows
// +build windows

package protocol

// DefaultNetAddr returns the default network and address for
// communication with the daemon.
func DefaultNetAddr() (network, address string) {
	return "tcp", "127.0.0.1:" + DefaultTCPPort
}
