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

type ResultType int

const (
	DataResultType ResultType = iota
	LogResultType
)

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

func WriteResult(w io.Writer, rt ResultType, args string) error {
	// TODO: Should newlines be changed to nulls in the args to keep it line-oriented?
	// e.g. use case is multi-line log message
	return nil
}

func ReadResult(r io.Reader) (rt ResultType, args string, err error) {
	return // TODO
}
