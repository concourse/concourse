package event_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/event"
)

type fakeEvent struct {
	Hello      string `json:"hello"`
	AddedField string `json:"added"`
}

func (fakeEvent) EventType() atc.EventType  { return "fake" }
func (fakeEvent) Version() atc.EventVersion { return "5.1" }

var _ = Describe("ParseEvent", func() {
	BeforeEach(func() {
		event.RegisterEvent(fakeEvent{})
	})

	It("can parse an older event type with a compatible major version", func() {
		e, err := event.ParseEvent("5.0", "fake", []byte(`{"hello":"sup"}`))
		Expect(err).ToNot(HaveOccurred())
		Expect(e).To(Equal(fakeEvent{Hello: "sup"}))
	})

	It("can parse a newer event type with a compatible major version", func() {
		e, err := event.ParseEvent("5.3", "fake", []byte(`{"hello":"sup","future":"field"}`))
		Expect(err).ToNot(HaveOccurred())
		Expect(e).To(Equal(fakeEvent{Hello: "sup"}))
	})

	It("fails to parse if the type is unknown", func() {
		_, err := event.ParseEvent("4.0", "fake-unknown", []byte(`{"hello":"sup"}`))
		Expect(err).To(Equal(event.UnknownEventTypeError{
			Type: "fake-unknown",
		}))
	})

	It("fails to parse if the version is incompatible", func() {
		_, err := event.ParseEvent("4.0", "fake", []byte(`{"hello":"sup"}`))
		Expect(err).To(Equal(event.UnknownEventVersionError{
			Type:          "fake",
			Version:       "4.0",
			KnownVersions: []string{"5.1"},
		}))
	})
})
