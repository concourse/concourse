package syslog_test

import (
	"context"
	"crypto/tls"
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

func (s *testServer) listenTCP(insecure bool) net.Listener {
	cert, err := tls.LoadX509KeyPair("testdata/cert.pem", "testdata/public.pem")
	Expect(err).NotTo(HaveOccurred())

	var ln net.Listener
	if !insecure {
		config := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		ln, err = tls.Listen("tcp", "127.0.0.1:0", config)
		Expect(err).NotTo(HaveOccurred())
	} else {
		ln, err = net.Listen("tcp", "[::]:0")
		Expect(err).NotTo(HaveOccurred())
	}

	Expect(err).NotTo(HaveOccurred())

	s.Addr = ln.Addr().String()
	return ln
}

func (s *testServer) serveTCP(ln net.Listener) {
	defer GinkgoRecover()
	for {
		select {
		case <-s.Close:
			ln.Close()
			return
		default:
			conn, err := ln.Accept()
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(1 * time.Second)

			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)

			s.Messages <- string(buf[0:n])
		}
	}
}

func newTestServer(insecure bool) *testServer {
	server := testServer{
		Close:    make(chan bool, 2),
		Messages: make(chan string, 20),
	}
	ln := server.listenTCP(insecure)

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
	var s *testServer
	var insecure bool

	BeforeEach(func() {
		fakeBuildFactory = new(dbfakes.FakeBuildFactory)
		fakeBuildFactory.GetDrainableBuildsReturns([]db.Build{newFakeBuild(123), newFakeBuild(345)}, nil)

	})

	Context("when there are builds that have not been drained", func() {

		Context("when tls is enabled", func() {
			JustBeforeEach(func() {
				insecure = false
				s = newTestServer(insecure)
			})

			It("connects to remote server given correct cert", func() {
				testDrainer := syslog.NewDrainer("tls", s.Addr, "test", []string{"testdata/cert.pem"}, fakeBuildFactory)

				err := testDrainer.Run(context.TODO())
				Expect(err).NotTo(HaveOccurred())

				s.Close <- true
			})

			It("fails connects to remote server given incorrect cert", func() {
				testDrainer := syslog.NewDrainer("tls", s.Addr, "test", []string{"testdata/client.pem"}, fakeBuildFactory)

				err := testDrainer.Run(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("x509: certificate signed by unknown authority"))

				s.Close <- true
			})
		})

		Context("when tls is not set", func() {
			JustBeforeEach(func() {
				insecure = true
				s = newTestServer(insecure)
			})

			It("drains all build events by tcp", func() {

				defer GinkgoRecover()
				testDrainer := syslog.NewDrainer("tcp", s.Addr, "test", []string{}, fakeBuildFactory)
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
})
