package main

import (
	"github.com/gogama/reee-evolution/log"
	"io"
	"net"
)

type listCommand struct {
}

func (cmd *listCommand) Validate() error {
	return nil
}

func (cmd *listCommand) Exec(cmdID string, logger log.Printer, ins io.Reader, outs io.Writer, conn net.Conn) error {
	return nil
}
