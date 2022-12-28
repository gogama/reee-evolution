package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/gogama/reee-evolution/cmd/reeed/cache"
	"github.com/gogama/reee-evolution/cmd/reeed/rule"
	"github.com/gogama/reee-evolution/cmd/reeed/store"
	"github.com/gogama/reee-evolution/cmd/reeeuse"
	"github.com/gogama/reee-evolution/daemon"
	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"github.com/gogama/reee-evolution/version"
)

type args struct {
	Address   string  `arg:"-a,--addr,env:REEE_ADDR" help:"listen on address"`
	Network   string  `arg:"-n,--net,env:REEE_NET" help:"listen on network"`
	DBFile    string  `arg:"--db,env:REEE_DB" help:"path to email events database" placeholder:"FILE"`
	NoDB      bool    `arg:"--no-db" help:"don't log events to database"`
	RulePath  string  `arg:"--rules,env:REEE_RULES" help:"path to rule script directory" placeholder:"DIR"`
	SamplePct percent `arg:"-s,--sample" help:"sample percentage, e.g. 25%" default:"1%"`
	RandSeed  *int64  `arg:"-S,--seed" help:"seed for Math.random() number generator"`
	Quiet     bool    `arg:"-q,--quiet" help:"log only high-importance messages"`
	Verbose   bool    `arg:"-v,--verbose" help:"log all available messages"`
}

func (a *args) Version() string {
	return version.OfCmd()
}

type exitCoder interface {
	Code() int
}

func main() {
	// Capture earliest known start time.
	start := time.Now()

	// Configure the command line interface.
	network, address := protocol.DefaultNetAddr()
	a := args{
		Network: network,
		Address: address,
	}
	if dir, err := reeeuse.Dir(os.UserCacheDir); err == nil {
		a.DBFile = reeeuse.File(dir, ".sqlite3")
	}
	if dir, err := reeeuse.Dir(os.UserConfigDir); err == nil {
		a.RulePath = dir
	}

	// Parse arguments. Exit early on parsing error, validation error,
	// or a help or version request.
	arg.MustParse(&a)

	// Run the daemon program.
	err := runDaemon(context.Background(), os.Stderr, start, &a)
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

func runDaemon(parent context.Context, w io.Writer, start time.Time, a *args) error {
	// Initialize logging.
	lvl := log.NormalLevel
	if a.Verbose && a.Quiet {
		return errors.New("cannot be both quiet and verbose")
	} else if a.Verbose {
		lvl = log.VerboseLevel
	} else if a.Quiet {
		lvl = log.TaciturnLevel
	}
	logger := log.WithWriter(lvl, w)

	// Get a context that ends when we get a terminating signal.
	signalCtx, stop := reeeuse.SignalContext(parent)

	// Clean up any old domain socket.
	cleanupSocket(logger, a.Network, a.Address)

	// Load the rules.
	groups, err := loadRuleGroups(signalCtx, logger, a)
	if err != nil {
		return err
	}

	// Create or open the SQLite3-based persistent store.
	var s daemon.MessageStore
	if a.NoDB {
		s = &store.NullStore{}
	} else {
		log.Normal(logger, "opening message store... [path: %s]", a.DBFile)
		if s, err = store.NewSQLite3(signalCtx, a.DBFile); err != nil {
			return err
		}
		if closer, ok := s.(io.Closer); ok {
			defer func() {
				_ = closer.Close()
			}()
		}
	}

	// Create the cache.
	c := cache.New(cache.Policy{
		MaxCount: 100,
		MaxSize:  25 * 1024 * 1024,
		MaxAge:   20 * time.Minute,
	})

	// TODO: Apply some reasonable timeouts on the sockets.

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
	log.Normal(logger, "listening...             [network: %s, address: %s]", a.Network, a.Address)

	// Start the daemon.
	d := daemon.Daemon{
		Listener:  listener,
		Logger:    logger,
		Groups:    groups,
		Cache:     c,
		Store:     s,
		SampleSrc: rand.NewSource(time.Now().UnixMilli()),
		SamplePct: float64(a.SamplePct),
	}
	var fatalErr atomic.Value
	go func() {
		localErr := d.Serve()
		if err != daemon.ErrStopped {
			fatalErr.Store(localErr)
			stop()
		}
	}()

	// Indicate successful startup.
	elapsed := time.Since(start)
	log.Normal(logger, "started.                 [%s]", elapsed)
	// Wait for a terminating signal.
	<-signalCtx.Done()

	// If daemon's server had a fatal error, return that error.
	if val := fatalErr.Load(); val != nil {
		return val.(error)
	}

	// Allocate a short context for cleanup.
	log.Normal(logger, "stopping...")
	cleanupCtx, cancel := context.WithTimeout(parent, 200*time.Millisecond)

	// Stop the daemon.
	err = d.Stop(cleanupCtx)
	cancel()
	if err == nil {
		log.Normal(logger, "stopped.")
	}

	// TODO: Clean up the Unix domain socket file if it exists.

	return err
}

func loadRuleGroups(ctx context.Context, logger log.Printer, a *args) (map[string][]daemon.Rule, error) {
	var seedLog string
	if a.RandSeed == nil {
		seedLog = "<file load time>"
	} else if *a.RandSeed == 0 {
		seedLog = "<default>"
	} else {
		seedLog = strconv.FormatInt(*a.RandSeed, 10)
	}
	log.Normal(logger, "loading rules...         [path: %s, seed: %s]", a.RulePath, seedLog)

	if info, err := os.Lstat(a.RulePath); err != nil {
		// TODO: Log error and fail out.
	} else if !info.IsDir() {
		// TODO: Log warning and fail out.
	}

	var groups rule.GroupSet

	// Find all the JavaScript files and load them.
	err := filepath.WalkDir(a.RulePath, func(path string, d os.DirEntry, err error) error {
		if d.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}
		var randSeed int64
		if a.RandSeed == nil {
			randSeed = time.Now().Unix()
		} else {
			randSeed = *a.RandSeed
		}
		return groups.Load(ctx, logger, path, randSeed)
	})
	if err != nil {
		return nil, err
	}

	return groups.ToMap(), nil
}

type percent float64

func percentError(b []byte) error {
	return fmt.Errorf("invalid percentage: %q. must be a number in [0..100] plus optional %% symbol", b)
}

func (pct *percent) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		return percentError(b)
	}

	// Validate.
	var i int
	var dot bool
	for ; i < len(b); i++ {
		if b[i] == '.' {
			i++
			break
		} else if b[i] == '%' && i == len(b)-1 {
			b = b[:len(b)-1]
			break
		} else if !('0' <= b[i] && b[i] <= '9') {
			return percentError(b)
		}
	}
	for ; dot && i < len(b); i++ {
		if b[i] == '%' && i == len(b)-1 {
			b = b[:len(b)-1]
			break
		} else if !('0' <= b[i] && b[i] <= '9') {
			return percentError(b)
		}
	}

	// Convert.
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil && !(0.0 <= f && f <= 100.0) {
		return percentError(b)
	}

	// Finished.
	*pct = percent(f / 100.0)
	return nil
}
