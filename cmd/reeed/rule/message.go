package rule

import (
	"fmt"
	"net/mail"
	"reflect"
	"strings"
	"time"
	"unsafe"

	ics "github.com/arran4/golang-ical"
	"github.com/dop251/goja"
	"github.com/gogama/reee-evolution/daemon"
	"github.com/jhillyerd/enmime"
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
		err := jsMessagePrototypeDefineProp(cont.vm, proto, prop.propName, func(msg *jsMessage) string {
			return msg.msg.Envelope.GetHeader(prop.headerName)
		}, func(value string) (goja.Value, error) {
			return cont.vm.ToValue(value), nil
		})
		if err != nil {
			return nil, err
		}
	}
	// Define properties relating to the standard parts.
	err := jsMessagePrototypeDefineProp(cont.vm, proto, "text", func(msg *jsMessage) string {
		return msg.msg.Envelope.Text
	}, func(value string) (goja.Value, error) {
		return cont.vm.ToValue(value), nil
	})
	if err != nil {
		return nil, err
	}
	err = jsMessagePrototypeDefineProp(cont.vm, proto, "html", func(msg *jsMessage) string {
		return msg.msg.Envelope.HTML
	}, func(value string) (goja.Value, error) {
		return cont.vm.ToValue(value), nil
	})
	if err != nil {
		return nil, err
	}
	// Define date/time properties.
	err = jsMessagePrototypeDefineProp(cont.vm, proto, "date", func(msg *jsMessage) time.Time {
		t, err := mail.ParseDate(msg.msg.Envelope.GetHeader("Date"))
		if err != nil {
			return time.Time{}
		}
		return t
	}, func(t time.Time) (goja.Value, error) {
		return marshalDate(cont.vm, t)
	})
	if err != nil {
		return nil, err
	}
	// Define lazy map map properties.
	err = jsMessagePrototypeDefineProp(cont.vm, proto, "headers", func(msg *jsMessage) headersMap {
		return headersMap{Envelope: msg.msg.Envelope}
	}, func(hm headersMap) (goja.Value, error) {
		return marshalLazyMap(cont.vm, &cont.headersProto, hm, hm, nil)
	})
	if err != nil {
		return nil, err
	}
	err = jsMessagePrototypeDefineProp(cont.vm, proto, "tags", func(msg *jsMessage) mutableMap {
		return tagsMap{Tagger: msg.tagger}
	}, func(mm mutableMap) (goja.Value, error) {
		return marshalLazyMap(cont.vm, &cont.tagsProto, mm, nil, mm)
	})
	if err != nil {
		return nil, err
	}
	// Define attachments property.
	err = jsMessagePrototypeDefineProp(cont.vm, proto, "attachments", func(msg *jsMessage) []*enmime.Part {
		return msg.msg.Envelope.Attachments
	}, func(attachments []*enmime.Part) (goja.Value, error) {
		return marshalAttachments(cont.vm, &cont.attachmentProto, attachments)
	})
	if err != nil {
		return nil, err
	}
	// Define the calendar property.
	err = jsMessagePrototypeDefineProp(cont.vm, proto, "calendar", func(msg *jsMessage) *ics.Calendar {
		return parseCalendar(msg.msg.Envelope.OtherParts, msg.msg.Envelope.Inlines)
	}, func(calendar *ics.Calendar) (goja.Value, error) {
		if calendar == nil {
			return goja.Null(), nil
		}
		return marshalCalendar(cont, calendar)
	})

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

	// Cached lazy maps.
	headers goja.Value
	tags    goja.Value

	// Cached materialized array of attachments.
	attachments goja.Value

	// Cached materialized view of iCalendar part, if available.
	calendar goja.Value
}

var jsMessageType = reflect.TypeOf(jsMessage{})

func jsMessagePrototypeDefineAddressesProp(cont *vmContainer, proto *goja.Object, propName, headerName string) error {
	return jsMessagePrototypeDefineProp(cont.vm, proto, propName, func(msg *jsMessage) string {
		// TODO: Switch to using msg.Envelope.AddressList(...), it is more tolerant of errors.
		return msg.msg.Envelope.GetHeader(headerName)
	}, func(addresses string) (goja.Value, error) {
		return marshalAddresses(cont, addresses)
	})
}

