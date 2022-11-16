package log

import (
	"io"
	"sync"
)

type withWriter struct {
	lvl Level
	mu  sync.Mutex
	w   io.Writer
}

func WithWriter(lvl Level, w io.Writer) Printer {
	if w == nil {
		panic("log: nil writer")
	}
	return &withWriter{lvl: lvl, w: w}
}

var newline = []byte{'\n'}

func (w *withWriter) Print(lvl Level, msg string) {
	if w.lvl < lvl {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.w.Write([]byte(msg))
	if err == nil && len(msg) > 0 && msg[len(msg)-1] != '\n' {
		_, _ = w.w.Write(newline)
	}
}

func (w *withWriter) Level() Level {
	return w.lvl
}
