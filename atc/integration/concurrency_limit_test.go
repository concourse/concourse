package integration_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Concurrent request limits", func() {
	BeforeEach(func() {
		cmd.ConcurrentRequestLimits = []wrappa.ConcurrentRequestLimitFlag{
			wrappa.ConcurrentRequestLimitFlag{
				Action: atc.ListAllJobs,
				Limit:  0,
			},
		}
	})

	It("limits concurrent ListAllJobs requests", func() {
		client := login(atcURL, "test", "test")
		httpClient := client.HTTPClient()
		request, _ := http.NewRequest(
			"GET",
			client.URL()+"/api/v1/jobs",
			nil,
		)
		response, _ := httpClient.Do(request)
		Expect(response.StatusCode).To(Equal(http.StatusTooManyRequests))
	})
})
