package daemon

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"github.com/gogama/reee-evolution/rule"
	"github.com/jhillyerd/enmime"
)

type GroupRecord struct {
	Message   *enmime.Envelope
	Hash      string
	StartTime time.Time
	Group     string
	Rules     []RuleRecord
}

type RuleRecord struct {
	StartTime time.Time
	Rule      string
	Result    int // TODO: Should be a meaningful value.
}

type History interface {
	Record(gr GroupRecord) error
}

type EnvelopeCache interface {
	Get(key string) *enmime.Envelope
	Put(key string, env *enmime.Envelope, size int64)
}

type Daemon struct {
	Listener net.Listener
	Logger   log.Printer // TODO: Should be something that supports verbosity levels.
	Groups   map[string][]rule.Rule
	Hist     History
	Cache    EnvelopeCache
	lock     sync.RWMutex
}

func (d *Daemon) Serve() error {
	// Synchronous, and you can `go d.Serve()` it if you want async.

	defer func() {
		_ = d.Listener.Close()
	}()
	for {
		conn, err := d.Listener.Accept()
		if err != nil {
			// TODO: Log fatal error.
			// d.Logger.Fatal("accept error:", err)
		}

		go d.handle(conn)
	}
}

func (d *Daemon) Shutdown(ctx context.Context) error {
	// Analogous to http.Server's Shutdown.
	return nil
}

func (d *Daemon) handle(conn net.Conn) {
	cmd, lvl, args, err := protocol.ReadCommand(conn)
	var isEOF bool
	if err == io.EOF {
		isEOF = true
	} else if err != nil {
		// TODO: Do something drastic
	}

	ctx, err := newCmdContext(d, conn, args, isEOF, lvl)
	if err != nil {
		// TODO: do something
	}

	switch cmd {
	case protocol.ListCommand:
		err = handleList(&ctx)
	case protocol.EvalCommand:
		err = handleEval(&ctx)
	default:
		// TODO: Unsupported command.
	}
}

func handleList(ctx *cmdContext) error {
	if len(ctx.args) > 0 {
		// TODO: error
	} else if ctx.isEOF {
		// TODO: error
	}

	for group, rules := range ctx.d.Groups {
		_, err := ctx.conn.Write([]byte(group))
		if err != nil {
			// TODO
		}
		for _, r := range rules {
			_, err = ctx.conn.Write([]byte(" "))
			if err != nil {
				// TODO
			}
			_, err = ctx.conn.Write([]byte(r.String()))
			if err != nil {
				// TODO
			}
		}
		_, err = ctx.conn.Write([]byte("\n"))
		if err != nil {
			// TODO
		}
	}

	return nil
}

func handleEval(ctx *cmdContext) error {
	// TODO: Input should be something like: eval <group> <verbosity> [<rule>]
	// TODO: Output should be lines like "log [text]" or "error [text]" or "success [code]"

	// var verbosity int TODO
	var g, r string
	var rules []rule.Rule
	var ok bool

	if rules, ok = ctx.d.Groups[g]; !ok {
		// TODO
	}

	if r != "" {
		for i := range rules {
			if r == rules[i].String() {
				rules[0] = rules[i]
				rules = rules[0:]
				r = ""
			}
		}
		if r == "" {
			// TODO: error rule not found
		}
	}

	var buf bytes.Buffer
	if !ctx.isEOF {
		_, err := io.Copy(&buf, ctx.conn)
		if err != nil {
			// TODO
		}
	}

	msg, err := parseMsg(ctx, &buf)
	if err != nil {
		// TODO:
	}

	for i := range rules {
		// TODO: Give the rule a logger
		err = rules[i].Eval(msg)
		// If it is code say 0, continue otherwise
	}

	return nil
}

func getMsg(ctx *cmdContext, key string) *enmime.Envelope {
	ctx.d.lock.RLock()
	defer ctx.d.lock.RUnlock()
	return ctx.d.Cache.Get(key)
}

func parseMsg(ctx *cmdContext, buf *bytes.Buffer) (*enmime.Envelope, error) {
	key := fmt.Sprintf("%x", md5.Sum(buf.Bytes()))
	msg := getMsg(ctx, key)
	if msg == nil {
		var err error
		msg, err = enmime.NewParser().ReadEnvelope(buf)
		if err != nil {
			return nil, err
		}
		ctx.d.lock.Lock()
		defer ctx.d.lock.Unlock()
		ctx.d.Cache.Put(key, msg, int64(buf.Len()))
	}
	return msg, nil
}

func parseEvalArgs(args []string) (g, r string, err error) {
	return
}
