package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gogama/reee-evolution/cmd/reeeuse"
	"github.com/gogama/reee-evolution/daemon"
	"github.com/jhillyerd/enmime"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"github.com/gogama/reee-evolution/version"
)

type args struct {
	Address string `arg:"--addr,env:REEE_ADDR" help:"daemon address"`
	Network string `arg:"--net,env:REEE_NET" help:"daemon network"`
	Verbose bool   `arg:"-v,--verbose" help:"enable verbose logging"`
}

func (a *args) Version() string {
	return version.OfCmd()
}

type exitCoder interface {
	Code() int
}

func main() {
	// Configure the command line interface.
	network, address := protocol.DefaultNetAddr()
	a := args{
		Network: network,
		Address: address,
	}

	// Parse arguments. Exit early on parsing error, validation error,
	// or a help or version request.
	arg.MustParse(&a)

	// Run the daemon program.
	err := runDaemon(context.Background(), os.Stdin, os.Stderr, &a)
	if err != nil {
		msg := err.Error()
		if strings.HasSuffix(msg, "\n") {
			msg = msg[:len(msg)-1]
		}
		if msg != "" {
			_, _ = fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		}
		code := 1
		if coder, ok := err.(exitCoder); ok {
			code = coder.Code()
		}
		os.Exit(code)
	}
}

func runDaemon(parent context.Context, r io.Reader, w io.Writer, a *args) error {
	// Initialize logging.
	lvl := log.NormalLevel
	if a.Verbose {
		lvl = log.VerboseLevel
	}
	logger := log.WithWriter(lvl, w)

	// Get a context that ends when we get a terminating signal.
	signalCtx, stop := reeeuse.SignalContext(parent)

	// Load the rules.
	groups, err := loadRuleGroups(signalCtx, logger, a)

	// Create the history.
	// TODO. SQLite3 history.

	// Create the cache.
	// TODO. Cache.

	// Create the listener.
	listener, err := net.Listen(a.Network, a.Address)
	if err != nil {
		return err
	}
	err = signalCtx.Err()
	if err != nil {
		_ = listener.Close()
		return nil
	}
	log.Verbose(logger, "listening... (network: %s, address: %s)", a.Network, a.Address)

	// Start the daemon.
	d := daemon.Daemon{
		Listener: listener,
		Logger:   logger,
		Groups:   groups,
	}
	var fatalErr atomic.Value
	go func() {
		localErr := d.Serve()
		if err != daemon.ErrStopped {
			fatalErr.Store(localErr)
			stop()
		}
	}()

	// Wait for a terminating signal.
	<-signalCtx.Done()

	// If daemon's server had a fatal error, return that error.
	if val := fatalErr.Load(); val != nil {
		return val.(error)
	}

	// Allocate a short context for cleanup.
	log.Verbose(logger, "stopping...")
	cleanupCtx, cancel := context.WithTimeout(parent, 200*time.Millisecond)

	// Stop the daemon.
	err = d.Stop(cleanupCtx)
	cancel()
	if err == nil {
		log.Verbose(logger, "stopped.")
	}
	return err
}

func loadRuleGroups(ctx context.Context, logger log.Printer, a *args) (map[string][]daemon.Rule, error) {
	// TODO: Load some real rule groups

	return map[string][]daemon.Rule{
		"foo": {
			&tempDummyRule{
				name: "bar",
				f: func(ctx context.Context, logger log.Printer, msg *enmime.Envelope) (stop bool, err error) {
					log.Verbose(logger, "bar rule returns (false, nil)")
					return false, nil
				},
			},
			&tempDummyRule{
				name: "baz",
				f: func(ctx context.Context, logger log.Printer, msg *enmime.Envelope) (stop bool, err error) {
					log.Verbose(logger, "baz rule returns (true, nil)")
					return true, nil
				},
			},
			&tempDummyRule{
				name: "qux",
				f: func(ctx context.Context, logger log.Printer, msg *enmime.Envelope) (stop bool, err error) {
					panic("qux rule does not get called...")
				},
			},
		},
		"hello": {
			&tempDummyRule{
				name: "world",
				f: func(ctx context.Context, logger log.Printer, msg *enmime.Envelope) (stop bool, err error) {
					return false, errors.New("world rule fails with error")
				},
			},
		},
	}, nil
}

// todo: delete
type tempDummyRuleFunc func(ctx context.Context, logger log.Printer, msg *enmime.Envelope) (stop bool, err error)

// todo: delete
type tempDummyRule struct {
	name string
	f    tempDummyRuleFunc
}

// todo: delete
func (todoDelete *tempDummyRule) String() string {
	return todoDelete.name
}

// todo: delete
func (todoDelete *tempDummyRule) Eval(ctx context.Context, logger log.Printer, msg *enmime.Envelope) (stop bool, err error) {
	return todoDelete.f(ctx, logger, msg)
}
