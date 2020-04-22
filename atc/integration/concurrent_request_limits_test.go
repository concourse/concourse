package integration_test

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Concurrent request limits", func() {
	var (
		atcProcess ifrit.Process
		atcURL     string
	)

	BeforeEach(func() {
		cmd.ConcurrentRequestLimits = map[wrappa.LimitedRoute]int{
			wrappa.LimitedRoute(atc.ListAllJobs): 0,
		}

		atcURL = fmt.Sprintf("http://localhost:%v", cmd.BindPort)

		runner, err := cmd.Runner([]string{})
		Expect(err).NotTo(HaveOccurred())

		atcProcess = ifrit.Invoke(runner)

		Eventually(func() error {
			_, err := http.Get(atcURL + "/api/v1/info")
			return err
		}, 20*time.Second).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		atcProcess.Signal(os.Interrupt)
		<-atcProcess.Wait()
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
