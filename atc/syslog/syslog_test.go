package syslog_test

import (
	"crypto/tls"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/concourse/concourse/atc/syslog"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/square/certstrap/pkix"
)

var _ = Describe("Syslog", func() {
	var server *testServer
	const (
		hostname = "hostname"
		tag      = "tag"
		message  = "build 123 log"
		eventID  = "123"
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

				ca, err := pkix.CreateCertificateAuthority(key, "", time.Now().Add(time.Hour), "Acme Co", "", "", "", "", nil)
				Expect(err).NotTo(HaveOccurred())

				req, err := pkix.CreateCertificateSigningRequest(key, "", []net.IP{net.IPv4(127, 0, 0, 1)}, nil, nil, "Acme Co", "", "", "", "")
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

				caFile, err := os.CreateTemp("", "ca")
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

				err = sl.Write(hostname, tag, time.Now(), message, eventID)
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

			It("handles invalid cert file paths", func() {
				_, err := syslog.Dial("tls", server.Addr, []string{"non_existent_cert.pem"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to read certificate file"))
			}, 0.2)

			It("handles invalid cert content", func() {
				invalidCertFile, err := os.CreateTemp("", "invalid_cert")
				Expect(err).NotTo(HaveOccurred())
				defer os.Remove(invalidCertFile.Name())

				_, err = invalidCertFile.WriteString("This is not a valid certificate")
				Expect(err).NotTo(HaveOccurred())

				err = invalidCertFile.Close()
				Expect(err).NotTo(HaveOccurred())

				_, err = syslog.Dial("tls", server.Addr, []string{invalidCertFile.Name()})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse certificate"))
			}, 0.2)
		})

		Context("when tls is not set", func() {
			BeforeEach(func() {
				server = newTestServer(nil)
			})

			It("connects and writes to server", func() {
				sl, err := syslog.Dial("tcp", server.Addr, []string{})
				sl.Write(hostname, tag, time.Now(), message, eventID)
				Expect(err).NotTo(HaveOccurred())

				got := <-server.Messages
				Expect(got).To(ContainSubstring(message))
				Expect(got).NotTo(ContainSubstring("build 123 status"))

				err = sl.Close()
				Expect(err).NotTo(HaveOccurred())
			}, 0.2)

			It("formats messages according to RFC5424", func() {
				sl, err := syslog.Dial("tcp", server.Addr, []string{})
				Expect(err).NotTo(HaveOccurred())

				now := time.Date(2023, 4, 15, 12, 30, 45, 123456000, time.UTC)
				err = sl.Write(hostname, tag, now, message, eventID)
				Expect(err).NotTo(HaveOccurred())

				got := <-server.Messages

				Expect(got).To(ContainSubstring("hostname"))
				Expect(got).To(ContainSubstring("tag"))
				Expect(got).To(ContainSubstring("build 123 log"))
				Expect(got).To(ContainSubstring("eventId=\"123\""))

				// Check for general RFC5424 structure with a more flexible pattern
				// The pattern allows for flexibility in the priority and timestamp format
				basicPattern := `<\d+>1 .+ hostname tag - - \[concourse@0 eventId="123"\] build 123 log`
				matched, err := regexp.MatchString(basicPattern, got)
				Expect(err).NotTo(HaveOccurred())
				Expect(matched).To(BeTrue(), "Message format doesn't match expected pattern. Got: "+got)

				err = sl.Close()
				Expect(err).NotTo(HaveOccurred())
			}, 0.2)

			It("sanitizes messages with special characters", func() {
				sl, err := syslog.Dial("tcp", server.Addr, []string{})
				Expect(err).NotTo(HaveOccurred())

				specialMessage := "line1\nline2\rline3\x00line4"
				err = sl.Write(hostname, tag, time.Now(), specialMessage, eventID)
				Expect(err).NotTo(HaveOccurred())

				got := <-server.Messages

				// Check for the sanitized content
				Expect(got).To(ContainSubstring("line1 line2 line3 line4"))

				// Extract the message part (after the structured data)
				parts := strings.SplitN(got, "] ", 2)
				Expect(len(parts)).To(BeNumerically(">", 1), "Message did not contain expected format")

				// Get the message body and trim both trailing newline and whitespace
				messageBody := strings.TrimSpace(parts[1])

				// Verify content is correctly sanitized
				Expect(messageBody).To(Equal("line1 line2 line3 line4"))

				err = sl.Close()
				Expect(err).NotTo(HaveOccurred())
			}, 0.2)

			It("handles concurrent writes", func() {
				sl, err := syslog.Dial("tcp", server.Addr, []string{})
				Expect(err).NotTo(HaveOccurred())

				var wg sync.WaitGroup
				numGoroutines := 5

				wg.Add(numGoroutines)
				for i := 0; i < numGoroutines; i++ {
					go func() {
						defer wg.Done()
						defer GinkgoRecover()

						err := sl.Write(hostname, tag, time.Now(), message, eventID)
						Expect(err).NotTo(HaveOccurred())
					}()
				}

				wg.Wait()

				// Verify we received at least one message
				select {
				case <-server.Messages:
				case <-time.After(time.Second):
					Fail("Didn't receive any messages")
				}

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
				err = sl.Write(hostname, tag, time.Now(), message, eventID)
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

		It("errors with empty address", func() {
			_, err := syslog.Dial("tcp", "", []string{})
			Expect(err).To(HaveOccurred())
		})
	})

})
