package uaa_test

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"

	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/uaa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UAA Provider", func() {
	var (
		authConfig      *uaa.UAAAuthConfig
		redirectURI     string
		uaaProvider     provider.Provider
		found           bool
		uaaTeamProvider uaa.UAATeamProvider
		sslCert         auth.FileContentsFlag
	)

	BeforeEach(func() {
		authConfig = nil
	})

	Describe("PreTokenClient", func() {
		JustBeforeEach(func() {
			uaaTeamProvider = uaa.UAATeamProvider{}
			uaaProvider, found = uaaTeamProvider.ProviderConstructor(authConfig, redirectURI)
			Expect(found).To(BeTrue())
		})

		Context("when an ssl cert is configured", func() {
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
				authConfig = &uaa.UAAAuthConfig{
					CFCACert: sslCert,
				}

				redirectURI = "some-redirect-url"
			})

			It("doesn't return an error", func() {
				_, err := uaaProvider.PreTokenClient()
				Expect(err).NotTo(HaveOccurred())
			})

			It("constructs HTTP client with disable keep alive context", func() {
				httpClient, err := uaaProvider.PreTokenClient()
				Expect(err).NotTo(HaveOccurred())
				Expect(httpClient).NotTo(BeNil())
				Expect(httpClient.Transport).NotTo(BeNil())
				Expect(httpClient.Transport.(*http.Transport).DisableKeepAlives).To(BeTrue())
			})

			It("constructs HTTP client with given cert into the cert pool", func() {
				httpClient, err := uaaProvider.PreTokenClient()
				Expect(err).NotTo(HaveOccurred())
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
				authConfig = &uaa.UAAAuthConfig{}
				redirectURI = "some-redirect-url"
			})

			It("doesn't return an error", func() {
				_, err := uaaProvider.PreTokenClient()
				Expect(err).NotTo(HaveOccurred())
			})

			It("constructs HTTP client with disable keep alive context", func() {
				httpClient, err := uaaProvider.PreTokenClient()
				Expect(err).NotTo(HaveOccurred())
				Expect(httpClient).NotTo(BeNil())
				Expect(httpClient.Transport).NotTo(BeNil())
				Expect(httpClient.Transport.(*http.Transport).DisableKeepAlives).To(BeTrue())
			})

			It("constructs HTTP client where RootCA is nil", func() {
				httpClient, err := uaaProvider.PreTokenClient()
				Expect(err).NotTo(HaveOccurred())
				Expect(httpClient).NotTo(BeNil())
				Expect(httpClient.Transport).NotTo(BeNil())
				tlsConfig := httpClient.Transport.(*http.Transport).TLSClientConfig

				Expect(tlsConfig).To(BeNil())
			})
		})

		Context("when an invalid ssl cert is configured", func() {
			BeforeEach(func() {
				sslCert = "some-invalid-cert"
				authConfig = &uaa.UAAAuthConfig{
					CFCACert: sslCert,
				}
				redirectURI = "some-redirect-url"
			})

			It("returns an error", func() {
				_, err := uaaProvider.PreTokenClient()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("AuthMethod", func() {
		var (
			authMethod provider.AuthMethod
			authConfig *uaa.UAAAuthConfig
		)
		BeforeEach(func() {
			authConfig = &uaa.UAAAuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates path for route", func() {
			Expect(authMethod).To(Equal(provider.AuthMethod{
				Type:        provider.AuthTypeOAuth,
				DisplayName: "UAA",
				AuthURL:     "http://bum-bum-bum.com/auth/uaa?team_name=dudududum",
			}))
		})
	})
})
