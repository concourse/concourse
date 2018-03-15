package concourse_test

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
			expectedURL := "/api/v1/teams/some-team/volumes"

			expectedVolumes = []atc.Volume{
				{
					ID:              "myid-1",
					WorkerName:      "some-worker",
					Type:            "some-type",
					ContainerHandle: "some-container-handle",
				},
				{
					ID:              "myid-2",
					WorkerName:      "some-other-worker",
					Type:            "some-other-type",
					ContainerHandle: "some-other-container-handle",
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
			volumes, err := team.ListVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(Equal(expectedVolumes))
		})
	})
})
