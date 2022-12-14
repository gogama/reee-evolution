package rule

import (
	"time"

	"github.com/dop251/goja"
)

func marshalDate(vm *goja.Runtime, t time.Time) (goja.Value, error) {
	if t.IsZero() {
		return goja.Undefined(), nil
	}
	value, err := vm.New(vm.Get("Date").ToObject(vm), vm.ToValue(t.UnixNano()/1e6))
	if err != nil {
		return nil, err
	}
	return value, nil
}
