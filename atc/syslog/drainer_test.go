package syslog_test

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/syslog"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newFakeBuild(id int) db.Build {
	fakeEventSource := new(dbfakes.FakeEventSource)

	msg1 := json.RawMessage(`{"time":1533744538,"payload":"build ` + strconv.Itoa(id) + ` log"}`)
	fakeEventSource.NextReturnsOnCall(0, event.Envelope{
		Data:    &msg1,
		Event:   "log",
		EventID: "1",
	}, nil)

	msg2 := json.RawMessage(`{"time":1533744538,"status":"build ` + strconv.Itoa(id) + ` status"}`)
	fakeEventSource.NextReturnsOnCall(1, event.Envelope{
		Data:    &msg2,
		Event:   "status",
		EventID: "2",
	}, nil)

	msg3 := json.RawMessage(`{"time":1533744538,"version":{"version":"0.0.1"},"metadata":[{"name":"version","value":"0.0.1"}]}`)
	fakeEventSource.NextReturnsOnCall(2, event.Envelope{
		Data:    &msg3,
		Event:   "finish-get",
		EventID: "3",
	}, nil)

	msg4 := json.RawMessage(`{"time":1533744538,"selected_worker":"example-worker"}`)
	fakeEventSource.NextReturnsOnCall(3, event.Envelope{
		Data:    &msg4,
		Event:   "selected-worker",
		EventID: "4",
	}, nil)

	msg5 := json.RawMessage(`{"time":1533744538}`)
	fakeEventSource.NextReturnsOnCall(4, event.Envelope{
		Data:    &msg5,
		Event:   "initialize-task",
		EventID: "5",
	}, nil)

	fakeEventSource.NextReturnsOnCall(5, event.Envelope{}, db.ErrEndOfBuildEventStream)

	fakeEventSource.NextReturns(event.Envelope{}, db.ErrEndOfBuildEventStream)

	fakeBuild := new(dbfakes.FakeBuild)
	fakeBuild.EventsReturns(fakeEventSource, nil)
	fakeBuild.IDReturns(id)

	return fakeBuild
}

var _ = Describe("Drainer", func() {
	var fakeBuildFactory *dbfakes.FakeBuildFactory
	var server *testServer

	BeforeEach(func() {
		fakeBuildFactory = new(dbfakes.FakeBuildFactory)
		fakeBuildFactory.GetDrainableBuildsReturns([]db.Build{newFakeBuild(123), newFakeBuild(345)}, nil)
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when there are builds that have not been drained", func() {
		Context("when tls is not set", func() {
			BeforeEach(func() {
				server = newTestServer(nil)
			})

			It("drains all build events by tcp", func() {
				testDrainer := syslog.NewDrainer("tcp", server.Addr, "test", []string{}, fakeBuildFactory)
				err := testDrainer.Run(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				got := <-server.Messages
				Expect(got).To(ContainSubstring("build 123 log"))
				Expect(got).To(ContainSubstring("build 345 log"))
				Expect(got).To(ContainSubstring(`get {"version": {"version":"0.0.1"}, "metadata": [{"name":"version","value":"0.0.1"}]`))
				Expect(got).To(ContainSubstring("build 123 status"))
				Expect(got).To(ContainSubstring("build 345 status"))
				Expect(got).To(ContainSubstring("selected worker: example-worker"))
				Expect(got).To(ContainSubstring("task initializing"))
			}, 0.2)
		})

	})
})
