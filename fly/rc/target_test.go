package rc_test

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/concourse/fly/rc"
	"golang.org/x/oauth2"
	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
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
				}))
			})
		})

		Context("when there is ca-cert", func() {
			type targetDetailsYAML struct {
				Targets map[rc.TargetName]rc.TargetProps
			}

			BeforeEach(func() {
				flyrcConfig := targetDetailsYAML{
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

				Expect((*base).TLSClientConfig).To(Equal(&tls.Config{
					InsecureSkipVerify: false,
					RootCAs:            expectedCaCertPool,
				}))
			})
		})
	})
})
