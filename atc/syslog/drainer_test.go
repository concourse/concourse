package syslog_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/square/certstrap/pkix"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/syslog"
)

type testServer struct {
	Addr     string
	Messages chan string

	ln     net.Listener
	closed bool
	wg     *sync.WaitGroup
}

func newTestServer(cert *tls.Certificate) *testServer {
	server := &testServer{
		Messages: make(chan string, 20),

		wg: new(sync.WaitGroup),
	}

	server.ListenTCP(cert)

	server.wg.Add(1)
	go server.ServeTCP()

	return server
}

func (server *testServer) ListenTCP(cert *tls.Certificate) net.Listener {
	var ln net.Listener

	var err error
	if cert != nil {
		config := &tls.Config{
			Certificates: []tls.Certificate{*cert},
		}
		server.ln, err = tls.Listen("tcp", "127.0.0.1:0", config)
		Expect(err).NotTo(HaveOccurred())
	} else {
		server.ln, err = net.Listen("tcp", "[::]:0")
		Expect(err).NotTo(HaveOccurred())
	}

	Expect(err).NotTo(HaveOccurred())

	server.Addr = server.ln.Addr().String()

	return ln
}

func (server *testServer) ServeTCP() {
	defer server.wg.Done()
	defer GinkgoRecover()

	for {
		conn, err := server.ln.Accept()
		if server.closed {
			return
		}

		Expect(err).NotTo(HaveOccurred())

		time.Sleep(100 * time.Millisecond)

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)

		// expect bad certificate from 'bad cert' test
		if err != nil && err.Error() == "remote error: tls: bad certificate" {
			continue
		}

		Expect(err).NotTo(HaveOccurred())

		server.Messages <- string(buf[0:n])
	}
}

func (server *testServer) Close() {
	server.closed = true
	server.ln.Close()
	server.wg.Wait()
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
	var server *testServer

	BeforeEach(func() {
		fakeBuildFactory = new(dbfakes.FakeBuildFactory)
		fakeBuildFactory.GetDrainableBuildsReturns([]db.Build{newFakeBuild(123), newFakeBuild(345)}, nil)
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when there are builds that have not been drained", func() {
		Context("when tls is enabled", func() {
			var caFilePath string

			BeforeEach(func() {
				key, err := pkix.CreateRSAKey(1024)
				Expect(err).NotTo(HaveOccurred())

				ca, err := pkix.CreateCertificateAuthority(key, "", time.Now().Add(time.Hour), "Acme Co", "", "", "", "")
				Expect(err).NotTo(HaveOccurred())

				req, err := pkix.CreateCertificateSigningRequest(key, "", []net.IP{net.IPv4(127, 0, 0, 1)}, nil, "Acme Co", "", "", "", "")
				Expect(err).NotTo(HaveOccurred())

				cert, err := pkix.CreateCertificateHost(ca, key, req, time.Now().Add(time.Hour))
				Expect(err).NotTo(HaveOccurred())

				keyPEM, err := key.ExportPrivate()
				Expect(err).NotTo(HaveOccurred())

				caPEM, err := ca.Export()
				Expect(err).NotTo(HaveOccurred())

				certPEM, err := cert.Export()
				Expect(err).NotTo(HaveOccurred())

				tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
				Expect(err).NotTo(HaveOccurred())

				caFile, err := ioutil.TempFile("", "ca")
				Expect(err).NotTo(HaveOccurred())

				_, err = caFile.Write(caPEM)
				Expect(err).NotTo(HaveOccurred())

				err = caFile.Close()
				Expect(err).NotTo(HaveOccurred())

				caFilePath = caFile.Name()

				server = newTestServer(&tlsCert)
			})

			AfterEach(func() {
				err := os.RemoveAll(caFilePath)
				Expect(err).ToNot(HaveOccurred())
			})

			It("connects to remote server given correct cert", func() {
				testDrainer := syslog.NewDrainer("tls", server.Addr, "test", []string{caFilePath}, fakeBuildFactory)

				err := testDrainer.Run(context.TODO())
				Expect(err).NotTo(HaveOccurred())
			})

			It("fails connects to remote server given incorrect cert", func() {
				testDrainer := syslog.NewDrainer("tls", server.Addr, "test", []string{"testdata/incorrect-cert.pem"}, fakeBuildFactory)

				err := testDrainer.Run(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("x509: certificate signed by unknown authority"))
			})
		})

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
