package topgun_test

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"golang.org/x/oauth2"

	_ "github.com/lib/pq"

	. "github.com/concourse/concourse/topgun"
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multiple ATCs Login Session Test", func() {
	Context("with two atcs available", func() {
		var atcs []BoshInstance
		var atc0URL string
		var atc1URL string

		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/web-instances.yml",
				"-v", "web_instances=2",
			)

			atcs = JobInstances("web")
			atc0URL = "http://" + atcs[0].IP + ":8080"
			atc1URL = "http://" + atcs[1].IP + ":8080"
		})

		Context("make api request to a different atc by a token from a stopped atc", func() {
			var token *oauth2.Token

			BeforeEach(func() {
				var err error
				token, err = FetchToken(atc0URL, AtcUsername, AtcPassword)
				Expect(err).ToNot(HaveOccurred())

				By("stopping the first atc")
				Bosh("stop", atcs[0].Name)
			})

			AfterEach(func() {
				Bosh("start", atcs[0].Name)
			})

			It("request successfully", func() {
				client := &http.Client{}
				reqHeader := http.Header{}
				reqHeader.Set("Authorization", "Bearer "+token.AccessToken)

				By("make request with the token to second atc")
				request, err := http.NewRequest("GET", atc1URL+"/api/v1/workers", nil)
				request.Header = reqHeader
				Expect(err).NotTo(HaveOccurred())

				response, err := client.Do(request)
				Expect(err).NotTo(HaveOccurred())

				var workers []atc.Worker
				err = json.NewDecoder(response.Body).Decode(&workers)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when two atcs have the same external url (dex redirect uri is the same)", func() {
			deploy := func() {
				Deploy(
					"deployments/concourse.yml",
					"-o", "operations/web-instances.yml",
					"-v", "web_instances=2",
					"-o", "operations/external-url.yml",
					"-v", "external_url="+atc0URL,
				)
			}

			BeforeEach(deploy)

			It("should be able to login to both ATCs", func() {
				Fly.Login(AtcUsername, AtcPassword, atc1URL)
				Fly.Login(AtcUsername, AtcPassword, atc0URL)

				By("deploying a second time (with a different token signing key)")
				deploy()

				Fly.Login(AtcUsername, AtcPassword, atc0URL)
				Fly.Login(AtcUsername, AtcPassword, atc1URL)
			})
		})
	})
})
