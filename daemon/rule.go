package daemon

import (
	"context"
	"fmt"
	"github.com/gogama/reee-evolution/log"
	"github.com/jhillyerd/enmime"
)

type Rule interface {
	fmt.Stringer
	Eval(ctx context.Context, logger log.Printer, msg *enmime.Envelope) (stop bool, err error)
}
