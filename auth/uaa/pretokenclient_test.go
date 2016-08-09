package uaa_test

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"

	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/uaa"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pretokenclient", func() {
	var (
		dbUAAAuth        *db.UAAAuth
		redirectURI      string
		uaaProvider      provider.Provider
		constructorError error
	)

	JustBeforeEach(func() {
		uaaProvider, constructorError = uaa.NewProvider(dbUAAAuth, redirectURI)
	})

	Context("when an ssl cert is configured", func() {
		var sslCert string
		BeforeEach(func() {
			sslCert = `-----BEGIN CERTIFICATE-----
MIICsjCCAhugAwIBAgIJAJgyGeIL1aiPMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwIBcNMTUwMzE5MjE1NzAxWhgPMjI4ODEyMzEyMTU3MDFa
MEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJ
bnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAOTD37e9wnQz5fHVPdQdU8rjokOVuFj0wBtQLNO7B2iN+URFaP2wi0KOU0ye
njISc5M/mpua7Op72/cZ3+bq8u5lnQ8VcjewD1+f3LCq+Os7iE85A/mbEyT1Mazo
GGo9L/gfz5kNq78L9cQp5lrD04wF0C05QtL8LVI5N9SqT7mlAgMBAAGjgacwgaQw
HQYDVR0OBBYEFNtN+q97oIhvyUEC+/Sc4q0ASv4zMHUGA1UdIwRuMGyAFNtN+q97
oIhvyUEC+/Sc4q0ASv4zoUmkRzBFMQswCQYDVQQGEwJBVTETMBEGA1UECBMKU29t
ZS1TdGF0ZTEhMB8GA1UEChMYSW50ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkggkAmDIZ
4gvVqI8wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQUFAAOBgQCZKuxfGc/RrMlz
aai4+5s0GnhSuq0CdfnpwZR+dXsjMO6dlrD1NgQoQVhYO7UbzktwU1Hz9Mc3XE7t
HCu8gfq+3WRUgddCQnYJUXtig2yAqmHf/WGR9yYYnfMUDKa85i0inolq1EnLvgVV
K4iijxtW0XYe5R1Od6lWOEKZ6un9Ag==
-----END CERTIFICATE-----
`
			dbUAAAuth = &db.UAAAuth{
				CFCACert: sslCert,
			}

			redirectURI = "some-redirect-url"
		})

		It("doesn't return an error", func() {
			Expect(constructorError).NotTo(HaveOccurred())
		})

		It("constructs HTTP client with disable keep alive context", func() {
			httpClient := uaaProvider.PreTokenClient()
			Expect(httpClient).NotTo(BeNil())
			Expect(httpClient.Transport).NotTo(BeNil())
			Expect(httpClient.Transport.(*http.Transport).DisableKeepAlives).To(BeTrue())
		})

		It("constructs HTTP client with given cert into the cert pool", func() {
			httpClient := uaaProvider.PreTokenClient()
			Expect(httpClient).NotTo(BeNil())
			Expect(httpClient.Transport).NotTo(BeNil())
			tlsConfig := httpClient.Transport.(*http.Transport).TLSClientConfig

			var block *pem.Block
			block, _ = pem.Decode([]byte(sslCert))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig.RootCAs.Subjects()).To(ContainElement(cert.RawSubject))
		})
	})

	Context("when no ssl cert is configured", func() {
		BeforeEach(func() {
			dbUAAAuth = &db.UAAAuth{}
			redirectURI = "some-redirect-url"
		})

		It("doesn't return an error", func() {
			Expect(constructorError).NotTo(HaveOccurred())
		})

		It("constructs HTTP client with disable keep alive context", func() {
			httpClient := uaaProvider.PreTokenClient()
			Expect(httpClient).NotTo(BeNil())
			Expect(httpClient.Transport).NotTo(BeNil())
			Expect(httpClient.Transport.(*http.Transport).DisableKeepAlives).To(BeTrue())
		})

		It("constructs HTTP client where RootCA is nil", func() {
			httpClient := uaaProvider.PreTokenClient()
			Expect(httpClient).NotTo(BeNil())
			Expect(httpClient.Transport).NotTo(BeNil())
			tlsConfig := httpClient.Transport.(*http.Transport).TLSClientConfig

			Expect(tlsConfig).To(BeNil())
		})
	})

	Context("when an invalid ssl cert is configured", func() {
		BeforeEach(func() {
			dbUAAAuth = &db.UAAAuth{CFCACert: "some-invalid-cert"}
			redirectURI = "some-redirect-url"
		})

		It("returns an error", func() {
			Expect(constructorError).To(HaveOccurred())
		})
	})
})
