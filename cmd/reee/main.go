package main

import (
	"bufio"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/gogama/reee-evolution/log"
	"io"
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
	Address string `arg:"--addr,env:REEE_ADDR" help:"daemon address"`
	Network string `arg:"--net,env:REEE_NET" help:"daemon network"`
	Verbose bool   `arg:"-v,--verbose" help:"enable verbose logging"`
}

func (a *args) Version() string {
	return version.OfCmd()
}

type subCommand interface {
	Validate() error
	Exec(cmdID string, logger log.Printer, ins io.Reader, outs io.Writer, r *bufio.Reader, w *bufio.Writer) error
}

type exitCoder interface {
	Code() int
}

func main() {
	// Configure command line interface.
	network, address := protocol.DefaultNetAddr()
	a := args{
		Network: network,
		Address: address,
	}

	// Parse arguments. Exit early on parsing error, validation error,
	// or a help or version request.
	p := arg.MustParse(&a)

	// Locate the sub-command to be executed.
	var sub subCommand
	var ok bool
	if sub, ok = p.Subcommand().(subCommand); !ok {
		p.WriteUsage(os.Stderr)
		fmt.Fprintln(os.Stderr, "error: command is required")
		os.Exit(1)
	}

	// Run the executeCommand.
	err := executeCommand(os.Stdin, os.Stdout, os.Stderr, sub, &a)
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

func executeCommand(ins io.Reader, outs, errs io.Writer, sub subCommand, a *args) error {
	// Validate that the command arguments are valid before proceeding.
	if err := sub.Validate(); err != nil {
		return err
	}

	// Initialize logging.
	lvl := log.NormalLevel
	if a.Verbose {
		lvl = log.VerboseLevel
	}
	logger := log.WithWriter(lvl, errs)

	// Connect to the daemon.
	conn, err := net.Dial(a.Network, a.Address)
	if err != nil {
		log.Verbose(logger, "error: %s", err)
		return fmt.Errorf("failed to connect to daemon (network %s, address %s)\n", a.Network, a.Address)
	}
	defer func() {
		closeErr := conn.Close()
		if closeErr != nil {
			log.Normal(logger, "error: failed to close connection: %s", closeErr)
		}
	}()

	// Allocate an ID for the command to execute.
	cmdID, err := uuid.NewV6()
	if err != nil {
		return err
	}
	log.Verbose(logger, "command ID: %s", cmdID)

	// TODO: We will want to put wire logging right here, maybe by
	//       wrapping the conn.

	// Buffer the connection.
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	// Execute the sub-command.
	return sub.Exec(cmdID.String(), logger, ins, outs, r, w)
}
