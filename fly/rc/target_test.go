package rc_test

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/concourse/concourse/fly/rc"
	fakes "github.com/concourse/concourse/go-concourse/concourse/concoursefakes"
	"golang.org/x/oauth2"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Target", func() {
	const rootCA = `-----BEGIN CERTIFICATE-----
MIICPDCCAaUCEHC65B0Q2Sk0tjjKewPMur8wDQYJKoZIhvcNAQECBQAwXzELMAkG
A1UEBhMCVVMxFzAVBgNVBAoTDlZlcmlTaWduLCBJbmMuMTcwNQYDVQQLEy5DbGFz
cyAzIFB1YmxpYyBQcmltYXJ5IENlcnRpZmljYXRpb24gQXV0aG9yaXR5MB4XDTk2
MDEyOTAwMDAwMFoXDTI4MDgwMTIzNTk1OVowXzELMAkGA1UEBhMCVVMxFzAVBgNV
BAoTDlZlcmlTaWduLCBJbmMuMTcwNQYDVQQLEy5DbGFzcyAzIFB1YmxpYyBQcmlt
YXJ5IENlcnRpZmljYXRpb24gQXV0aG9yaXR5MIGfMA0GCSqGSIb3DQEBAQUAA4GN
ADCBiQKBgQDJXFme8huKARS0EN8EQNvjV69qRUCPhAwL0TPZ2RHP7gJYHyX3KqhE
BarsAx94f56TuZoAqiN91qyFomNFx3InzPRMxnVx0jnvT0Lwdd8KkMaOIG+YD/is
I19wKTakyYbnsZogy1Olhec9vn2a/iRFM9x2Fe0PonFkTGUugWhFpwIDAQABMA0G
CSqGSIb3DQEBAgUAA4GBALtMEivPLCYATxQT3ab7/AoRhIzzKBxnki98tsX63/Do
lbwdj2wsqFHMc9ikwFPwTtYmwHYBV4GSXiHx0bH/59AhWM1pF+NEHJwZRDmJXNyc
AA9WjQKZ7aKQRUzkuxCkPfAyAw7xzvjoyVGM5mKf5p/AfbdynMk2OmufTqj/ZA1k
-----END CERTIFICATE-----
`

	const clientCert = `-----BEGIN CERTIFICATE-----
MIIDZTCCAk2gAwIBAgIUX7Uw88QRy27mQJqKLoCypKgEwyQwDQYJKoZIhvcNAQEL
BQAwQjELMAkGA1UEBhMCWFgxFTATBgNVBAcMDERlZmF1bHQgQ2l0eTEcMBoGA1UE
CgwTRGVmYXVsdCBDb21wYW55IEx0ZDAeFw0yMDA4MjYxMzQyMzVaFw0yMDA5MjUx
MzQyMzVaMEIxCzAJBgNVBAYTAlhYMRUwEwYDVQQHDAxEZWZhdWx0IENpdHkxHDAa
BgNVBAoME0RlZmF1bHQgQ29tcGFueSBMdGQwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQCnN888R5+LZQ2ngGDpcNuFLwxgIewZCSJizWlXDfGNgQZ7g/rX
jsTDfsax9t5cFafvh3fC2eR5GCddNIggsA8DFEwss7e1b0bljDesqtRlXLofH0Q6
Crgj9gbzorsRqLjbLNpuKD7qTeuyaaI/ofRn4blu6hx8T2r0DRgNRlTcSz/0GByK
xg99aH4BmTSM6VRtkxOoXnRmTbe8EJeeva7nNAqIpQzGTnhOvEOnQsfM1wpgASS3
dvQxgZi9vFPqg/bbdSYzBm1SctOXpHHERR5pgZCxP2vV6ZSxMH9cyw6jiYmSXzCq
6GCRyP3ofIZi0oI/Csa10Z1l26v0ZLOJdGKtAgMBAAGjUzBRMB0GA1UdDgQWBBQV
UGnNB+GFX6Rs3eKrWjIHSgEcFTAfBgNVHSMEGDAWgBQVUGnNB+GFX6Rs3eKrWjIH
SgEcFTAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBGqpQ+8FX5
uCYd7PPeD1WNoKlIeA2FfX+SJcCFJY4MdqtjBFPTtUCMD3medXezujjwQeVR8Cup
HK+3O/YEG/bNayqA8nZBF2tNGeNHXNl6iSYV1UjKvELUvh5S5QRrvrbNTmUJuSsL
WHhU/KEi/FYPeOzQEGxR7meRJWggBM57b9s81W+I+XrJ71wfaYiI70GbKeRoHWA0
gnhXZV7MBiEgNSHOGaavjkF22E2At40XHWL87dUJZVMp4SEu/e4XJeWvum21VDrs
mlR/qNacLCRV2kECu+9YEtT//lb5njdHkMMeXmYp4lrNpCuXzbAWpRo8VER6RQ2Q
6vNmvRoBV5G1
-----END CERTIFICATE-----
`

	const clientKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCnN888R5+LZQ2n
