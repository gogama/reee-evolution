package main

import (
	"bufio"
	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"io"
	"net"
	"time"
)

type evalCommand struct {
	Group string `arg:"positional,required" help:"rule group to evaluate"`
	Rule  string `arg:"positional" help:"optional rule to evaluate within group"`
}

func (cmd *evalCommand) Validate() error {
	// TODO: Make sure group and rule names don't have spacey characters.
	return nil
}

func (cmd *evalCommand) Exec(cmdID string, logger log.Printer, ins io.Reader, _ io.Writer, conn net.Conn) error {
	pc := protocol.Command{
		Type:  protocol.EvalCommandType,
		ID:    cmdID,
		Level: log.LevelOf(logger),
		Args:  cmd.Group,
	}
	if len(cmd.Rule) > 0 {
		pc.Args += " " + cmd.Rule
	}
	w := bufio.NewWriter(conn)

	start := time.Now()
	err := protocol.WriteCommand(w, pc)
	if err != nil {
		return err
	}
	elapsed := time.Since(start)
	log.Verbose(logger, "wrote %s command for command ID %s in %d.", protocol.EvalCommandType, cmdID, elapsed)

	start = time.Now()
	n, err := io.Copy(w, ins)
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	log.Verbose(logger, "copied %d bytes from input to connection in %d.", n, elapsed)

	// TODO: Read back log messages.

	return nil
}
