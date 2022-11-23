package daemon

import (
	"context"
	"fmt"

	"github.com/gogama/reee-evolution/log"
)

type Rule interface {
	fmt.Stringer
	Eval(ctx context.Context, logger log.Printer, msg *Message) (stop bool, err error)
}
