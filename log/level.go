package log

import (
	"bytes"
	"fmt"
)

type Level int

type Leveler interface {
	Level() Level
}

const (
	TaciturnLevel Level = iota - 1
	NormalLevel
	VerboseLevel
)

var level = [][]byte{
	// Request commands from client to Daemon.
	[]byte("taciturn"),
	[]byte("normal"),
	[]byte("verbose"),
}

func (lvl *Level) String() string {
	if text, err := lvl.MarshalText(); err == nil {
		return string(text)
	}
	return ""
}

func (lvl *Level) MarshalText() (text []byte, err error) {
	i := int(*lvl) + 1
	if 0 <= i && i < len(level) {
		text = level[i]
	} else {
		err = fmt.Errorf("log: invalid level value: %d", lvl)
	}
	return
}

func (lvl *Level) UnmarshalText(text []byte) error {
	for i := range level {
		if bytes.Equal(level[i], text) {
			*lvl = Level(i - 1)
			return nil
		}
	}
	return fmt.Errorf("log: invalid level text: %q", text)
}
