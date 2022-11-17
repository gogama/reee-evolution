//go:build !windows
// +build !windows

package main

import (
	"context"
	"os"
	"os/signal"
)

func signalContext(parent context.Context) (ctx context.Context, stop context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
}
