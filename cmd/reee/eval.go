package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
)

type evalCommand struct {
	Group string `arg:"positional,required" help:"rule group to evaluate"`
	Rule  string `arg:"positional" help:"optional rule to evaluate within group"`
}

func (cmd *evalCommand) Validate() error {
	err := validateRuleOrGroupName("group", cmd.Group)
	if err != nil {
		return err
	}
	return validateRuleOrGroupName("rule", cmd.Rule)
}

func (cmd *evalCommand) Exec(cmdID string, logger log.Printer, ins io.Reader, _ io.Writer, r *bufio.Reader, w *bufio.Writer) error {
	log.Verbose(logger, "reading and buffering input...")
	var buf bytes.Buffer
	start := time.Now()
	n, err := io.Copy(&buf, ins)
	if err != nil {
		return err
	}
	b := buf.Bytes()
	N := strconv.Itoa(len(b))
	hash := md5.Sum(b)
	elapsed := time.Since(start)
	log.Verbose(logger, "read %s bytes of input with md5sum %x in %s.", N, hash, elapsed)

	var sb strings.Builder
	sb.Grow(len(cmd.Group) + 1 + len(cmd.Rule) + 1 + len(N))
	_, _ = sb.WriteString(N)
	_ = sb.WriteByte(' ')
	_, _ = sb.WriteString(cmd.Group)
	if len(cmd.Rule) > 0 {
		_ = sb.WriteByte(' ')
		_, _ = sb.WriteString(cmd.Rule)
	}
	pc := protocol.Command{
		Type:  protocol.EvalCommandType,
		ID:    cmdID,
		Level: log.LevelOf(logger),
		Args:  sb.String(),
	}

	log.Verbose(logger, "sending %s command...", protocol.EvalCommandType)
	start = time.Now()
	err = protocol.WriteCommand(w, pc)
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	log.Verbose(logger, "sent %s command for cmd %s in %s.", protocol.EvalCommandType, cmdID, elapsed)

	log.Verbose(logger, "sending data...")
	start = time.Now()
	_, err = w.Write(b)
	if err != nil {
		return err
	}
	err = w.Flush()
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	log.Verbose(logger, "sent %d bytes in %s.", n, elapsed)

	start = time.Now()
	rst, err := protocol.ReadResult(logger, r)
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	log.Verbose(logger, "read %s result and %d bytes of data in %s.", rst.Type, len(rst.Data), elapsed)

	switch rst.Type {
	case protocol.SuccessResultType:
		if len(rst.Data) > 0 {
			log.Verbose(logger, "received %d bytes of unexpected data in success result", len(rst.Data))
			return errors.New("unexpected data in success result")
		}
		return nil
	case protocol.ErrorResultType:
		return errors.New(string(rst.Data))
	default:
		panic(fmt.Sprintf("reee: unhandled result type: %d", rst.Type))
	}
}

// TODO: This should be specific to validating rule and group and should
// ideally be shared code that daemon can also use.
func validateRuleOrGroupName(category, name string) error {
	for i := range name {
		c := name[i]
		if 'a' <= c && c <= 'z' ||
			'A' <= c && c <= 'Z' ||
			'0' <= c && c <= '9' ||
			c == '_' || c == '-' {
			continue
		} else {
			return fmt.Errorf("%s contains invalid character '%c'. "+
				"valid characters are [a-zA-Z0-9_-]", category, c)
		}
	}
	return nil
}
