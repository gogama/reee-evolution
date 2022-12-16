package daemon

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"math/rand"
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

type Daemon struct {
	Listener  net.Listener
	Logger    log.Printer
	Groups    map[string][]Rule
	Cache     MessageCache
	Store     MessageStore
	SampleSrc rand.Source
	SamplePct float64

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
	defer ctx.cancel()
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
	var deferredErr error

	if rules, ok = ctx.d.Groups[g]; !ok {
		deferredErr = fmt.Errorf("group not found: %s", g)
	}

	if deferredErr != nil && r != "" {
		for i := range rules {
			if r == rules[i].String() {
				rules = []Rule{rules[i]}
				r = ""
				break
			}
		}
		if r == "" {
			deferredErr = fmt.Errorf("rule not found: %s [group: %s]", r, g)
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
		} else if deferredErr == nil {
			deferredErr = err
		}
	}
	md5Sum := fmt.Sprintf("%x", md5.Sum(buf))
	elapsed := time.Since(start)
	ctx.Verbose("read %d bytes of input with md5sum %s in %s.", m, md5Sum, elapsed)

	if deferredErr != nil {
		return deferredErr
	} else if m < n {
		return fmt.Errorf("insufficient input: received only %d/%d expected bytes", m, n)
	}

	msg := getCachedMsg(ctx, md5Sum)
	var storeID string
	if msg == nil {
		// Parse the MIME envelope.
		start = time.Now()
		var e *enmime.Envelope
		e, err = enmime.NewParser().ReadEnvelope(bytes.NewReader(buf))
		if err != nil {
			return fmt.Errorf("invalid message: %s", err.Error())
		}
		storeID = toStoreID(e, md5Sum)
		elapsed = time.Since(start)
		ctx.Verbose("parsed MIME envelope for %s in %s.", storeID, elapsed)

		// Try to retrieve metadata from the store.
		msg, err = prepareMsg(ctx, md5Sum, storeID, e, buf)
		if err != nil {
			return err
		}
	} else {
		storeID = toStoreID(msg.Envelope, md5Sum)
	}

	ger := &EvalRecord{
		Message:   msg,
		startTime: time.Now(),
		group:     g,
		rules:     make([]*RuleEvalRecord, 0, len(rules)),
	}

	var stop bool
	var ruleEvalErr error
	start = time.Now()
	for i := 0; i < len(rules) && !stop; i++ {
		rer := &RuleEvalRecord{
			evalRecord: ger,
			startTime:  time.Now(),
			rule:       rules[i].String(),
		}
		stop, ruleEvalErr = rules[i].Eval(ctx.ctx, ctx, msg, rer)
		rer.endTime = time.Now()
		rer.stop = stop
		rer.err = ruleEvalErr
		ger.rules = append(ger.rules, rer)
		if ruleEvalErr != nil {
			ctx.Verbose("rule %s ended early with error: %s", rules[i], ruleEvalErr)
			break
		}
	}
	ger.endTime = time.Now()
	elapsed = time.Since(start)
	ctx.Verbose("evaluated %d rules in %s.", len(ger.rules), elapsed)

	start = time.Now()
	err = ctx.d.Store.RecordEval(storeID, ger)
	if ruleEvalErr != nil {
		return ruleEvalErr
	} else if err != nil {
		return err
	}
	elapsed = time.Since(start)
	ctx.Verbose("recorded evaluation record in %s.", elapsed)
	return nil
}

func getCachedMsg(ctx *cmdContext, cacheKey string) *Message {
	ctx.d.lock.RLock()
	defer ctx.d.lock.RUnlock()
	return getCachedMsgLocked(ctx, cacheKey)
}

func getCachedMsgLocked(ctx *cmdContext, cacheKey string) *Message {
	msg := ctx.d.Cache.Get(cacheKey)
	if msg != nil {
		ctx.Verbose("retrieved message from cache with cacheKey %s.", cacheKey)
	}
	return msg
}

func prepareMsg(ctx *cmdContext, cacheKey, storeID string, e *enmime.Envelope, buf []byte) (*Message, error) {
	ctx.d.lock.Lock()
	defer ctx.d.lock.Unlock()

	// Try to fetch from cache in case another request cached it since
	// we checked.
	msg := getCachedMsgLocked(ctx, cacheKey)
	if msg != nil {
		return msg, nil
	}

	// Try to get the metadata from the persistent message store.
	start := time.Now()
	meta, ok, err := ctx.d.Store.GetMetadata(storeID)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start)
	if ok {
		ctx.Verbose("found metadata for %s in message store in %s.", storeID, elapsed)
		msg = &Message{Envelope: e, fullText: buf, metadata: meta}
		ctx.d.Cache.Put(cacheKey, msg, uint64(len(buf)))
		return msg, nil
	}
	ctx.Verbose("did not find metadata for %s in message store in %s.", storeID, elapsed)

	// At this point we know the message isn't in the store. Make a
	// sampling decision.
	max := int64(float64(1<<63) * ctx.d.SamplePct)
	s := ctx.d.SampleSrc.Int63()
	sampled := s <= max
	if sampled {
		ctx.Verbose("sampled %s.", storeID)
	} else {
		ctx.Verbose("did not sample %s at %f%%. (value %d > max %d)", storeID, ctx.d.SamplePct*100.0, s, max)
	}

	// Construct the message.
	msg = &Message{
		Envelope: e,
		fullText: buf,
		metadata: Metadata{
			sampled: sampled,
		},
	}

	// Write the message back to the store.
	start = time.Now()
	err = ctx.d.Store.PutMessage(storeID, msg)
	if err != nil {
		return nil, err
	}
	elapsed = time.Since(start)
	ctx.Verbose("put %s into store in %s.", storeID, elapsed)

	// Put the message into cache.
	ctx.d.Cache.Put(cacheKey, msg, uint64(len(buf)))

	// Message is ready.
	return msg, nil
}
