package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"
	. "github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Client", func() {
	var (
		api      string
		username string
		password string
		cert     string
		insecure bool
	)

	BeforeEach(func() {
		api = "f"
		username = ""
		password = ""
		cert = ""
		insecure = false
	})

	Describe("#NewClient", func() {
		It("Returns back an ATC Client", func() {
			target := rc.NewTarget(api, username, password, cert, insecure)
			client, err := NewClient(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).To(BeAssignableToTypeOf(AtcClient{}))
		})

		It("Errors when passed target props with an invalid url", func() {
			target := rc.NewTarget("", username, password, cert, insecure)
			_, err := NewClient(target)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("API is blank"))
		})
	})

	Describe("All Requests", func() {
	})

	Describe("#MakeRequest", func() {
		var (
			atcServer *ghttp.Server
			client    Client
			config    atc.Config
		)

		BeforeEach(func() {
			var err error
			atcServer = ghttp.NewServer()
			config = atc.Config{}

			client, err = NewClient(
				rc.NewTarget(atcServer.URL(), "", "", "", false),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			atcServer.Close()
		})

		It("Makes a request to the given route", func() {
			expectedURL := "/api/v1/builds/foo"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
				),
			)
			var build atc.Build
			err := client.MakeRequest(&build, atc.GetBuild, map[string]string{"build_id": "foo"}, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
		})

		It("Makes a request with the given parameters to the given route", func() {
			expectedURL := "/api/v1/containers"
			expectedResponse := []atc.Container{
				{
					ID:           "first-container",
					PipelineName: "my-special-pipeline",
					Type:         "check",
					Name:         "bob",
					BuildID:      1,
					WorkerName:   "abc",
				},
				{
					ID:           "second-container",
					PipelineName: "my-special-pipeline",
					Type:         "check",
					Name:         "alice",
					BuildID:      1,
					WorkerName:   "def",
				},
			}
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, "type=check"),
					ghttp.RespondWithJSONEncoded(200, expectedResponse, http.Header{atc.ConfigVersionHeader: {"42"}}),
				),
			)
			var containers []atc.Container
			err := client.MakeRequest(&containers, atc.ListContainers, nil, map[string]string{"type": "check"})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(atcServer.ReceivedRequests())).To(Equal(1))
			Expect(containers).To(Equal(expectedResponse))
		})
	})
})
