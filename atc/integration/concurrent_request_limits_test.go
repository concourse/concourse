package integration_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent request limits", func() {
	BeforeEach(func() {
		cmd.Database.ConcurrentRequestLimits = map[string]int{
			atc.ListAllJobs: 0,
		}
	})

	It("disables ListAllJobs requests", func() {
		client := login(atcURL, "test", "test")
		httpClient := client.HTTPClient()
		request, _ := http.NewRequest(
			"GET",
			client.URL()+"/api/v1/jobs",
			nil,
		)

		response, _ := httpClient.Do(request)

		Expect(response.StatusCode).To(Equal(http.StatusNotImplemented))
	})
})
