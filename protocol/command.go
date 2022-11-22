package protocol

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/gogama/reee-evolution/log"
)

type CommandType int

const (
	EvalCommandType CommandType = iota
	ListCommandType
)

func (t CommandType) String() string {
	return commandType[t]
}

var commandType = []string{
	"eval",
	"list",
}

type Command struct {
	Type  CommandType
	ID    string
	Level log.Level
	Args  string
}

func WriteCommand(w *bufio.Writer, cmd Command) error {
	_, err := w.WriteString(commandType[cmd.Type])
	if err != nil {
		return err
	}
	err = w.WriteByte(' ')
	if err != nil {
		return err
	}
	_, err = w.WriteString(cmd.ID)
	if err != nil {
		return err
	}
	err = w.WriteByte(' ')
	if err != nil {
		return err
	}
	b, err := cmd.Level.MarshalText()
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	if err != nil {
		return err
	}
	if cmd.Args != "" {
		err = w.WriteByte(' ')
		if err != nil {
			return err
		}
		_, err = w.WriteString(cmd.Args)
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

func ReadCommand(r *bufio.Reader) (cmd Command, err error) {
	line, err := r.ReadBytes('\n')
	if err == io.EOF {
		err = fmt.Errorf("protocol: read command: premature EOF before EOL after %d bytes", len(line))
		return
	} else if err != nil {
		return
	}
	rem := line
	line = line[0 : len(line)-1]
	// Isolate the command type.
	p := bytes.IndexByte(rem, ' ')
	if p < 1 {
		err = fmt.Errorf("protocol: read command: unfinished command type in [%s]", line)
		return
	}
	t := rem[0:p]
	cmd.Type = -1
	for i := range commandType {
		if string(t) == commandType[i] {
			cmd.Type = CommandType(i)
		}
	}
	if cmd.Type < 0 {
		err = fmt.Errorf("protocol: read command: invalid command type [%s] in [%s]", t, line)
	}
	rem = rem[p+1:]
	// Isolate the command ID.
	p = bytes.IndexByte(rem, ' ')
	if p < 1 {
		err = fmt.Errorf("protocol: read command: unfinished command ID in [%s]", line)
		return
	}
	cmd.ID = string(rem[0:p])
	rem = rem[p+1:]
	// Isolate the requested log level.
	p = bytes.IndexByte(rem, ' ')
	if p < 1 {
		p = len(rem) - 1
		if p < 1 {
			err = fmt.Errorf("protocol: read command: unfinished log level in [%s]", line)
			return
		}
	}
	err = cmd.Level.UnmarshalText(rem[0:p])
	if err != nil {
		err = fmt.Errorf("protocol: read command: invalid log level [%s] in [%s]", rem[0:p], line)
		return
	}
	rem = rem[p+1 : len(rem)-1] // Truncate newline
	// Isolate the arguments.
	cmd.Args = string(rem)
	return
}
