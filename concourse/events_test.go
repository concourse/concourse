package concourse_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/go-concourse/concourse/fakes"
)

var _ = Describe("ATC Handler Events", func() {
	Describe("Events", func() {
		var fakeConnection *fakes.FakeConnection

		BeforeEach(func() {
			fakeConnection = new(fakes.FakeConnection)
			client = concourse.NewClient(fakeConnection)
		})

		It("returns events that can stream events", func() {
			expectedEventStream := sse.EventSource{}
			expectedBuildID := "1"
			fakeConnection.ConnectToEventStreamReturns(&expectedEventStream, nil)

			_, err := client.BuildEvents(expectedBuildID)
			Expect(err).NotTo(HaveOccurred())

			request := fakeConnection.ConnectToEventStreamArgsForCall(0)
			Expect(request).To(Equal(concourse.Request{
				RequestName: atc.BuildEvents,
				Params:      rata.Params{"build_id": expectedBuildID},
			}))
		})
	})
})
