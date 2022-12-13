package rule

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/gogama/reee-evolution/daemon"
	"github.com/gogama/reee-evolution/log"
)

type GroupSet struct {
	groups map[string]*jsGroup
	vms    []*vmContainer
}

func (set *GroupSet) Load(ctx context.Context, logger log.Printer, path string) error {
	log.Verbose(logger, "loading groups from %s...", path)

	text, err := loadFileText(ctx, path)
	if err != nil {
		return nil
	}

	program, err := goja.Compile(path, text, true)
	if err != nil {
		return err
	}

	vm := goja.New()
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
	cont := &vmContainer{
		path: path,
		id:   len(set.vms),
		vm:   vm,
	}
	set.vms = append(set.vms, cont)
	hc := installAddRuleHook(set, cont)

	// TODO: Include context.
	_, err = vm.RunProgram(program)
	if err != nil {
		return err
	}

	log.Verbose(logger, "loaded %d groups and %d rules from %s.", len(hc.groups), hc.numRules, path)
	return nil
}

func (set *GroupSet) ToMap() map[string][]daemon.Rule {
	m := make(map[string][]daemon.Rule, len(set.groups))
	for _, g := range set.groups {
		rules := make([]daemon.Rule, len(g.rules))
		for i := range g.rules {
			rules[i] = g.rules[i]
		}
		m[g.name] = rules
	}
	return m
}

func loadFileText(ctx context.Context, path string) (string, error) {
	ch := make(chan struct{})
	var b []byte
	var err error
	go func() {
		b, err = os.ReadFile(path)
		close(ch)
	}()
	select {
	case <-ch:
		if err != nil {
			return "", err
		}
		return string(b), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

type jsHookContainer struct {
	hook     goja.Value
	groups   map[string]bool
	numRules int
}

func installAddRuleHook(set *GroupSet, cont *vmContainer) *jsHookContainer {
	hc := &jsHookContainer{
		groups: make(map[string]bool),
	}
	// Create the GoLang hook function.
	hookFunc := func(call goja.FunctionCall, vm *goja.Runtime) goja.Value {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(goja.Value); !ok {
					// TODO: Use logger to log panic value
					debug.PrintStack()
				}
				panic(r)
			}
		}()
		if len(call.Arguments) != 1 {
			throwJSException(vm, "reeed: addRules must receive at least 1 argument")
		}
		rm, err := unmarshalRuleMap(vm, call.Arguments[0])
		if err != nil {
			throwJSException(vm, err)
		}
		groups := make([]string, 0, len(rm))
		for g := range rm {
			// TODO: Validation on the group names please.
			groups = append(groups, g)
		}
		sort.Strings(groups)
		if set.groups == nil {
			set.groups = make(map[string]*jsGroup)
		}
		for _, g := range groups {
			hc.groups[g] = true
			var group *jsGroup
			if group = set.groups[g]; group == nil {
				group = &jsGroup{
					parent:      set,
					name:        g,
					rulesByName: make(map[string]*jsRule),
				}
				set.groups[g] = group
			}
			ruleCandidates := rm[g]
			for i, r := range ruleCandidates {
				var rule *jsRule
				rule, err = unmarshalRule(vm, r, cont, i, g)
				if err != nil {
					throwJSException(vm, err)
				}
				if group.rulesByName[rule.name] != nil {
					throwJSException(vm, fmt.Sprintf("duplicate rule name %s in group %s (%s)", group.name, rule.name, cont.path))
				}
				group.rulesByName[rule.name] = rule
				group.rules = append(group.rules, rule)
				hc.numRules++
			}
		}
		return goja.Undefined()
	}
	// Make the hook function available in the JavaScript runtime.
	reeeObject := cont.vm.NewObject()
	reeeObject.Set("addRules", cont.vm.ToValue(hookFunc))
	cont.vm.Set("reee", reeeObject)
	// Return the hook container.
	return hc
}

type vmContainer struct {
	path string
	id   int
	vm   *goja.Runtime
	mu   sync.Mutex
}

func (cont *vmContainer) acquire(ctx context.Context) error {
	// TODO
	return nil
}

func (cont *vmContainer) release() {
	// TODO
}

type jsGroup struct {
	parent      *GroupSet
	rules       []*jsRule
	rulesByName map[string]*jsRule
	name        string
}
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

	jsMsg := marshalMsg(r.cont.vm, msg, tagger)
	jsLogger := marshalLogger(r.cont.vm, logger)

	jsStop, err := r.f(jsMsg, jsLogger)
	if err != nil {
		return false, err
	}

	stop = jsStop.ToBoolean()

	// TODO: Delete temporary code here
	log.Verbose(logger, "RULE %s RETURNED %t.", r.name, stop)

	return
}

type ruleFunc func(msg goja.Value, logger goja.Value) (goja.Value, error)
type ruleMap map[string][]*goja.Object

func unmarshalRuleMap(runtime *goja.Runtime, v goja.Value) (rm ruleMap, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("reeed: can't convert argument to rule map: %s", r)
		}
	}()
	err = runtime.ExportTo(v, &rm)
	if err != nil {
		err = fmt.Errorf("reeed: can't convert argument to rule map: %s", err)
	}
	return
}

func unmarshalRule(vm *goja.Runtime, o *goja.Object, cont *vmContainer, i int, g string) (rule *jsRule, err error) {
	keys := o.Keys()
	var name string
	var f ruleFunc
	for _, key := range keys {
		switch key {
		case "name":
			name = o.Get("name").String()
			if name == "" {
				err = fmt.Errorf("reeed: blank rule name: rule %d in group %s", i, g)
				return
			}
		case "rule":
			fv := o.Get("rule")
			err = vm.ExportTo(fv, &f)
			if err != nil {
				err = fmt.Errorf("reeed: can't unmarshal rule function: rule %d in group %s: %s", i, g, err)
				return
			}
		}
		if name != "" && f != nil {
			break
		}
	}
	if name == "" {
		err = fmt.Errorf("reeed: can't determine rule name: rule %d in group %s", i, g)
		return
	}
	// TODO: Validation on rule name here please.
	rule = &jsRule{
		cont: cont,
		name: name,
		f:    f,
	}
	return
}

func marshalMsg(vm *goja.Runtime, msg *daemon.Message, tagger daemon.Tagger) goja.Value {
	// TODO:
	return goja.Undefined()
}

func marshalLogger(vm *goja.Runtime, logger log.Printer) goja.Value {
	// TODO:
	return goja.Undefined()
}
func throwJSException(vm *goja.Runtime, value any) {
	panic(vm.ToValue(value)) // goja converts the panic to a JS exception
}

/**
reeed.addRules({
	"foo": [
		{
			name: "bar",
			rule: function(msg, logger) {

			}
		}

	]
})
*/