gGDpcNuFLwxgIewZCSJizWlXDfGNgQZ7g/rXjsTDfsax9t5cFafvh3fC2eR5GCdd
NIggsA8DFEwss7e1b0bljDesqtRlXLofH0Q6Crgj9gbzorsRqLjbLNpuKD7qTeuy
aaI/ofRn4blu6hx8T2r0DRgNRlTcSz/0GByKxg99aH4BmTSM6VRtkxOoXnRmTbe8
EJeeva7nNAqIpQzGTnhOvEOnQsfM1wpgASS3dvQxgZi9vFPqg/bbdSYzBm1SctOX
pHHERR5pgZCxP2vV6ZSxMH9cyw6jiYmSXzCq6GCRyP3ofIZi0oI/Csa10Z1l26v0
ZLOJdGKtAgMBAAECggEABX72Frsb6U729exoQwPskyIKvBYhVmlQcgLiVXQl3krB
VcnusqsEmJBQI4VDpa8oh9zh+MuEkN5UXOHfH4Pp2mYOYuG9Rf9USzMimVA8DuDP
VTqH2YiEqNnrPJK6p0fuW3XL8BbuinDpMEH8jS7bg5aNq7GSIhvSHhdYFQecvmjF
JRhQCcp/kvy7YgCG+Eg9bNYyGTKjx7WEWxY0BtF4saqiGN3l8T4gGHMdk/RBCkPZ
QG69V0FyC5iAi07J/x9eNSIwuq4egzzdraklnaLcrsSEOvjvTDl9+xUEESFW/OTO
b6RD5uGahhjJoX+WCOX01E7EN2EP1QgU+Fookq4YWQKBgQDSbIyRjQokH/DnNon/
Fn8/cChz1vPgMDHojRP/pxk/gMC8Jcp0JrpKFMZcPphkuc5O6rjf8BExUncESXec
uAcr9+G/TS1fA8eRpgFS3gmSwmQsJHRiNprhyGKM6WOhIa1Oi5ku6ZGZZf/GGaEA
Fv2jqV1RanVxwU3mNW8rP7V/6wKBgQDLb5olPo/IfBf9oP9nwhiWK1ZPErICdiH7
UtADDCTZhAE9VNSOvf78PduEKXVDKUWj5IaSgaijIj6edcYbCmRjqzG5g1choyAY
hZ/hAUX3X6nA1lc/ND/v7epVjZ8I7vzf72VRfDpTNdnjoRMzhR4c1HpKfdZwzTXT
AWFWRaAZxwKBgGmUY3eIb+UuTZ6Fg/oE3LYE3Zc57EW5iOEpIDavLgDp5krBH3Lm
F6SiBeE02xv3CqgYJ8jc2JOJ0APLpQNyZs7N4mwtGi3JZLIUvCdLFzyW4tIvPGIn
CdFtzNztIbswfZeifarHMPHp9sr8AwdbgcpDaXo3U1RPbHmsp+noXnYfAoGBAMp9
OyD3NIaJfheluJK+T1qpqC7snOJ2UzylIQbnf4ZCLjmtxiSOWM8ZgvX5jg5bdkW7
oXcSN5io7UssTxN7NJFARS4x3PhONhQybQC5E7s2LPEUZ6MxjrJyTVz6qeFqf6kl
z+Nbk3Jfl5FLMqGFToPDujWLK3b7yydLqGcGxmThAoGBALrluhB3186MZsZGPccc
gEyHgI3smYf7BBSlkOOj3Ws8tckiVv0WXKmk8kpB2H4Z49l1vkqT0fqLdZGioUCT
pOyP4gMpJpf2MofQq96Ng4GFSoSl7i52olbd7e6P6IGP01AAwwlmF64dl5oGk4hW
Lfkzl8ebb+tt0XFMUFc42WNr
-----END PRIVATE KEY-----
`

	var (
		tmpDir string
		flyrc  string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "fly-test")
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("HOME", tmpDir)

		flyrc = filepath.Join(userHomeDir(), ".flyrc")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Complete", func() {
		BeforeEach(func() {
			flyrcContents := `targets:
  some-target-b: {}
  some-target-a: {}
  another-target: {}
  `
			ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("lists matching targets in order", func() {
			name := rc.TargetName("some-target")
			comps := name.Complete("some-target")
			Expect(comps).To(HaveLen(2))
			Expect(comps[0].Item).To(Equal("some-target-a"))
			Expect(comps[1].Item).To(Equal("some-target-b"))
		})
	})

	Describe("LoadTarget", func() {
		Context("when there is no ca-cert", func() {
			BeforeEach(func() {
				flyrcContents := `targets:
  some-target:
    api: http://concourse.com
    insecure: true
    token:
      type: Bearer
      value: some-token`
				ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
			})

			It("loads target with correct transport", func() {
				target, err := rc.LoadTarget("some-target", false)
				Expect(err).NotTo(HaveOccurred())
				transport, ok := target.Client().HTTPClient().Transport.(*oauth2.Transport)
				Expect(ok).To(BeTrue())
				Expect((*transport).Source).To(Equal(oauth2.StaticTokenSource(&oauth2.Token{
					TokenType:   "Bearer",
					AccessToken: "some-token",
				})))
				base, ok := (*transport).Base.(*http.Transport)
				Expect(ok).To(BeTrue())
				Expect((*base).TLSClientConfig).To(Equal(&tls.Config{
					InsecureSkipVerify: true,
					RootCAs:            nil,
					Certificates:       []tls.Certificate{},
				}))
			})
		})

		Context("when there is ca-cert", func() {
			BeforeEach(func() {
				flyrcConfig := rc.RC{
					Targets: map[rc.TargetName]rc.TargetProps{
						"some-target": {
							API:      "http://concourse.com",
							CACert:   rootCA,
							TeamName: "some-team",
							Token: &rc.TargetToken{
								Type:  "Bearer",
								Value: "some-token",
							},
						},
					},
				}
				flyrcContents, err := yaml.Marshal(flyrcConfig)
				Expect(err).NotTo(HaveOccurred())

				ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
			})

			It("loads target with correct transport", func() {
				target, err := rc.LoadTarget("some-target", false)
				Expect(err).NotTo(HaveOccurred())
				transport, ok := target.Client().HTTPClient().Transport.(*oauth2.Transport)
				Expect(ok).To(BeTrue())
				base, ok := (*transport).Base.(*http.Transport)
				Expect(ok).To(BeTrue())

				var expectedCaCertPool *x509.CertPool
				if runtime.GOOS != "windows" {
					expectedCaCertPool, err = x509.SystemCertPool()
					Expect(err).NotTo(HaveOccurred())
				} else {
					expectedCaCertPool = x509.NewCertPool()
				}
				ok = expectedCaCertPool.AppendCertsFromPEM([]byte(rootCA))
				Expect(ok).To(BeTrue())

				config := (*base).TLSClientConfig
				Expect(config.InsecureSkipVerify).To(Equal(false))
				// x509.CertPool lazyily loads certs, which breaks direct equality comparisions
				Expect(config.RootCAs.Subjects()).To(Equal(expectedCaCertPool.Subjects()))
				Expect(config.Certificates).To(HaveLen(0))
			})
		})

		Context("when there is a client certificate path and a client key path", func() {
			BeforeEach(func() {
				certPath := filepath.Join(userHomeDir(), "client.pem")
				keyPath := filepath.Join(userHomeDir(), "client.key")

				err := ioutil.WriteFile(certPath, []byte(clientCert), 0600)

				Expect(err).ToNot(HaveOccurred())

				err = ioutil.WriteFile(keyPath, []byte(clientKey), 0600)
				Expect(err).ToNot(HaveOccurred())

				flyrcConfig := rc.RC{
					Targets: map[rc.TargetName]rc.TargetProps{
						"some-target": {
							API:            "http://concourse.com",
							ClientCertPath: certPath,
							ClientKeyPath:  keyPath,
							TeamName:       "some-team",
							Token: &rc.TargetToken{
								Type:  "Bearer",
								Value: "some-token",
							},
						},
					},
				}
				flyrcContents, err := yaml.Marshal(flyrcConfig)
				Expect(err).NotTo(HaveOccurred())

				ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
			})

			It("loads target with correct transport", func() {
				target, err := rc.LoadTarget("some-target", false)
				Expect(err).NotTo(HaveOccurred())
				transport, ok := target.Client().HTTPClient().Transport.(*oauth2.Transport)
				Expect(ok).To(BeTrue())
				base, ok := (*transport).Base.(*http.Transport)
				Expect(ok).To(BeTrue())

				expectedX509Cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))

				Expect((*base).TLSClientConfig).To(Equal(&tls.Config{
					InsecureSkipVerify: false,
					Certificates:       []tls.Certificate{expectedX509Cert},
				}))
			})
		})

		Context("when there is a client certificate path, but no client key path", func() {
			BeforeEach(func() {
				certPath := filepath.Join(userHomeDir(), "client.pem")

				err := ioutil.WriteFile(certPath, []byte(clientCert), 0600)
				Expect(err).ToNot(HaveOccurred())

				flyrcConfig := rc.RC{
					Targets: map[rc.TargetName]rc.TargetProps{
						"some-target": {
							API:            "http://concourse.com",
							ClientCertPath: certPath,
							TeamName:       "some-team",
							Token: &rc.TargetToken{
								Type:  "Bearer",
								Value: "some-token",
							},
						},
					},
				}
				flyrcContents, err := yaml.Marshal(flyrcConfig)
				Expect(err).NotTo(HaveOccurred())

				ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
			})

			It("warns the user and exits with failure", func() {
				_, err := rc.LoadTarget("some-target", false)
				Expect(err).Should(MatchError("A client certificate may not be declared without defining a client key"))
			})
		})

		Context("when there is a client key path, but no client certificate path", func() {
			BeforeEach(func() {
				keyPath := filepath.Join(userHomeDir(), "client.key")

				err := ioutil.WriteFile(keyPath, []byte(clientKey), 0600)
				Expect(err).ToNot(HaveOccurred())

				flyrcConfig := rc.RC{
					Targets: map[rc.TargetName]rc.TargetProps{
						"some-target": {
							API:           "http://concourse.com",
							ClientKeyPath: keyPath,
							TeamName:      "some-team",
							Token: &rc.TargetToken{
								Type:  "Bearer",
								Value: "some-token",
							},
						},
					},
				}
				flyrcContents, err := yaml.Marshal(flyrcConfig)
				Expect(err).NotTo(HaveOccurred())

				ioutil.WriteFile(flyrc, []byte(flyrcContents), 0777)
			})

			It("warns the user and exits with failure", func() {
				_, err := rc.LoadTarget("some-target", false)
				Expect(err).Should(MatchError("A client key may not be declared without defining a client certificate"))
			})
		})
	})

	Describe("FindTeam", func() {
		It("finds the team", func() {
			fakeClient := new(fakes.FakeClient)

			rc.NewTarget(
				"test-target",
				"default-team",
				"http://example.com",
				nil,
				"ca-cert",
				nil,
				"",
				"",
				[]tls.Certificate{},
				true,
				fakeClient,
			).FindTeam("the-team")

			Expect(fakeClient.FindTeamCallCount()).To(Equal(1), "client.FindTeam should be used")
			Expect(fakeClient.FindTeamArgsForCall(0)).To(Equal("the-team"), "FindTeam should pass through team name")
		})
	})
})
