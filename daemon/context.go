package daemon

import (
	"bufio"
	"context"
	"fmt"
	"sync"

	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
)

type cmdContext struct {
	ctx       context.Context
	cancel    context.CancelFunc
	d         *Daemon
	connID    uint64
	r         *bufio.Reader
	w         *bufio.Writer
	cmdID     string
	args      string
	isEOF     bool
	logPrefix string
	logErr    error
	lvl       [3]log.Level
}

func newCmdContext(d *Daemon, connID uint64, r *bufio.Reader, w *bufio.Writer, cmd protocol.Command, isEOF bool) cmdContext {
	childCtx, cancel := context.WithCancel(d.ctx)
	ctx := cmdContext{
		ctx:       childCtx,
		cancel:    cancel,
		d:         d,
		connID:    connID,
		r:         r,
		w:         w,
		cmdID:     cmd.ID,
		args:      cmd.Args,
		isEOF:     isEOF,
		logPrefix: fmt.Sprintf("[conn %d, cmd %s] ", connID, cmd.ID),
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
	ctx.Print(log.NormalLevel, fmt.Sprintf(format, v...))
}

func (ctx *cmdContext) Verbose(format string, v ...interface{}) {
	if log.VerboseLevel > ctx.lvl[0] {
		return
	}
	ctx.Print(log.VerboseLevel, fmt.Sprintf(format, v...))
}

func (ctx *cmdContext) Print(lvl log.Level, msg string) {
	if lvl > ctx.lvl[0] {
		return
	}
	prefixedMsg := ctx.logPrefix + msg
	var wg sync.WaitGroup
	if lvl <= ctx.lvl[1] && ctx.logErr == nil {
		wg.Add(1)
		go func() {
			// FIXME: Need to truncate prefixedMsg at first newline to prevent protocol breakage.
			err := protocol.WriteLog(ctx.w, lvl, prefixedMsg)
			if err != nil {
				ctx.logErr = err
				log.Normal(ctx.d.Logger, "[conn %d, cmd %s]: failed to log message: %s", ctx.connID, ctx.cmdID, err.Error())
				log.Verbose(ctx.d.Logger, "[conn %d, cmd %s]: \tlog message: %s", ctx.connID, ctx.cmdID, msg)
			}
			wg.Done()
		}()
	}
	if lvl <= ctx.lvl[2] {
		ctx.d.Logger.Print(lvl, prefixedMsg)
	}
	wg.Wait()
}

func (ctx *cmdContext) Level() log.Level {
	return ctx.lvl[0]
}
