package rule

import (
	"github.com/dop251/goja"
	"github.com/gogama/reee-evolution/daemon"
	"github.com/jhillyerd/enmime"
)

type immutableMap interface {
	keys() []string
	get(key string) (string, bool)
}

type mutableMap interface {
	immutableMap
	set(key, value string)
	deleteKey(key string)
}

type jsLazyMap struct {
	im   immutableMap
	mm   mutableMap
	keys goja.Value
}

func marshalLazyMap(vm *goja.Runtime, protoPtr **goja.Object, im immutableMap, mm mutableMap) (goja.Value, error) {
	proto := *protoPtr
	var err error
	if proto == nil {
		proto, err = jsLazyMapPrototype(vm, mm != nil)
		if err != nil {
			return nil, err
		}
		*protoPtr = proto
	}
	lm := &jsLazyMap{
		im: im,
		mm: mm,
	}
	o := vm.ToValue(lm).ToObject(vm)
	err = o.SetPrototype(proto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func jsLazyMapPrototype(vm *goja.Runtime, mutable bool) (*goja.Object, error) {
	proto := vm.NewObject()
	err := defineGetterProperty(vm, proto, "keys", jsLazyMapKeys)
	if err != nil {
		return nil, err
	}
	err = proto.Set("get", vm.ToValue(jsLazyMapGet))
	if err != nil {
		return nil, err
	}
	if !mutable {
		return proto, nil
	}
	err = proto.Set("set", vm.ToValue(jsLazyMapSet))
	if err != nil {
		return nil, err
	}
	err = proto.Set("deleteKey", vm.ToValue(jsLazyMapDeleteKey))
	if err != nil {
		return nil, err
	}
	return proto, nil
}

func jsLazyMapKeys(vm *goja.Runtime, this any) (goja.Value, error) {
	if this, ok := this.(*jsLazyMap); ok {
		if this.keys == nil {
			keys := vm.ToValue(this.im.keys())
			if this.mm == nil {
				this.keys = keys
			}
			return keys, nil
		}
		return this.keys, nil
	}
	return nil, errUnexpectedThisType(&jsLazyMap{}, this)
}

func jsLazyMapGet(call goja.FunctionCall, vm *goja.Runtime) goja.Value {
	if len(call.Arguments) != 1 {
		throwJSException(vm, "reeed: get() requires exactly one argument")
	}
	key_ := call.Arguments[0].Export()
	var key string
	var ok bool
	if key, ok = key_.(string); !ok {
		throwJSException(vm, errUnexpectedArgType(0, "", key_))
	}
	this_ := call.This.Export()
	var this *jsLazyMap
	if this, ok = this_.(*jsLazyMap); !ok {
		throwJSException(vm, errUnexpectedThisType(&jsLazyMap{}, this_))
	} else if value, ok := this.im.get(key); ok {
		return vm.ToValue(value)
	}
	return goja.Undefined()
}

func jsLazyMapSet(call goja.FunctionCall, vm *goja.Runtime) goja.Value {
	if len(call.Arguments) != 2 {
		throwJSException(vm, "reeed: set() requires exactly two arguments")
	}
	key_ := call.Arguments[0].Export()
	var key string
	var ok bool
	if key, ok = key_.(string); !ok {
		throwJSException(vm, errUnexpectedArgType(0, "", key_))
	}
	value_ := call.Arguments[1].Export()
	var value string
	if value, ok = value_.(string); !ok {
		throwJSException(vm, errUnexpectedArgType(1, "", value_))
	}
	this_ := call.This.Export()
	var this *jsLazyMap
	if this, ok = this_.(*jsLazyMap); !ok {
		throwJSException(vm, errUnexpectedThisType(&jsLazyMap{}, this_))
	}
	this.mm.set(key, value)
	return call.Arguments[1]
}

func jsLazyMapDeleteKey(call goja.FunctionCall, vm *goja.Runtime) goja.Value {
	if len(call.Arguments) != 1 {
		throwJSException(vm, "reeed: deleteKey() requires exactly one argument")
	}
	key_ := call.Arguments[0].Export()
	var key string
	var ok bool
	if key, ok = key_.(string); !ok {
		throwJSException(vm, errUnexpectedArgType(0, "", key_))
	}
	this_ := call.This.Export()
	var this *jsLazyMap
	if this, ok = this_.(*jsLazyMap); !ok {
		throwJSException(vm, errUnexpectedThisType(&jsLazyMap{}, this_))
	}
	this.mm.deleteKey(key)
	return goja.Undefined()
}

type headersMap struct {
	*enmime.Envelope
}

func (hm headersMap) keys() []string {
	return hm.GetHeaderKeys()
}

func (hm headersMap) get(key string) (string, bool) {
	value := hm.GetHeader(key)
	return value, value != ""
}

type tagsMap struct {
	daemon.Tagger
}

func (tm tagsMap) keys() []string {
	return tm.Keys()
}

func (tm tagsMap) get(key string) (string, bool) {
	return tm.GetTag(key)
}

func (tm tagsMap) set(key, value string) {
	tm.SetTag(key, value)
}

func (tm tagsMap) deleteKey(key string) {
	tm.DeleteTag(key)
}
