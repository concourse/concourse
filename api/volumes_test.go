package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volumes API", func() {
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
				userContextReader.GetTeamReturns("some-team", 42, true, true)
			})

			Context("when identifying the team errors", func() {
				BeforeEach(func() {
					teamDB.GetTeamReturns(db.SavedTeam{}, true, errors.New("disaster"))
				})

				It("returns 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			// TODO this test will no longer be meaningful once DBNG takes fully over and we lose the teamDB concept.
			Context("when the team db gave back a missing team", func() {
				BeforeEach(func() {
					teamDB.GetTeamReturns(db.SavedTeam{}, false, nil)
				})

				It("returns 401 Not Authorized", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("when identifying the team succeeds", func() {
				BeforeEach(func() {
					teamDB.GetTeamReturns(db.SavedTeam{
						ID: 1,
					}, true, nil)
				})

				It("asks the factory for the volumes", func() {
					Expect(fakeVolumeFactory.GetTeamVolumesCallCount()).To(Equal(1))
				})

				Context("when getting all volumes succeeds", func() {
					BeforeEach(func() {
						fakeVolumeFactory.GetTeamVolumesStub = func(teamID int) ([]dbng.CreatedVolume, error) {
							if teamID != 1 {
								return []dbng.CreatedVolume{}, nil
							}

							volume1 := new(dbngfakes.FakeCreatedVolume)
							volume1.HandleReturns("some-resource-cache-handle")
							volume1.WorkerReturns(&dbng.Worker{Name: "some-worker"})
							volume1.SizeInBytesReturns(1024)
							volume2 := new(dbngfakes.FakeCreatedVolume)
							volume2.HandleReturns("some-import-handle")
							volume2.WorkerReturns(&dbng.Worker{Name: "some-worker"})
							volume2.SizeInBytesReturns(2048)
							volume3 := new(dbngfakes.FakeCreatedVolume)
							volume3.HandleReturns("some-output-handle")
							volume3.WorkerReturns(&dbng.Worker{Name: "some-other-worker"})
							volume3.SizeInBytesReturns(4096)
							volume4 := new(dbngfakes.FakeCreatedVolume)
							volume4.HandleReturns("some-cow-handle")
							volume4.WorkerReturns(&dbng.Worker{Name: "some-worker"})
							volume4.SizeInBytesReturns(8192)

							return []dbng.CreatedVolume{
								volume1,
								volume2,
								volume3,
								volume4,
							}, nil
						}
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns all volumes", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"id": "some-resource-cache-handle",
								"worker_name": "some-worker",
								"type": "",
								"identifier": "",
								"size_in_bytes": 1024
							},
							{
								"id": "some-import-handle",
								"worker_name": "some-worker",
								"type": "",
								"identifier": "",
								"size_in_bytes": 2048
							},
							{
								"id": "some-output-handle",
								"worker_name": "some-other-worker",
								"type": "",
								"identifier": "",
								"size_in_bytes": 4096
							},
							{
								"id": "some-cow-handle",
								"worker_name": "some-worker",
								"type": "",
								"identifier": "",
								"size_in_bytes": 8192
							}
						]`,
						))
					})
				})

				Context("when getting all volumes fails", func() {
					BeforeEach(func() {
						fakeVolumeFactory.GetTeamVolumesReturns([]dbng.CreatedVolume{}, errors.New("oh no!"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})
})
