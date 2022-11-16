package main

import (
	"fmt"
	"io"
	"os"
	"strings"

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

type exitCoder interface {
	Code() int
}

func main() {
	// Configure CLI.
	network, address := protocol.DefaultNetAddr()
	a := args{
		Network: network,
		Address: address,
	}

	// Parse arguments. Exit early on parsing error, validation error,
	// or a help or version request.
	arg.MustParse(&a)

	// Run the daemon program.
	err := daemon(os.Stderr, &a)
	if err != nil {
		msg := err.Error()
		if strings.HasSuffix(msg, "\n") {
			msg = msg[:len(msg)-1]
		}
		if msg != "" {
			fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		}
		code := 1
		if coder, ok := err.(exitCoder); ok {
			code = coder.Code()
		}
		os.Exit(code)
	}
}

func daemon(w io.Writer, a *args) error {
	// Initialize logging.
	lvl := log.NormalLevel
	if a.Verbose {
		lvl = log.VerboseLevel
	}
	logger := log.WithWriter(lvl, w)

	fmt.Fprintln(w, "foo", lvl, lvl.String())

	// TODO Add more beyond this point.
	log.Verbose(logger, "hello, world")
	return nil
}

func (a *args) Version() string {
	return version.OfCmd()
}
