package daemon

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
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
	Groups   map[string][]Rule
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

	w := bufio.NewWriter(conn)

	ctx := newCmdContext(d, connID, r, w, cmd, isEOF)
	ctx.Verbose("daemon received %v", cmd)

	var data []byte
	switch cmd.Type {
	case protocol.ListCommandType:
		data, err = handleList(&ctx)
	case protocol.EvalCommandType:
		err = handleEval(&ctx)
	default:
		panic(fmt.Sprintf("daemon: unhandled command type: %d", cmd.Type))
	}

	if connErr, ok := err.(connError); ok {
		log.Normal(d.Logger, ctx.logPrefix+"error: "+connErr.Error())
		return
	} else if err != nil {
		log.Verbose(d.Logger, ctx.logPrefix+"error: "+err.Error())
		err = protocol.WriteError(w, err.Error())
		if err != nil {
			log.Normal(d.Logger, ctx.logPrefix+"error: "+err.Error())
		}
	} else {
		err = protocol.WriteSuccess(w, data)
		if err != nil {
			log.Normal(d.Logger, ctx.logPrefix+"error: "+err.Error())
		} else {
			log.Verbose(d.Logger, ctx.logPrefix+"success: %d bytes of result data written", len(data))
		}
	}
}

type connError struct {
	err error
}

func (err connError) Error() string {
	return err.err.Error()
}

func handleList(ctx *cmdContext) ([]byte, error) {
	if len(ctx.args) > 0 {
		return nil, fmt.Errorf("%s command not allowed arguments but had %q", protocol.ListCommandType, ctx.args)
	}

	var b bytes.Buffer
	var n int
	for group, rules := range ctx.d.Groups {
		_, _ = b.Write([]byte(group))
		for _, r := range rules {
			n++
			_ = b.WriteByte(' ')
			_, _ = b.WriteString(r.String())
		}
		b.WriteByte('\n')
	}

	ctx.Verbose("buffered %d groups and %d rules in %d bytes", len(ctx.d.Groups), n, b.Len())

	return b.Bytes(), nil
}

const evalErrPrefix = "args format must be <len> <group> [<rule>] but "

func handleEval(ctx *cmdContext) error {
	if ctx.args == "" {
		return errors.New(evalErrPrefix + "args is empty")
	}

	N, rem, found := strings.Cut(ctx.args, " ")
	n, err := strconv.Atoi(N)
	if err != nil || n < 0 {
		return fmt.Errorf(evalErrPrefix+"first element is %q", N)
	}

	if !found {
		return errors.New(evalErrPrefix + "args does not contain <group>")
	}
	g, r, found := strings.Cut(rem, " ")
	if !found {
		g = rem
	}

	var rules []Rule
	var ok bool

	if rules, ok = ctx.d.Groups[g]; !ok {
		return fmt.Errorf("group not found: %s", g)
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

	ctx.Verbose("reading %d bytes of input for group: %s, rules: %s...", n, g, rules)

	buf := make([]byte, n)
	var m int
	start := time.Now()
	for !ctx.isEOF && m < n {
		var o int
		o, err = ctx.r.Read(buf[m:])
		m += o
		if err == io.EOF {
			ctx.isEOF = true
		} else if err != nil {
			return err
		}
	}
	hash := fmt.Sprintf("%x", md5.Sum(buf))
	elapsed := time.Since(start)
	ctx.Verbose("read %d bytes of input with md5sum %s in %s.", m, hash, elapsed)

	if m < n {
		return fmt.Errorf("insufficient input: received only %d/%d expected bytes", m, n)
	}

	msg := getMsg(ctx, hash)
	if msg != nil {
		ctx.Verbose("retrieved message from cache with key %s.", hash)
	} else {
		start = time.Now()
		msg, err = parseMsg(ctx, buf)
		if err != nil {
			return fmt.Errorf("invalid message: %s", err.Error())
		}
		elapsed = time.Since(start)
		ctx.Verbose("parsed message in %s.", elapsed)
	}

	var stop bool
	for i := 0; i < len(rules) && !stop; i++ {
		stop, err = rules[i].Eval(ctx, ctx, msg)
		if err != nil {
			// TODO: handle rule evaluation error
		}
	}

	return nil
}

func getMsg(ctx *cmdContext, key string) *enmime.Envelope {
	ctx.d.lock.RLock()
	defer ctx.d.lock.RUnlock()
	return ctx.d.Cache.Get(key)
}

func parseMsg(ctx *cmdContext, b []byte) (*enmime.Envelope, error) {
	//key := fmt.Sprintf("%x", md5.Sum(b.Bytes()))
	// FIXME: Ensure cache works. Line below commented out because currently cache is always nil.
	// msg := getMsg(ctx, key)
	var msg *enmime.Envelope
	if msg == nil {
		var err error
		r := bytes.NewReader(b)
		msg, err = enmime.NewParser().ReadEnvelope(r)
		if err != nil {
			return nil, err
		}
		ctx.d.lock.Lock()
		defer ctx.d.lock.Unlock()
		// FIXME: Ensure cache works.
		//ctx.d.Cache.Put(key, msg, int64(b.Len()))
	}
	return msg, nil
}

func parseEvalArgs(args []string) (g, r string, err error) {
	return
}
