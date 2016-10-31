package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Workers", func() {
	Describe("ListWorkers", func() {
		var (
			expectedWorkers []atc.Worker
		)

		BeforeEach(func() {
			expectedURL := "/api/v1/workers"

			expectedWorkers = []atc.Worker{
				{
					Name:     "myname-1",
					Platform: "linux",
				},
				{
					Name:     "myname-2",
					Platform: "windows",
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedWorkers),
				),
			)
		})

		It("returns all the workers", func() {
			workers, err := client.ListWorkers()
			Expect(err).NotTo(HaveOccurred())
			Expect(workers).To(Equal(expectedWorkers))
		})
	})

	Describe("SaveWorker", func() {
		var worker atc.Worker
		BeforeEach(func() {
			worker = atc.Worker{
				Name:     "worker-name",
				Tags:     []string{"some-tag"},
				Platform: "linux",
			}
			expectedURL := "/api/v1/workers"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, worker),
				),
			)
		})

		It("saves the worker", func() {
			savedWorker, err := client.SaveWorker(worker, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(*savedWorker).To(Equal(worker))
		})
	})
})
