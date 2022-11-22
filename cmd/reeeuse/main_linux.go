//go:build !windows
// +build !windows

package reeeuse

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func SignalContext(parent context.Context) (ctx context.Context, stop context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
}
