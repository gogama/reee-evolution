package protocol

import (
	"io"

	"github.com/gogama/reee-evolution/log"
)

type Command int

const (
	ListCommand Command = iota
	EvalCommand
)

var command = [][]byte{
	// Request commands from client to Daemon.
	[]byte("list"),
	[]byte("eval"),
}

func WriteCommand(w io.Writer, cmd Command, args string) error {
	_, err := w.Write(command[cmd])
	if err != nil {
		return err
	}
	if args != "" {
		_, err = w.Write([]byte(args))
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte("\n"))
	return err
}

func ReadCommand(r io.Reader) (cmd Command, lvl log.Level, args string, err error) {
	return // TODO
}
