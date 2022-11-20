package main

import (
	"bufio"
	"fmt"
	"github.com/gogama/reee-evolution/log"
	"github.com/gogama/reee-evolution/protocol"
	"io"
	"time"
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
	pc := protocol.Command{
		Type:  protocol.EvalCommandType,
		ID:    cmdID,
		Level: log.LevelOf(logger),
		Args:  cmd.Group,
	}
	if len(cmd.Rule) > 0 {
		pc.Args += " " + cmd.Rule
	}

	start := time.Now()
	err := protocol.WriteCommand(w, pc)
	if err != nil {
		return err
	}
	elapsed := time.Since(start)
	log.Verbose(logger, "wrote %s command for cmd %s in %s.", protocol.EvalCommandType, cmdID, elapsed)

	start = time.Now()
	n, err := io.Copy(w, ins)
	if err != nil {
		return err
	}
	elapsed = time.Since(start)
	log.Verbose(logger, "copied %d bytes from input to connection in %s.", n, elapsed)

	// TODO: Read back result.

	return nil
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
