package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/gogama/reee-evolution/protocol"
	"github.com/gogama/reee-evolution/version"
)

type args struct {
	// Sub-commands.
	EvalCommand *evalCommand `arg:"subcommand:eval"`
	ListCommand *listCommand `arg:"subcommand:list"`

	// Global arguments.
	Verbose bool   `arg:"-v,--verbose" help:"enable verbose logging"`
	Network string `arg:"--net,env:REEE_NET" help:"daemon network"`
	Address string `arg:"--addr,env:REEE_ADDR" help:"daemon address"`
}

type subCommand interface {
	Exec(conn net.Conn) error
}

type exitCoder interface {
	Code() int
}

func main() {
	// Configure CLI.
	network, address := protocol.DefaultNetAddr()
	args := args{
		Network: network,
		Address: address,
	}

	// Parse arguments. Exit early on parsing error, validation error,
	// or a help or version request.
	p := arg.MustParse(&args)

	// Locate the sub-command to be executed.
	var sub subCommand
	var ok bool
	if sub, ok = p.Subcommand().(subCommand); !ok {
		p.WriteUsage(os.Stderr)
		fmt.Fprintln(os.Stderr, "error: command is required")
		os.Exit(1)
	}

	// TODO: Set up logger so we can do verbose error message logging.

	// Connect to the daemon.
	conn, err := net.Dial(args.Network, args.Address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error failed to connect to daemon (network %s, address %s)\n", args.Network, args.Address)
		// TODO: log verbose log of real error message here.
		os.Exit(1)
	}

	// Execute the sub-command.
	err = sub.Exec(conn)
	if err != nil {
		msg := err.Error()
		if strings.HasSuffix(msg, "\n") {
			msg = msg[:len(msg)-1]
		}
		if msg != "" {
			fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		}
		var coder exitCoder
		code := 1
		if coder, ok = err.(exitCoder); ok {
			code = coder.Code()
		}
		os.Exit(code)
	}
}

func (a *args) Version() string {
	return version.OfCmd()
}
