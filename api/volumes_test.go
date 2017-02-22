package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volumes API", func() {

	var fakeWorker *dbngfakes.FakeWorker

	BeforeEach(func() {
		fakeWorker = new(dbngfakes.FakeWorker)
		fakeWorker.NameReturns("some-worker")
	})

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
				userContextReader.GetTeamReturns("some-team", true, true)
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
						someOtherFakeWorker := new(dbngfakes.FakeWorker)
						someOtherFakeWorker.NameReturns("some-other-worker")

						fakeVolumeFactory.GetTeamVolumesStub = func(teamID int) ([]dbng.CreatedVolume, error) {
							if teamID != 1 {
								return []dbng.CreatedVolume{}, nil
							}

							volume1 := new(dbngfakes.FakeCreatedVolume)
							volume1.HandleReturns("some-resource-cache-handle")
							volume1.WorkerReturns(fakeWorker)
							volume1.TypeReturns(dbng.VolumeTypeResource)
							volume1.SizeInBytesReturns(1024)
							volume1.ResourceTypeReturns(&dbng.VolumeResourceType{
								ResourceType: &dbng.VolumeResourceType{
									BaseResourceType: &dbng.WorkerBaseResourceType{
										Name:    "some-base-resource-type",
										Version: "some-base-version",
									},
									Version: atc.Version{"custom": "version"},
								},
								Version: atc.Version{"some": "version"},
							}, nil)
							volume2 := new(dbngfakes.FakeCreatedVolume)
							volume2.HandleReturns("some-import-handle")
							volume2.WorkerReturns(fakeWorker)
							volume2.SizeInBytesReturns(2048)
							volume2.TypeReturns(dbng.VolumeTypeResourceType)
							volume2.BaseResourceTypeReturns(&dbng.WorkerBaseResourceType{
								Name:    "some-base-resource-type",
								Version: "some-base-version",
							}, nil)
							volume3 := new(dbngfakes.FakeCreatedVolume)
							volume3.HandleReturns("some-output-handle")
							volume3.WorkerReturns(someOtherFakeWorker)
							volume3.ContainerHandleReturns("some-container-handle")
							volume3.PathReturns("some-path")
							volume3.ParentHandleReturns("some-parent-handle")
							volume3.SizeInBytesReturns(4096)
							volume3.TypeReturns(dbng.VolumeTypeContainer)
							volume4 := new(dbngfakes.FakeCreatedVolume)
							volume4.HandleReturns("some-cow-handle")
							volume4.WorkerReturns(fakeWorker)
							volume4.ContainerHandleReturns("some-container-handle")
							volume4.PathReturns("some-path")
							volume4.SizeInBytesReturns(8192)
							volume4.TypeReturns(dbng.VolumeTypeContainer)

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
								"type": "resource",
								"size_in_bytes": 1024,
								"container_handle": "",
								"path": "",
								"parent_handle": "",
								"resource_type": {
									"resource_type": {
									  "resource_type": null,
										"base_resource_type": {
											"name": "some-base-resource-type",
											"version": "some-base-version"
										},
										"version": {"custom": "version"}
									},
									"base_resource_type": null,
									"version": {"some": "version"}
								},
								"base_resource_type": null
							},
							{
								"id": "some-import-handle",
								"worker_name": "some-worker",
								"type": "resource-type",
								"size_in_bytes": 2048,
								"container_handle": "",
								"path": "",
								"parent_handle": "",
								"resource_type": null,
								"base_resource_type": {
									"name": "some-base-resource-type",
									"version": "some-base-version"
								}
							},
							{
								"id": "some-output-handle",
								"worker_name": "some-other-worker",
								"type": "container",
								"size_in_bytes": 4096,
								"container_handle": "some-container-handle",
								"path": "some-path",
								"parent_handle": "some-parent-handle",
								"resource_type": null,
								"base_resource_type": null
							},
							{
								"id": "some-cow-handle",
								"worker_name": "some-worker",
								"type": "container",
								"size_in_bytes": 8192,
								"container_handle": "some-container-handle",
								"parent_handle": "",
								"path": "some-path",
								"resource_type": null,
								"base_resource_type": null
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
