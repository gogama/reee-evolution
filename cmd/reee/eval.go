package main

import "net"

type evalCommand struct {
	Group string   `arg:"positional,required" help:"rule group to evaluate"`
	Rule  []string `arg:"positional" help:"optional names of rules to evaluate within group"`
}

func (cmd *evalCommand) Exec(conn net.Conn) error {
	return nil
}
