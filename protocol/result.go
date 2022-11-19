package protocol

import (
	"bufio"
	"io"
)

type ResultType int

const (
	SuccessResultType ResultType = iota
	ErrorResultType
	LogResultType
)

var resultType = [][]byte{
	// Request commands from client to Daemon.
	[]byte("success"),
	[]byte("error"),
	[]byte("log"),
}

func WriteResult(w *bufio.Writer, rt ResultType, args string) error {
	_, err := w.Write(resultType[rt])
	if err != nil {
		return err
	}
	if args != "" {
		err = w.WriteByte(' ')
		if err != nil {
			return err
		}
		_, err = w.WriteString(args)
		if err != nil {
			return err
		}
	}
	err = w.WriteByte('\n')
	if err != nil {
		return err
	}
	return w.Flush()
}

func ReadResult(r io.Reader) (rt ResultType, args string, err error) {
	return // TODO
}
