package rule

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/gogama/reee-evolution/log"
)

func marshalLogger(group, rule string, cont *vmContainer, logger log.Printer) (goja.Value, error) {
	if cont.loggerProto == nil {
		proto, err := jsLoggerPrototype(cont.vm)
		if err != nil {
			return nil, err
		}
		cont.loggerProto = proto
	}
	l := &jsLogger{
		prefix: "[" + group + "." + rule + "] ",
		logger: logger,
	}
	o := cont.vm.ToValue(l).ToObject(cont.vm)
	err := o.SetPrototype(cont.loggerProto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func jsLoggerPrototype(vm *goja.Runtime) (*goja.Object, error) {
	proto := vm.NewObject()
	err := proto.Set("log", vm.ToValue(jsLogVerbose))
	if err != nil {
		return nil, err
	}
	return proto, nil
}

func jsLogVerbose(call goja.FunctionCall, vm *goja.Runtime) goja.Value {
	if len(call.Arguments) < 1 {
		throwJSException(vm, "reeed: log() must receive at least one argument")
	}
	this_ := call.This.Export()
	var this *jsLogger
	var ok bool
	if this, ok = this_.(*jsLogger); !ok {
		throwJSException(vm, errUnexpectedType(&jsLogger{}, this_))
	}
	if !this.hasLevel {
		this.isVerbose = log.LevelOf(this.logger) >= log.VerboseLevel
		this.hasLevel = true
	}
	format := this.prefix + call.Arguments[0].String()
	var v []any
	if len(call.Arguments) > 1 {
		v = make([]any, len(call.Arguments)-1)
		for i := range v {
			v[i] = call.Arguments[i+1].Export()
		}
	}
	if len(v) == 0 && strings.IndexByte(format, '%') < 0 {
		this.logger.Print(log.VerboseLevel, format)
	} else {
		this.logger.Print(log.VerboseLevel, fmt.Sprintf(format, v...))
	}
	return goja.Undefined()
}

type jsLogger struct {
	prefix    string
	logger    log.Printer
	hasLevel  bool
	isVerbose bool
}
