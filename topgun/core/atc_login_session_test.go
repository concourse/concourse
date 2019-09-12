package topgun_test

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"golang.org/x/oauth2"

	_ "github.com/lib/pq"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multiple ATCs Login Session Test", func() {
	Context("with two atcs available", func() {
		var atcs []boshInstance
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

		Describe("using database storage for dex", func() {
			It("uses the same client for multiple ATCs", func() {
				var numClient int
				err := psql.Select("COUNT(*)").From("client").RunWith(dbConn).QueryRow().Scan(&numClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(numClient).To(Equal(1))
			})
		})

		Context("make api request to a different atc by a token from a stopped atc", func() {
			var token *oauth2.Token

			BeforeEach(func() {
				var err error
				token, err = FetchToken(atc0URL, atcUsername, atcPassword)
				Expect(err).ToNot(HaveOccurred())

				By("stopping the first atc")
				bosh("stop", atcs[0].Name)
			})

			AfterEach(func() {
				bosh("start", atcs[0].Name)
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
				fly.Login(atcUsername, atcPassword, atc1URL)
				fly.Login(atcUsername, atcPassword, atc0URL)

				By("deploying a second time (with a different token signing key)")
				deploy()

				fly.Login(atcUsername, atcPassword, atc0URL)
				fly.Login(atcUsername, atcPassword, atc1URL)
			})
		})
	})
})
