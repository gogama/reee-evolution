//go:build windows
// +build windows

package main

import "github.com/gogama/reee-evolution/log"

func cleanupSocket(logger log.Printer, network, addr string) {
	// No-op on windows.
}