func jsMessagePrototypeDefineProp[T any](vm *goja.Runtime, proto *goja.Object, propName string, get func(*jsMessage) T, convert func(T) (goja.Value, error)) error {
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
	return defineGetterProperty(vm, proto, propName, func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsMessage); ok {
			propPtr := (*goja.Value)(unsafe.Add(unsafe.Pointer(this), propField.Offset))
			if *propPtr != nil {
				return *propPtr, nil
			}
			rawValue := get(this)
			propValue, err := convert(rawValue)
			if err != nil {
				return nil, err
			}
			*propPtr = propValue
			return propValue, nil
		}
		return nil, errUnexpectedThisType(&jsMessage{}, this)
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
	if addresses == "" {
		return goja.Null(), nil
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
	err := defineGetterProperty(vm, proto, "header", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsAddresses); ok {
			return this.header, nil
		}
		return nil, errUnexpectedThisType(&jsAddresses{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "mailboxes", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsAddresses); ok {
			return this.mailboxes, nil
		}
		return nil, errUnexpectedThisType(&jsAddresses{}, this)
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
	name := goja.Undefined()
	if mailbox.Name != "" {
		name = cont.vm.ToValue(mailbox.Name)
	}
	address, localPart, domain := goja.Undefined(), goja.Undefined(), goja.Undefined()
	if mailbox.Address != "" {
		address = cont.vm.ToValue(mailbox.Address)
		if i := strings.IndexByte(mailbox.Address, '@'); i >= 0 {
			localPart = cont.vm.ToValue(mailbox.Address[0:i])
			domain = cont.vm.ToValue(mailbox.Address[i+1:])
		}
	}
	m := &jsMailbox{
		name:      name,
		address:   address,
		localPart: localPart,
		domain:    domain,
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
	err := defineGetterProperty(vm, proto, "name", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsMailbox); ok {
			return this.name, nil
		}
		return nil, errUnexpectedThisType(&jsMailbox{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "address", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsMailbox); ok {
			return this.address, nil
		}
		return nil, errUnexpectedThisType(&jsMailbox{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "localPart", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsMailbox); ok {
			return this.localPart, nil
		}
		return nil, errUnexpectedThisType(&jsMailbox{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "domain", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsMailbox); ok {
			return this.domain, nil
		}
		return nil, errUnexpectedThisType(&jsMailbox{}, this)
	})
	if err != nil {
		return nil, err
	}
	return proto, nil
}

// TODO: Move this elsewhere.
func defineGetterProperty(vm *goja.Runtime, proto *goja.Object, propName string, getter func(*goja.Runtime, any) (goja.Value, error)) error {
	return proto.DefineAccessorProperty(propName, vm.ToValue(func(call goja.FunctionCall, vm2 *goja.Runtime) goja.Value {
		assertGetter(call, vm, propName)
		value, err2 := getter(vm, call.This.Export())
		if err2 != nil {
			throwJSException(vm, err2)
		}
		return value
	}), goja.Undefined(), goja.FLAG_FALSE, goja.FLAG_FALSE)
}

type jsMailbox struct {
	name      goja.Value // string
	address   goja.Value // string
	localPart goja.Value // string (local-part of address)
	domain    goja.Value // string (domain of address)
}

type jsAttachment struct {
	part *enmime.Part

	fileName    goja.Value // string
	fileModDate goja.Value // time.Time
	contentType goja.Value // string
}

func marshalAttachments(vm *goja.Runtime, protoPtr **goja.Object, attachments []*enmime.Part) (goja.Value, error) {
	a := make([]goja.Value, len(attachments))
	for i := range attachments {
		attachment, err := marshalAttachment(vm, protoPtr, attachments[i])
		if err != nil {
			return nil, err
		}
		a[i] = attachment
	}
	return vm.ToValue(a), nil
}

func marshalAttachment(vm *goja.Runtime, protoPtr **goja.Object, attachment *enmime.Part) (goja.Value, error) {
	proto := *protoPtr
	var err error
	if proto == nil {
		proto, err = jsAttachmentPrototype(vm)
		if err != nil {
			return nil, err
		}
		*protoPtr = proto
	}
	a := &jsAttachment{
		part: attachment,
	}
	o := vm.ToValue(a).ToObject(vm)
	err = o.SetPrototype(proto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func jsAttachmentPrototype(vm *goja.Runtime) (*goja.Object, error) {
	proto := vm.NewObject()
	err := defineGetterProperty(vm, proto, "fileName", jsAttachmentFileName)
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "fileModDate", jsAttachmentFileModDate)
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "contentType", jsAttachmentContentType)
	if err != nil {
		return nil, err
	}
	return proto, nil
}

func jsAttachmentFileName(vm *goja.Runtime, this any) (goja.Value, error) {
	if this, ok := this.(*jsAttachment); ok {
		if this.fileName == nil {
			this.fileName = vm.ToValue(this.part.FileName)
		}
		return this.fileName, nil
	}
	return nil, errUnexpectedThisType(&jsAttachment{}, this)
}

func jsAttachmentFileModDate(vm *goja.Runtime, this any) (goja.Value, error) {
	if this, ok := this.(*jsAttachment); ok {
		if this.fileModDate == nil {
			fileModDate, err := marshalDate(vm, this.part.FileModDate)
			if err != nil {
				return nil, err
			}
			this.fileModDate = fileModDate
		}
		return this.fileModDate, nil
	}
	return nil, errUnexpectedThisType(&jsAttachment{}, this)
}

func jsAttachmentContentType(vm *goja.Runtime, this any) (goja.Value, error) {
	if this, ok := this.(*jsAttachment); ok {
		if this.contentType == nil {
			this.contentType = vm.ToValue(this.part.ContentType)
		}
		return this.contentType, nil
	}
	return nil, errUnexpectedThisType(&jsAttachment{}, this)
}
