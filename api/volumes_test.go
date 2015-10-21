package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipelines API", func() {
	Describe("GET /api/v1/volumes", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/volumes")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when getting all volumes succeeds", func() {
				BeforeEach(func() {
					volumesDB.GetVolumesReturns([]db.SavedVolumeData{
						{
							ID:        3,
							ExpiresIn: 2 * time.Minute,
							VolumeData: db.VolumeData{
								WorkerName:      "some-worker",
								TTL:             10 * time.Minute,
								Handle:          "some-handle",
								ResourceVersion: atc.Version{"some": "version"},
								ResourceHash:    "some-hash",
							},
						},
						{
							ID:        1,
							ExpiresIn: 23 * time.Hour,
							VolumeData: db.VolumeData{
								WorkerName:      "some-other-worker",
								TTL:             24 * time.Hour,
								Handle:          "some-other-handle",
								ResourceVersion: atc.Version{"some": "other-version"},
								ResourceHash:    "some-other-hash",
							},
						},
					}, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns all volumes", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
						{
							"id": "some-handle",
							"ttl_in_seconds": 120,
							"validity_in_seconds": 600,
							"resource_version": {"some": "version"},
							"worker_name": "some-worker"
						},
						{
							"id": "some-other-handle",
							"ttl_in_seconds": 82800,
							"validity_in_seconds": 86400,
							"resource_version": {"some": "other-version"},
							"worker_name": "some-other-worker"
						}
					]`))
				})
			})

			Context("when getting all builds fails", func() {
				BeforeEach(func() {
					volumesDB.GetVolumesReturns(nil, errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})
})
