package daemon

import (
	"context"
	"fmt"

	"github.com/gogama/reee-evolution/log"
)

type Tagger interface {
	GetTag(key string) (value string, hit bool)
	SetTag(key, value string)
	DeleteTag(key string)
}

type Rule interface {
	fmt.Stringer
	Eval(ctx context.Context, logger log.Printer, msg *Message, tagger Tagger) (stop bool, err error)
}
