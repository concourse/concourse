package syslog_test

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/syslog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func newFakeBuild(id int) db.Build {
	fakeEventSource := new(dbfakes.FakeEventSource)

	msg1 := json.RawMessage(`{"time":1533744538,"payload":"build ` + strconv.Itoa(id) + ` log"}`)

	fakeEventSource.NextReturnsOnCall(0, event.Envelope{
		Data:  &msg1,
		Event: "log",
	}, nil)

	msg2 := json.RawMessage(`{"time":1533744538,"payload":"build ` + strconv.Itoa(id) + ` status"}`)

	fakeEventSource.NextReturnsOnCall(1, event.Envelope{
		Data:  &msg2,
		Event: "status",
	}, nil)

	fakeEventSource.NextReturnsOnCall(2, event.Envelope{}, db.ErrEndOfBuildEventStream)

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
				Expect(got).NotTo(ContainSubstring("build 123 status"))
				Expect(got).NotTo(ContainSubstring("build 345 status"))
			}, 0.2)
		})

	})
})
