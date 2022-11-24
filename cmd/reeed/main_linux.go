//go:build !windows
// +build !windows

package main

import (
	"errors"
	"os"

	"github.com/gogama/reee-evolution/log"
)

func cleanupSocket(logger log.Printer, network, addr string) {
	if network != "unix" {
		return
	} else if _, err := os.Stat(addr); err == nil {
		log.Verbose(logger, "removing old domain socket [%s]...", addr)
		err = os.Remove(addr)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Normal(logger, "warning: failed to remove old domain socket [%s]: %s", addr, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Verbose(logger, "can't stat domain socket file [%s]: %s", addr, err)
	}
}
