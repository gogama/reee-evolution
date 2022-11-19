package protocol

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/gogama/reee-evolution/log"
	"io"
	"strconv"
)

type ResultType int

func (t ResultType) String() string {
	return resultType[t]
}

const (
	SuccessResultType ResultType = iota
	ErrorResultType
	logResultType
)

var resultType = []string{
	"success",
	"error",
	"log",
}

type Result struct {
	Type ResultType
	Data []byte
}

func WriteSuccess(w *bufio.Writer, data []byte) error {
	_, err := w.WriteString(resultType[SuccessResultType])
	if err != nil {
		return err
	}
	err = w.WriteByte(' ')
	if err != nil {
		return err
	}
	n := strconv.Itoa(len(data))
	_, err = w.WriteString(n)
	if err != nil {
		return err
	}
	err = w.WriteByte('\n')
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	w.Flush()
	return nil
}

func WriteError(w *bufio.Writer, msg string) error {
	_, err := w.WriteString(resultType[ErrorResultType])
	if err != nil {
		return err
	}
	err = w.WriteByte(' ')
	if err != nil {
		return err
	}
	_, err = w.WriteString(msg)
	if err != nil {
		return err
	}
	w.Flush()
	return nil
}

func WriteLog(w *bufio.Writer, lvl log.Level, msg string) error {
	_, err := w.WriteString(resultType[logResultType])
	if err != nil {
		return err
	}
	err = w.WriteByte(' ')
	if err != nil {
		return err
	}
	b, err := lvl.MarshalText()
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	if err != nil {
		return err
	}
	err = w.WriteByte(' ')
	if err != nil {
		return err
	}
	_, err = w.WriteString(msg)
	if err != nil {
		return err
	}
	return w.WriteByte('\n')
}

func ReadResult(logger log.Printer, r *bufio.Reader) (rst Result, err error) {
	for {
		var line []byte
		line, err = r.ReadBytes('\n')
		if err == io.EOF {
			err = fmt.Errorf("protocol: read result: premature EOF before EOL after %d bytes", len(line))
		}
		if err != nil {
			return
		}
		rem := line
		line = line[0 : len(line)-1]
		// Isolate the result type.
		p := bytes.IndexByte(rem, ' ')
		if p < 1 {
			p = len(rem) - 1
		}
		if p < 1 {
			err = fmt.Errorf("protocol: read result: missing result type in [%s]", line)
			return
		}
		t := rem[0:p]
		var rt ResultType = -1
		for i := range resultType {
			if string(t) == resultType[i] {
				rt = ResultType(i)
			}
		}
		rem = rem[p+1:]
		switch rt {
		case SuccessResultType:
			err = readSuccess(r, rem, line, &rst)
			return
		case ErrorResultType:
			err = readError(rem, &rst)
			return
		case logResultType:
			var lvl log.Level
			var msg string
			lvl, msg, err = readLog(rem, line)
			if err != nil {
				return
			}
			logger.Print(lvl, msg)
			continue
		default:
			err = fmt.Errorf("protocol: read result: invalid result type [%s] in [%s]", t, line)
			return
		}
	}
}

func readSuccess(r io.Reader, rem, line []byte, rst *Result) error {
	rem = rem[:len(rem)-1] // Truncate newline
	n, err := strconv.Atoi(string(rem))
	if err != nil {
		return fmt.Errorf("protocol: read result: invalid data length [%s] in [%s]", rem, line)
	}
	b := make([]byte, n)
	m := 0
	for {
		o, err := r.Read(b[m:])
		m += o
		if m == n {
			rst.Type = SuccessResultType
			rst.Data = b
			return nil
		} else if err == io.EOF {
			return fmt.Errorf("protocol: read result: only %d/%d expected data bytes found after [%s]", m, n, line)
		} else if err != nil {
			return err
		}
	}
}

func readError(rem []byte, rst *Result) error {
	rst.Type = ErrorResultType
	rst.Data = rem[0 : len(rem)-1] // Truncate newline
	return nil
}

func readLog(rem, line []byte) (lvl log.Level, msg string, err error) {
	p := bytes.IndexByte(rem, ' ')
	if p < 1 {
		p = len(rem) - 1
		if p < 1 {
			err = fmt.Errorf("protocol: read result: unfinished log level in [%s]", line)
			return
		}
	}
	err = lvl.UnmarshalText(rem[0:p])
	if err != nil {
		err = fmt.Errorf("protocol: read result: invalid log level [%s] in [%s]", rem[0:p], line)
		return
	}
	msg = string(rem[p+1 : len(rem)-1]) // Truncate newline
	return
}
