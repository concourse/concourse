package topgun_test

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
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

		BeforeEach(func() {
			By("Configuring two ATCs")
			Deploy("deployments/concourse-two-atcs-slow-tracking.yml")
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

		Context("make api request to a different atc by a token from a stopped atc", func() {
			It("request successfully", func() {

				var (
					err       error
					request   *http.Request
					response  *http.Response
					reqHeader http.Header
				)

				By("stopping the first atc")
				stopSession := spawnBosh("stop", atcs[0].Name)
				Eventually(stopSession).Should(gexec.Exit(0))

				token, err := fetchToken(atc0URL, "test", "test")
				Expect(err).ToNot(HaveOccurred())
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
	})
})
