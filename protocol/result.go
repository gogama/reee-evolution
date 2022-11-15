package protocol

import "io"

type ResultType int

const (
	DataResultType ResultType = iota
	LogResultType
)

func WriteResult(w io.Writer, rt ResultType, args string) error {
	// TODO: Should newlines be changed to nulls in the args to keep it line-oriented?
	// e.g. use case is multi-line log message
	return nil
}

func ReadResult(r io.Reader) (rt ResultType, args string, err error) {
	return // TODO
}
