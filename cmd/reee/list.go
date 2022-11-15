package main

import "net"

type listCommand struct {
}

func (cmd *listCommand) Exec(conn net.Conn) error {
	return nil
}
