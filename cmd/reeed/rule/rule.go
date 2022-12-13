package rule

import (
	"context"
	"fmt"
	"time"

	"github.com/dop251/goja"
	"github.com/gogama/reee-evolution/daemon"
	"github.com/gogama/reee-evolution/log"
)

type ruleFunc func(msg goja.Value, logger goja.Value) (goja.Value, error)

type jsRule struct {
	parent *jsGroup
	cont   *vmContainer
	name   string
	f      ruleFunc
}

func (r *jsRule) String() string {
	return r.name
}

func (r *jsRule) Eval(ctx context.Context, logger log.Printer, msg *daemon.Message, tagger daemon.Tagger) (stop bool, err error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err = r.cont.acquire(timeoutCtx)
	if err != nil {
		return false, err
	}
	defer r.cont.release()

	m := marshalMessage(r.cont.vm, msg, tagger)
	l, err := marshalLogger(r.parent.name, r.name, r.cont, logger)
	if err != nil {
		return false, err
	}

	jsStop, err := r.f(m, l)
	if err != nil {
		return false, err
	}

	stop = jsStop.ToBoolean()

	// TODO: Delete temporary code here
	log.Verbose(logger, "RULE %s RETURNED %t.", r.name, stop)

	return
}

func unmarshalRule(vm *goja.Runtime, o *goja.Object, cont *vmContainer, i int, parent *jsGroup) (rule *jsRule, err error) {
	keys := o.Keys()
	var name string
	var f ruleFunc
	for _, key := range keys {
		switch key {
		case "name":
			name = o.Get("name").String()
			if name == "" {
				err = fmt.Errorf("reeed: blank rule name: rule %d in group %s", i, parent.name)
				return
			}
		case "rule":
			fv := o.Get("rule")
			err = vm.ExportTo(fv, &f)
			if err != nil {
				err = fmt.Errorf("reeed: can't unmarshal rule function: rule %d in group %s: %s", i, parent.name, err)
				return
			}
		}
		if name != "" && f != nil {
			break
		}
	}
	if name == "" {
		err = fmt.Errorf("reeed: can't determine rule name: rule %d in group %s", i, parent.name)
		return
	}
	// TODO: Validation on rule name here please.
	rule = &jsRule{
		parent: parent,
		cont:   cont,
		name:   name,
		f:      f,
	}
	return
}
