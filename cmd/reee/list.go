package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"io"
	"time"
)

type listCommand struct {
}

func (cmd *listCommand) Validate() error {
	return nil
}

func (cmd *listCommand) Exec(cmdID string, logger log.Printer, _ io.Reader, outs io.Writer, r *bufio.Reader, w *bufio.Writer) error {
	pc := protocol.Command{
		Type:  protocol.ListCommandType,
		ID:    cmdID,
		Level: log.LevelOf(logger),
	}

	start := time.Now()
	err := protocol.WriteCommand(w, pc)
	if err != nil {
		return err
	}
	elapsed := time.Since(start)
	log.Verbose(logger, "wrote %s command for cmd %s in %s.", protocol.ListCommandType, cmdID, elapsed)

	start = time.Now()
	rst, err := protocol.ReadResult(logger, r)
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	log.Verbose(logger, "read %s result and %d bytes of data in %s.", rst.Type, len(rst.Data), elapsed)

	switch rst.Type {
	case protocol.SuccessResultType:
		_, err = outs.Write(rst.Data)
		return err
	case protocol.ErrorResultType:
		return errors.New(string(rst.Data))
	default:
		panic(fmt.Sprintf("reee: unhandled result type: %d", rst.Type))
	}
}
