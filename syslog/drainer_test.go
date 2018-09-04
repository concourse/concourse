package syslog_test

import (
	"context"
	"encoding/json"
	"net"
	"strconv"

	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/syslog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testServer struct {
	Addr     string
	Close    chan bool
	Messages chan string
}

func (s *testServer) listenTCP() net.Listener {
	ln, err := net.Listen("tcp", "[::]:0")
	if err != nil {
		panic("listen error")
	}

	s.Addr = ln.Addr().String()
	return ln
}

func (s *testServer) serveTCP(ln net.Listener) {
	for {
		select {
		case <-s.Close:
			ln.Close()
			return
		default:
			conn, err := ln.Accept()
			if err != nil {
				panic("Accept error")
			}

			time.Sleep(1 * time.Second)

			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				panic("Read error")
			} else {
				s.Messages <- string(buf[0:n])
			}
		}
	}
}

func newTestServer() *testServer {
	server := testServer{
		Close:    make(chan bool, 2),
		Messages: make(chan string, 20),
	}
	ln := server.listenTCP()
	go server.serveTCP(ln)
	return &server
}

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

	BeforeEach(func() {
		fakeBuildFactory = new(dbfakes.FakeBuildFactory)
		fakeBuildFactory.GetDrainableBuildsReturns([]db.Build{newFakeBuild(123), newFakeBuild(345)}, nil)
	})

	Context("when there are builds that have not been drained", func() {
		It("drains all build events", func() {
			s := newTestServer()

			testDrainer := syslog.NewDrainer("tcp", s.Addr, "test", fakeBuildFactory)
			err := testDrainer.Run(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			s.Close <- true

			got := <-s.Messages
			Expect(got).To(ContainSubstring("build 123 log"))
			Expect(got).To(ContainSubstring("build 345 log"))
			Expect(got).NotTo(ContainSubstring("build 123 status"))
			Expect(got).NotTo(ContainSubstring("build 345 status"))
		}, 0.2)
	})
})
