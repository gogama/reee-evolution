package daemon

import (
	"context"
	"github.com/gofrs/uuid"
	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"net"
	"sync"
)

type cmdContext struct {
	context.Context
	d     *Daemon
	cmdID string
	conn  net.Conn
	args  string
	isEOF bool
	lvl   [3]log.Level
}

func newCmdContext(d *Daemon, conn net.Conn, args string, isEOF bool, lvl log.Level) (cmdContext, error) {
	cmdID, err := uuid.NewV6()
	if err != nil {
		return cmdContext{}, err
	}
	ctx := cmdContext{
		d:     d,
		cmdID: cmdID.String(),
		conn:  conn,
		args:  args,
		isEOF: isEOF,
		lvl:   [3]log.Level{lvl, lvl, log.NormalLevel},
	}
	if leveler, ok := d.Logger.(log.Leveler); ok {
		ctx.lvl[2] = leveler.Level()
		if ctx.lvl[2] > lvl {
			ctx.lvl[0] = ctx.lvl[2]
		}
	}
	return ctx, nil
}

func (ctx *cmdContext) Print(lvl log.Level, msg string) {
	if lvl > ctx.lvl[0] {
		return
	}
	var wg sync.WaitGroup
	if lvl > ctx.lvl[1] {
		wg.Add(1)
		go func() {
			msg = lvl.String() + " " + msg
			err := protocol.WriteResult(ctx.conn, protocol.LogResultType, msg)
			if err != nil {
				log.Normal(ctx.d.Logger, "[cmd %s]: failed to log message: %s", ctx.cmdID, err.Error())
				log.Verbose(ctx.d.Logger, "[cmd %s]: \tlog message: %s", ctx.cmdID, msg)
			}
			wg.Done()
		}()
	}
	if lvl > ctx.lvl[2] {
		ctx.d.Logger.Print(lvl, "[cmd "+ctx.cmdID+"]: "+msg)
	}
	wg.Wait()
}

func (ctx *cmdContext) Level() log.Level {
	return ctx.lvl[0]
}
