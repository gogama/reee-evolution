package rule

import (
	"net/mail"

	"github.com/dop251/goja"
	"github.com/gogama/reee-evolution/daemon"
)

type JSMessage struct {
	runtime *goja.Runtime
	msg     *daemon.Message

	// Cached field values.
	from     goja.Value // cachedAddress
	to       goja.Value // cachedAddress
	subject  goja.Value // string
	date     goja.Value // time.Time
	cc       goja.Value // cachedAddress
	bcc      goja.Value // cachedAddress
	fullText goja.Value
}

func (m *JSMessage) From() goja.Value {
	return m.cachedAddress(&m.from, "From")
}

func (m *JSMessage) cachedAddress(field *goja.Value, headerName string) goja.Value {
	if *field == nil {
		var ca jsCachedAddress
		headerValue := m.msg.Envelope.GetHeader(headerName)
		ca.headerValue = m.runtime.ToValue(headerValue)
		list, err := mail.ParseAddressList(headerValue)
		if err != nil {
			// TODO: Somehow log something here. We should have a bound logger.
		} else {
			ca.mailboxes = make([]goja.Value, len(list))
			ca.length = m.runtime.ToValue(len(list))
			for i := range list {
				ca.mailboxes[i] = m.runtime.ToValue(jsMailbox{
					name:    m.runtime.ToValue(list[i].Name),
					address: m.runtime.ToValue(list[i].Address),
				})
			}
			*field = m.runtime.ToValue(ca)
		}
	}
	return *field
}

type jsCachedAddress struct {
	headerValue goja.Value
	length      goja.Value
	mailboxes   []goja.Value
}

func (ca *jsCachedAddress) Header() goja.Value {
	return ca.headerValue
}

func (ca *jsCachedAddress) Length() goja.Value {
	return ca.length
}

func (ca *jsCachedAddress) Item(i int) goja.Value {
	if 0 <= i && i <= len(ca.mailboxes) {
		return ca.mailboxes[i]
	}
	return goja.Undefined()
}

type jsMailbox struct {
	name    goja.Value // string
	address goja.Value // string
}

func (m *jsMailbox) Name() goja.Value {
	return m.name
}

func (m *jsMailbox) Address() goja.Value {
	return m.address
}

/**

Desired JavaScript access of mail item.

	m.subject
	m.fullText
	m.date
	m.from.header
	m.from.item(0).
	m.from.item(0).address
	m.toHeader
	m.to[0].name
	m.to[0].address
	m.ccHeader
	m.cc[0].name
	m.cc[0].address
*/
