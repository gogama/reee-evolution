package rule

import (
	"fmt"
	"net/mail"
	"reflect"
	"time"
	"unsafe"

	"github.com/dop251/goja"
	"github.com/gogama/reee-evolution/daemon"
)

func marshalMessage(cont *vmContainer, msg *daemon.Message, tagger daemon.Tagger) (goja.Value, error) {
	if cont.msgProto == nil {
		proto, err := jsMessagePrototype(cont)
		if err != nil {
			return nil, err
		}
		cont.msgProto = proto
	}
	m := &jsMessage{
		msg:    msg,
		tagger: tagger,
	}
	o := cont.vm.ToValue(m).ToObject(cont.vm)
	err := o.SetPrototype(cont.msgProto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

var jsMessageMailboxHeaderProps = []struct {
	propName   string
	headerName string
}{
	{"from", "From"},
	{"sender", "Sender"},
	{"replyTo", "Reply-To"},
	{"to", "To"},
	{"cc", "Cc"},
	{"bcc", "Bcc"},
}

var jsMessageStringHeaderProps = []struct {
	propName   string
	headerName string
}{
	{"id", "Message-Id"},
	{"subject", "Subject"},
}

func jsMessagePrototype(cont *vmContainer) (*goja.Object, error) {
	proto := cont.vm.NewObject()
	// Define properties relating to email headers that contain mailboxes.
	for _, prop := range jsMessageMailboxHeaderProps {
		err := jsMessagePrototypeDefineAddressesProp(cont, proto, prop.propName, prop.headerName)
		if err != nil {
			return nil, err
		}
	}
	// Define properties relating to email headers that contain string
	// values.
	for _, prop := range jsMessageStringHeaderProps {
		err := jsMessagePrototypeDefineProp(cont.vm, proto, prop.propName, func(msg *daemon.Message) string {
			return msg.Envelope.GetHeader(prop.headerName)
		}, func(value string) (goja.Value, error) {
			return cont.vm.ToValue(value), nil
		})
		if err != nil {
			return nil, err
		}
	}
	// Define properties relating to the standard parts.
	err := jsMessagePrototypeDefineProp(cont.vm, proto, "text", func(msg *daemon.Message) string {
		return msg.Envelope.Text
	}, func(value string) (goja.Value, error) {
		return cont.vm.ToValue(value), nil
	})
	if err != nil {
		return nil, err
	}
	err = jsMessagePrototypeDefineProp(cont.vm, proto, "html", func(msg *daemon.Message) string {
		return msg.Envelope.HTML
	}, func(value string) (goja.Value, error) {
		return cont.vm.ToValue(value), nil
	})
	if err != nil {
		return nil, err
	}
	// Define date/time properties.
	err = jsMessagePrototypeDefineProp(cont.vm, proto, "date", func(msg *daemon.Message) time.Time {
		t, err := mail.ParseDate(msg.Envelope.GetHeader("Date"))
		if err != nil {
			return time.Time{}
		}
		return t
	}, func(t time.Time) (goja.Value, error) {
		return marshalDate(cont.vm, t)
	})

	// Message prototype is ready.

	return proto, nil
}

type jsMessage struct {
	msg    *daemon.Message
	tagger daemon.Tagger

	// Cached address fields.
	from    goja.Value
	sender  goja.Value
	replyTo goja.Value
	to      goja.Value
	cc      goja.Value
	bcc     goja.Value

	// Cached string fields.
	id      goja.Value
	subject goja.Value
	text    goja.Value
	html    goja.Value

	// Cached time.Time fields.
	date goja.Value

	// Cached lazy headers.
	headers goja.Value

	// Cached materialized array of attachments.
	attachments goja.Value

	tags goja.Value
}

var jsMessageType = reflect.TypeOf(jsMessage{})

func jsMessagePrototypeDefineAddressesProp(cont *vmContainer, proto *goja.Object, propName, headerName string) error {
	return jsMessagePrototypeDefineProp(cont.vm, proto, propName, func(msg *daemon.Message) string {
		// TODO: Switch to using msg.Envelope.AddressList(...), it is more tolerant of errors.
		return msg.Envelope.GetHeader(headerName)
	}, func(addresses string) (goja.Value, error) {
		return marshalAddresses(cont, addresses)
	})
}

func jsMessagePrototypeDefineProp[T any](vm *goja.Runtime, proto *goja.Object, propName string, get func(*daemon.Message) T, convert func(T) (goja.Value, error)) error {
	// Determine the offset within the structure of the propField which
	// contains the cached value.
	var propField reflect.StructField
	var ok bool
	if propField, ok = jsMessageType.FieldByName(propName); !ok {
		panic(fmt.Sprintf("property field %q not found in %v", propName, jsMessageType))
	}

	// Add a property accessor to the prototype which checks for the
	// cached property at the given offset within the message, and if
	// it is not yet cached, fetches it from the underlying daemon
	// message.
	return defineGetterProperty(vm, proto, propName, func(this any) (goja.Value, error) {
		if this, ok := this.(*jsMessage); ok {
			propPtr := (*goja.Value)(unsafe.Add(unsafe.Pointer(this), propField.Offset))
			if *propPtr != nil {
				return *propPtr, nil
			}
			rawValue := get(this.msg)
			propValue, err := convert(rawValue)
			if err != nil {
				return nil, err
			}
			*propPtr = propValue
			return propValue, nil
		}
		return nil, errUnexpectedType(&jsMessage{}, this)
	})
}

func marshalAddresses(cont *vmContainer, addresses string) (goja.Value, error) {
	if cont.addressesProto == nil {
		proto, err := jsAddressesPrototype(cont.vm)
		if err != nil {
			return nil, err
		}
		cont.addressesProto = proto
	}
	list, err := mail.ParseAddressList(addresses)
	if err != nil {
		// TODO: Find a way to log this and just continue
		fmt.Println("ERROR", err, "TODO: fins a way to log this and continue")
		list = nil
	}

	a := make([]goja.Value, len(list))
	for i := range list {
		a[i], err = marshalMailbox(cont, list[i])
		if err != nil {
			return nil, err
		}
	}
	as := &jsAddresses{
		header:    cont.vm.ToValue(addresses),
		mailboxes: cont.vm.ToValue(a),
	}
	o := cont.vm.ToValue(as).ToObject(cont.vm)
	err = o.SetPrototype(cont.addressesProto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func jsAddressesPrototype(vm *goja.Runtime) (*goja.Object, error) {
	proto := vm.NewObject()
	err := defineGetterProperty(vm, proto, "header", func(this any) (goja.Value, error) {
		if this, ok := this.(*jsAddresses); ok {
			return this.header, nil
		}
		return nil, errUnexpectedType(&jsAddresses{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "mailboxes", func(this any) (goja.Value, error) {
		if this, ok := this.(*jsAddresses); ok {
			return this.mailboxes, nil
		}
		return nil, errUnexpectedType(&jsAddresses{}, this)
	})
	if err != nil {
		return nil, err
	}
	return proto, nil
}

type jsAddresses struct {
	header    goja.Value
	mailboxes goja.Value
}

func marshalMailbox(cont *vmContainer, mailbox *mail.Address) (goja.Value, error) {
	if cont.mailboxProto == nil {
		proto, err := jsMailboxPrototype(cont.vm)
		if err != nil {
			return nil, err
		}
		cont.mailboxProto = proto
	}
	m := &jsMailbox{
		name:    cont.vm.ToValue(mailbox.Name),
		address: cont.vm.ToValue(mailbox.Address),
	}
	o := cont.vm.ToValue(m).ToObject(cont.vm)
	err := o.SetPrototype(cont.mailboxProto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func jsMailboxPrototype(vm *goja.Runtime) (*goja.Object, error) {
	proto := vm.NewObject()
	err := defineGetterProperty(vm, proto, "name", func(this any) (goja.Value, error) {
		if this, ok := this.(*jsMailbox); ok {
			return this.name, nil
		}
		return nil, errUnexpectedType(&jsMailbox{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "address", func(this any) (goja.Value, error) {
		if this, ok := this.(*jsMailbox); ok {
			return this.address, nil
		}
		return nil, errUnexpectedType(&jsMailbox{}, this)
	})
	if err != nil {
		return nil, err
	}
	return proto, nil
}

// TODO: Move this elsewhere.
func defineGetterProperty(vm *goja.Runtime, proto *goja.Object, propName string, getter func(any) (goja.Value, error)) error {
	return proto.DefineAccessorProperty(propName, vm.ToValue(func(call goja.FunctionCall, vm2 *goja.Runtime) goja.Value {
		assertGetter(call, vm, propName)
		value, err2 := getter(call.This.Export())
		if err2 != nil {
			throwJSException(vm, err2)
		}
		return value
	}), goja.Undefined(), goja.FLAG_FALSE, goja.FLAG_FALSE)
}

type jsMailbox struct {
	name    goja.Value // string
	address goja.Value // string
}

/**
  How should headers behave? SHOULD BE LAZY.
	msg.headers.length -> Number of headers.
	msg.headers.keys -> Array of header keys.
	msg.headers.get("foo") -> header value.

  How should attachments behave? SHOULD BE MATERIALIZED ARRAY.
	msg.attachments -> array
	msg.attachments.length (array length)
	msg.attachments[0].fileName
	msg.attachments[0].fileModDate
	msg.attachments[0].contentType

		[Forget about trying to handle actual content bytes/length.]

  How should tags behave? SIMILAR TO HEADERS.
    msg.tags.length -> Number of tags.
    msg.tags.keys -> Array of tag keys.
	msg.tags.get("foo") -> tag value
	msg.tags.deleteKey("foo")

  How should full content bytes behave?
    Forget about it. Too complex, unlikely to be useful.
*/
