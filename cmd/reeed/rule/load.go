package rule

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"sync"

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
			throwJSException(vm, "reeed: addRules() must receive at least 1 argument")
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
				rule, err = unmarshalRule(vm, r, cont, i, group)
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
	path           string
	id             int
	vm             *goja.Runtime
	mu             sync.Mutex
	msgProto       *goja.Object
	loggerProto    *goja.Object
	addressesProto *goja.Object
	mailboxProto   *goja.Object
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

// TODO: Move this somewhere appropriate.
func assertGetter(call goja.FunctionCall, vm *goja.Runtime, name string) {
	if len(call.Arguments) != 0 {
		throwJSException(vm, fmt.Sprintf("reeed: %s may only be used as a property getter", name))
	}
}

// TODO: Move this somewhere appropriate.
func throwJSException(vm *goja.Runtime, value any) {
	panic(vm.ToValue(value)) // goja converts the panic to a JS exception
}

// TODO: Move this somewhere appropriate.
func errUnexpectedType(expected, actual any) error {
	return fmt.Errorf("reeed: expected this to be a %T, but it is a %T", expected, actual)
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
