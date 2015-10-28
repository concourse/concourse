package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Volumes", func() {
	Describe("ListVolumes", func() {
		var (
			expectedVolumes []atc.Volume
		)

		BeforeEach(func() {
			expectedURL := "/api/v1/volumes"

			expectedVolumes = []atc.Volume{
				{
					ID:              "myid-1",
					TTLInSeconds:    9,
					WorkerName:      "some-worker",
					ResourceVersion: atc.Version{"some": "version"},
				},
				{
					ID:              "myid-2",
					TTLInSeconds:    10,
					WorkerName:      "some-other-worker",
					ResourceVersion: atc.Version{"some": "other-version"},
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVolumes),
				),
			)
		})

		It("returns all the volumes", func() {
			volumes, err := client.ListVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(Equal(expectedVolumes))
		})
	})
})
