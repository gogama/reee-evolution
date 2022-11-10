package rule

import (
	"fmt"
	"github.com/jhillyerd/enmime"
)

type Rule interface {
	fmt.Stringer
	Eval(e *enmime.Envelope) error
}
