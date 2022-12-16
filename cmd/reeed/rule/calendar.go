package rule

import (
	"bytes"
	"fmt"
	"net/mail"
	"strings"

	ics "github.com/arran4/golang-ical"
	"github.com/dop251/goja"
	"github.com/jhillyerd/enmime"
)

func parseCalendar(parts ...[]*enmime.Part) *ics.Calendar {
	for i := range parts {
		for j := range parts[i] {
			part := parts[i][j]
			if part.ContentType != "text/calendar" {
				continue
			}
			reader := bytes.NewReader(part.Content)
			calendar, err := ics.ParseCalendar(reader)
			if err != nil {
				// TODO: Find a way to log this and continue.
				return nil
			}
			return calendar
		}
	}
	return nil
}
func marshalCalendar(cont *vmContainer, calendar *ics.Calendar) (goja.Value, error) {
	if cont.calendarProto == nil {
		proto, err := jsCalendarPrototype(cont.vm)
		if err != nil {
			return nil, err
		}
		cont.calendarProto = proto
	}
	list := calendar.Events()
	events := make([]goja.Value, len(list))
	for i := range list {
		fmt.Println("HELLO? There's an event here in this list", list[i])
		event, err := marshalCalendarEvent(cont, list[i])
		if err != nil {
			return nil, err
		}
		events[i] = event
	}
	c := &jsCalendar{
		events: cont.vm.ToValue(events),
	}
	o := cont.vm.ToValue(c).ToObject(cont.vm)
	err := o.SetPrototype(cont.calendarProto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

type jsCalendar struct {
	events goja.Value
}

func jsCalendarPrototype(vm *goja.Runtime) (*goja.Object, error) {
	proto := vm.NewObject()
	err := defineGetterProperty(vm, proto, "events", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsCalendar); ok {
			return this.events, nil
		}
		return nil, errUnexpectedThisType(&jsCalendar{}, this)
	})
	if err != nil {
		return nil, err
	}
	return proto, nil
}

func marshalCalendarEvent(cont *vmContainer, event *ics.VEvent) (goja.Value, error) {
	if cont.calendarEventProto == nil {
		proto, err := jsCalendarEventPrototype(cont.vm)
		if err != nil {
			return nil, err
		}
		cont.calendarEventProto = proto
	}
	summary := goja.Null()
	if value := event.GetProperty(ics.ComponentProperty(ics.PropertySummary)); value != nil {
		summary = cont.vm.ToValue(value.Value)
	}
	list := event.Attendees()
	attendees := make([]goja.Value, len(list))
	for i := range list {
		attendee, err := marshalCalendarAttendee(cont, list[i])
		if err != nil {
			return nil, err
		}
		attendees[i] = attendee
	}
	c := &jsCalendarEvent{
		summary:   summary,
		attendees: cont.vm.ToValue(attendees),
	}
	o := cont.vm.ToValue(c).ToObject(cont.vm)
	err := o.SetPrototype(cont.calendarEventProto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func jsCalendarEventPrototype(vm *goja.Runtime) (*goja.Object, error) {
	proto := vm.NewObject()
	err := defineGetterProperty(vm, proto, "summary", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsCalendarEvent); ok {
			return this.summary, nil
		}
		return nil, errUnexpectedThisType(&jsCalendarEvent{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "attendees", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsCalendarEvent); ok {
			return this.attendees, nil
		}
		return nil, errUnexpectedThisType(&jsCalendarEvent{}, this)
	})
	if err != nil {
		return nil, err
	}
	return proto, nil
}

type jsCalendarEvent struct {
	summary   goja.Value
	attendees goja.Value
}

func marshalCalendarAttendee(cont *vmContainer, attendee *ics.Attendee) (goja.Value, error) {
	if cont.calendarAttendeeProto == nil {
		proto, err := jsCalendarAttendeePrototype(cont.vm)
		if err != nil {
			return nil, err
		}
		cont.calendarAttendeeProto = proto
	}
	email := attendee.Value
	if strings.HasPrefix(email, "MAILTO:") || strings.HasPrefix(email, "mailto:") {
		email = email[7:]
	}
	address := mail.Address{
		Address: email,
	}
	if list := attendee.ICalParameters[string(ics.ParameterCn)]; len(list) > 0 {
		address.Name = list[0]
	}
	mailbox, err := marshalMailbox(cont, &address)
	if err != nil {
		return nil, err
	}
	role := goja.Null()
	if list := attendee.ICalParameters[string(ics.ParameterRole)]; len(list) > 0 {
		role = cont.vm.ToValue(list[0])
	}
	participationStatus := goja.Null()
	if value := attendee.ParticipationStatus(); value != "" {
		participationStatus = cont.vm.ToValue(string(value))
	}
	a := &jsCalendarAttendee{
		mailbox:             mailbox,
		role:                role,
		participationStatus: participationStatus,
	}
	o := cont.vm.ToValue(a).ToObject(cont.vm)
	err = o.SetPrototype(cont.calendarAttendeeProto)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func jsCalendarAttendeePrototype(vm *goja.Runtime) (*goja.Object, error) {
	proto := vm.NewObject()
	err := defineGetterProperty(vm, proto, "mailbox", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsCalendarAttendee); ok {
			return this.mailbox, nil
		}
		return nil, errUnexpectedThisType(&jsCalendarAttendee{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "role", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsCalendarAttendee); ok {
			return this.role, nil
		}
		return nil, errUnexpectedThisType(&jsCalendarAttendee{}, this)
	})
	if err != nil {
		return nil, err
	}
	err = defineGetterProperty(vm, proto, "participationStatus", func(_ *goja.Runtime, this any) (goja.Value, error) {
		if this, ok := this.(*jsCalendarAttendee); ok {
			return this.participationStatus, nil
		}
		return nil, errUnexpectedThisType(&jsCalendarAttendee{}, this)
	})
	if err != nil {
		return nil, err
	}
	return proto, nil
}

type jsCalendarAttendee struct {
	mailbox             goja.Value
	role                goja.Value
	participationStatus goja.Value
}
