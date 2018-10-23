package topgun_test

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Multiple ATCs Login Session Test", func() {
	Context("with two atcs available", func() {
		var atcs []boshInstance
		var atc0URL string
		var atc1URL string
		var client *http.Client
		var manifestFile string

		JustBeforeEach(func() {
			By("Configuring two ATCs")
			Deploy(manifestFile)
			waitForRunningWorker()

			atcs = JobInstances("atc")
			atc0URL = "http://" + atcs[0].IP + ":8080"
			atc1URL = "http://" + atcs[1].IP + ":8080"
		})

		AfterEach(func() {
			restartSession := spawnBosh("start", atcs[0].Name)
			<-restartSession.Exited
			Eventually(restartSession).Should(gexec.Exit(0))
		})

		Context("Using database storage for dex", func() {
			BeforeEach(func() {
				manifestFile = "deployments/concourse-two-atcs-slow-tracking.yml"
			})

			It("uses the same client for multiple ATCs", func() {
				var numClient int
				err := psql.Select("COUNT(*)").From("client").RunWith(dbConn).QueryRow().Scan(&numClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(numClient).To(Equal(1))
			})
		})

		Context("make api request to a different atc by a token from a stopped atc", func() {
			BeforeEach(func() {
				manifestFile = "deployments/concourse-two-atcs-slow-tracking.yml"
			})

			It("request successfully", func() {
				var (
					err       error
					request   *http.Request
					response  *http.Response
					reqHeader http.Header
				)

				client = &http.Client{}
				token, err := fetchToken(atc0URL, "some-user", "password")
				Expect(err).ToNot(HaveOccurred())

				By("stopping the first atc")
				stopSession := spawnBosh("stop", atcs[0].Name)
				Eventually(stopSession).Should(gexec.Exit(0))

				reqHeader = http.Header{}
				reqHeader.Set("Authorization", "Bearer "+token.AccessToken)

				By("make request with the token to second atc")
				request, err = http.NewRequest("GET", atc1URL+"/api/v1/workers", nil)
				request.Header = reqHeader
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())

				var workers []atc.Worker
				err = json.NewDecoder(response.Body).Decode(&workers)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when two atcs have the same external url (dex redirect uri is the same)", func() {
			BeforeEach(func() {
				manifestFile = "deployments/concourse-two-atcs-with-same-redirect-uri.yml"
			})

			It("should be able to login to both ATCs", func() {
				Eventually(func() *gexec.Session {
					return flyLogin("-c", atc0URL).Wait()
				}, 2*time.Minute).Should(gexec.Exit(0))

				Eventually(func() *gexec.Session {
					return flyLogin("-c", atc1URL).Wait()
				}, 2*time.Minute).Should(gexec.Exit(0))

				By("Deploying a second time (with a different token signing key")
				Deploy(manifestFile)

				Eventually(func() *gexec.Session {
					return flyLogin("-c", atc0URL).Wait()
				}, 2*time.Minute).Should(gexec.Exit(0))

				Eventually(func() *gexec.Session {
					return flyLogin("-c", atc1URL).Wait()
				}, 2*time.Minute).Should(gexec.Exit(0))
			})
		})
	})
})
