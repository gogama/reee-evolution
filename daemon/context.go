package daemon

import (
	"bufio"
	"context"
	"fmt"
	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"sync"
)

type cmdContext struct {
	context.Context
	d         *Daemon
	connID    uint64
	r         *bufio.Reader
	w         *bufio.Writer
	cmdID     string
	args      string
	isEOF     bool
	logPrefix string
	lvl       [3]log.Level
}

func newCmdContext(d *Daemon, connID uint64, r *bufio.Reader, w *bufio.Writer, cmd protocol.Command, isEOF bool) cmdContext {
	ctx := cmdContext{
		d:         d,
		connID:    connID,
		r:         r,
		w:         w,
		cmdID:     cmd.ID,
		args:      cmd.Args,
		isEOF:     isEOF,
		logPrefix: fmt.Sprintf("[conn %d, cmd %s]: ", connID, cmd.ID),
		lvl:       [3]log.Level{cmd.Level, cmd.Level, log.NormalLevel},
	}
	if leveler, ok := d.Logger.(log.Leveler); ok {
		ctx.lvl[2] = leveler.Level()
		if ctx.lvl[2] > cmd.Level {
			ctx.lvl[0] = ctx.lvl[2]
		}
	}
	return ctx
}

func (ctx *cmdContext) Normal(format string, v ...interface{}) {
	if log.NormalLevel > ctx.lvl[0] {
		return
	}
	ctx.Print(log.NormalLevel, fmt.Sprintf(ctx.logPrefix+format, v...))
}

func (ctx *cmdContext) Verbose(format string, v ...interface{}) {
	if log.VerboseLevel > ctx.lvl[0] {
		return
	}
	ctx.Print(log.VerboseLevel, fmt.Sprintf(ctx.logPrefix+format, v...))
}

func (ctx *cmdContext) Print(lvl log.Level, msg string) {
	if lvl > ctx.lvl[0] {
		return
	}
	var wg sync.WaitGroup
	if lvl <= ctx.lvl[1] {
		wg.Add(1)
		go func() {
			msg = lvl.String() + " " + msg
			err := protocol.WriteResult(ctx.w, protocol.LogResultType, msg)
			if err != nil {
				log.Normal(ctx.d.Logger, "[conn %d, cmd %s]: failed to log message: %s", ctx.connID, ctx.cmdID, err.Error())
				log.Verbose(ctx.d.Logger, "[conn %d, cmd %s]: \tlog message: %s", ctx.connID, ctx.cmdID, msg)
			}
			wg.Done()
		}()
	}
	if lvl <= ctx.lvl[2] {
		ctx.d.Logger.Print(lvl, msg)
	}
	wg.Wait()
}

func (ctx *cmdContext) Level() log.Level {
	return ctx.lvl[0]
}
