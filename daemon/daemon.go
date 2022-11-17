package daemon

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
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
	Logger   log.Printer
	Groups   map[string][]rule.Rule
	Hist     History
	Cache    EnvelopeCache

	lock      sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	numConns  atomic.Int64
	closeOnce sync.Once
	closeErr  error
}

var ErrStopped = errors.New("daemon: stopped")

func (d *Daemon) Serve() error {
	d.init()
	defer func() {
		_ = d.close()
	}()
	var connID uint64
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := d.Listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Normal(d.Logger, "daemon: accept error: %v; retrying in %v", err, tempDelay)
				timer := time.NewTimer(tempDelay)
				select {
				case <-timer.C:
					continue
				case <-d.ctx.Done():
					timer.Stop()
					return ErrStopped
				}
			}
			return err
		}
		d.numConns.Add(1)
		tempDelay = 0
		go d.handle(connID, conn)
		connID++
	}
}

func (d *Daemon) close() error {
	d.closeOnce.Do(func() {
		d.closeErr = d.Listener.Close()
	})
	return d.closeErr
}

func (d *Daemon) Stop(ctx context.Context) error {
	d.cancel()
	err := d.close()

	timer := time.NewTimer(5 * time.Millisecond)
	defer timer.Stop()
	for d.numConns.Load() > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			timer.Reset(5 * time.Millisecond)
		}
	}

	return err
}

func (d *Daemon) init() {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.ctx != nil {
		panic("daemon: reused")
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
}

func (d *Daemon) handle(connID uint64, conn net.Conn) {
	defer func() {
		_ = conn.Close()
		d.numConns.Add(-1)
	}()

	r := bufio.NewReader(conn)

	cmd, err := protocol.ReadCommand(r)
	var isEOF bool
	if err == io.EOF {
		isEOF = true
	} else if err != nil {
		log.Normal(d.Logger, "error: [conn %d]: %s", connID, err)
		return
	}

	ctx := newCmdContext(d, conn, cmd, isEOF)
	log.Verbose(d.Logger, "[conn %d, cmd %s]: received %v", connID, cmd.ID, cmd)

	switch cmd.Type {
	case protocol.ListCommandType:
		err = handleList(&ctx)
	case protocol.EvalCommandType:
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
		err = rules[i].Eval(ctx, ctx, msg)
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
	//key := fmt.Sprintf("%x", md5.Sum(buf.Bytes()))
	// FIXME: Ensure cache works. Line below commented out because currently cache is always nil.
	// msg := getMsg(ctx, key)
	var msg *enmime.Envelope
	if msg == nil {
		var err error
		msg, err = enmime.NewParser().ReadEnvelope(buf)
		if err != nil {
			return nil, err
		}
		ctx.d.lock.Lock()
		defer ctx.d.lock.Unlock()
		// FIXME: Ensure cache works.
		//ctx.d.Cache.Put(key, msg, int64(buf.Len()))
	}
	return msg, nil
}

func parseEvalArgs(args []string) (g, r string, err error) {
	return
}
