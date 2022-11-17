package main

import (
	"github.com/gogama/reee-evolution/log"
	"net"
)

type listCommand struct {
}

func (cmd *listCommand) Validate() error {
	return nil
}

func (cmd *listCommand) Exec(cmdID string, logger log.Printer, conn net.Conn) error {
	return nil
}
