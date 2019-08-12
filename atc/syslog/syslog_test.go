package syslog_test

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/concourse/concourse/atc/syslog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/square/certstrap/pkix"
)

var _ = Describe("Syslog", func() {
	var server *testServer
	const (
		hostname = "hostname"
		tag      = "tag"
		message  = "build 123 log"
	)

	AfterEach(func() {
		server.Close()
	})

	Context("when the address is valid tcp server", func() {
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

			It("connects and writes to server given correct cert", func() {
				sl, err := syslog.Dial("tls", server.Addr, []string{caFilePath})
				Expect(err).NotTo(HaveOccurred())

				err = sl.Write(hostname, tag, time.Now(), message)
				Expect(err).NotTo(HaveOccurred())

				got := <-server.Messages
				Expect(got).To(ContainSubstring(message))
				Expect(got).NotTo(ContainSubstring("build 123 status"))

				err = sl.Close()
				Expect(err).NotTo(HaveOccurred())
			}, 0.2)

			It("fails connects to server given incorrect cert", func() {
				_, err := syslog.Dial("tls", server.Addr, []string{"testdata/incorrect-cert.pem"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("x509: certificate signed by unknown authority"))
			}, 0.2)
		})

		Context("when tls is not set", func() {
			BeforeEach(func() {
				server = newTestServer(nil)
			})

			It("connects and writes to server", func() {
				sl, err := syslog.Dial("tcp", server.Addr, []string{})
				sl.Write(hostname, tag, time.Now(), message)
				Expect(err).NotTo(HaveOccurred())

				got := <-server.Messages
				Expect(got).To(ContainSubstring(message))
				Expect(got).NotTo(ContainSubstring("build 123 status"))

				err = sl.Close()
				Expect(err).NotTo(HaveOccurred())
			}, 0.2)
		})

		Context("after the connection is closed", func() {
			var (
				sl  *syslog.Syslog
				err error
			)

			BeforeEach(func() {
				server = newTestServer(nil)
				sl, err = syslog.Dial("tcp", server.Addr, []string{})
				Expect(err).NotTo(HaveOccurred())

				err = sl.Close()
				Expect(err).ToNot(HaveOccurred())
			})

			It("subsequent ops will error", func() {
				err = sl.Write(hostname, tag, time.Now(), message)
				Expect(err.Error()).To(ContainSubstring("connection already closed"))

				err = sl.Close()
				Expect(err.Error()).To(ContainSubstring("connection already closed"))
			})
		})
	})

	Context("when the address is invalid", func() {
		BeforeEach(func() {
			server = newTestServer(nil)
		})

		It("errors", func() {
			_, err := syslog.Dial("tcp", "bad.address", []string{})
			Expect(err).To(HaveOccurred())
		})
	})

})
