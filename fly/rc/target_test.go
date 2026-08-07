package rc_test

import (
	"crypto/tls"
	"crypto/x509"
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
	var (
		tmpDir string
		flyrc  string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "fly-test")
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
			os.WriteFile(flyrc, []byte(flyrcContents), 0777)
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
				os.WriteFile(flyrc, []byte(flyrcContents), 0777)
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
							CACert:   string(rootCA),
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

				os.WriteFile(flyrc, []byte(flyrcContents), 0777)
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
				ok = expectedCaCertPool.AppendCertsFromPEM(rootCA)
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

				err := os.WriteFile(certPath, clientCert, 0600)

				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(keyPath, clientKey, 0600)
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

				os.WriteFile(flyrc, []byte(flyrcContents), 0777)
			})

			It("loads target with correct transport", func() {
				target, err := rc.LoadTarget("some-target", false)
				Expect(err).NotTo(HaveOccurred())
				transport, ok := target.Client().HTTPClient().Transport.(*oauth2.Transport)
				Expect(ok).To(BeTrue())
				base, ok := (*transport).Base.(*http.Transport)
				Expect(ok).To(BeTrue())

				expectedX509Cert, err := tls.X509KeyPair(clientCert, clientKey)

				Expect((*base).TLSClientConfig).To(Equal(&tls.Config{
					InsecureSkipVerify: false,
					Certificates:       []tls.Certificate{expectedX509Cert},
				}))
			})
		})

		Context("when there is a client certificate path, but no client key path", func() {
			BeforeEach(func() {
				certPath := filepath.Join(userHomeDir(), "client.pem")

				err := os.WriteFile(certPath, clientCert, 0600)
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

				os.WriteFile(flyrc, []byte(flyrcContents), 0777)
			})

			It("warns the user and exits with failure", func() {
				_, err := rc.LoadTarget("some-target", false)
				Expect(err).Should(MatchError("A client certificate may not be declared without defining a client key"))
			})
		})

		Context("when there is a client key path, but no client certificate path", func() {
			BeforeEach(func() {
				keyPath := filepath.Join(userHomeDir(), "client.key")

				err := os.WriteFile(keyPath, []byte(clientKey), 0600)
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

				os.WriteFile(flyrc, []byte(flyrcContents), 0777)
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
