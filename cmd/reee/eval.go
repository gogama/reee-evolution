package main

import (
	"bufio"
	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"net"
	"strings"
)

type evalCommand struct {
	Group string   `arg:"positional,required" help:"rule group to evaluate"`
	Rule  []string `arg:"positional" help:"optional names of rules to evaluate within group"`
}

func (cmd *evalCommand) Validate() error {
	// TODO: Make sure group and rule names don't have spacey characters.
	return nil
}

func (cmd *evalCommand) Exec(cmdID string, logger log.Printer, conn net.Conn) error {
	pc := protocol.Command{
		Type:  protocol.EvalCommandType,
		ID:    cmdID,
		Level: log.LevelOf(logger),
		Args:  cmd.Group,
	}
	if len(cmd.Rule) > 0 {
		pc.Args += " " + strings.Join(cmd.Rule, " ")
	}
	w := bufio.NewWriter(conn)
	err := protocol.WriteCommand(w, pc)
	if err != nil {
		return err
	}
	log.Verbose(logger, "wrote %s command for command ID %s...", protocol.EvalCommandType, cmdID)
	return nil
}
