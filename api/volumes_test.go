package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volumes API", func() {

	var fakeWorker *dbfakes.FakeWorker

	BeforeEach(func() {
		fakeWorker = new(dbfakes.FakeWorker)
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
				jwtValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				jwtValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", true, true)
			})

			Context("when identifying the team succeeds", func() {
				BeforeEach(func() {
					dbTeam.IDReturns(1)
				})

				It("asks the factory for the volumes", func() {
					Expect(fakeVolumeFactory.GetTeamVolumesCallCount()).To(Equal(1))
				})

				Context("when getting all volumes succeeds", func() {
					BeforeEach(func() {
						someOtherFakeWorker := new(dbfakes.FakeWorker)
						someOtherFakeWorker.NameReturns("some-other-worker")

						fakeVolumeFactory.GetTeamVolumesStub = func(teamID int) ([]db.CreatedVolume, error) {
							if teamID != 1 {
								return []db.CreatedVolume{}, nil
							}

							volume1 := new(dbfakes.FakeCreatedVolume)
							volume1.HandleReturns("some-resource-cache-handle")
							volume1.WorkerNameReturns(fakeWorker.Name())
							volume1.TypeReturns(db.VolumeTypeResource)
							volume1.ResourceTypeReturns(&db.VolumeResourceType{
								ResourceType: &db.VolumeResourceType{
									WorkerBaseResourceType: &db.UsedWorkerBaseResourceType{
										Name:    "some-base-resource-type",
										Version: "some-base-version",
									},
									Version: atc.Version{"custom": "version"},
								},
								Version: atc.Version{"some": "version"},
							}, nil)
							volume2 := new(dbfakes.FakeCreatedVolume)
							volume2.HandleReturns("some-import-handle")
							volume2.WorkerNameReturns(fakeWorker.Name())
							volume2.TypeReturns(db.VolumeTypeResourceType)
							volume2.BaseResourceTypeReturns(&db.UsedWorkerBaseResourceType{
								Name:    "some-base-resource-type",
								Version: "some-base-version",
							}, nil)
							volume3 := new(dbfakes.FakeCreatedVolume)
							volume3.HandleReturns("some-output-handle")
							volume3.WorkerNameReturns(someOtherFakeWorker.Name())
							volume3.ContainerHandleReturns("some-container-handle")
							volume3.PathReturns("some-path")
							volume3.ParentHandleReturns("some-parent-handle")
							volume3.TypeReturns(db.VolumeTypeContainer)
							volume4 := new(dbfakes.FakeCreatedVolume)
							volume4.HandleReturns("some-cow-handle")
							volume4.WorkerNameReturns(fakeWorker.Name())
							volume4.ContainerHandleReturns("some-container-handle")
							volume4.PathReturns("some-path")
							volume4.TypeReturns(db.VolumeTypeContainer)
							volume5 := new(dbfakes.FakeCreatedVolume)
							volume5.HandleReturns("some-task-cache-handle")
							volume5.WorkerNameReturns(fakeWorker.Name())
							volume5.TypeReturns(db.VolumeTypeTaskCache)
							volume5.TaskIdentifierReturns("some-pipeline", "some-job", "some-task", nil)
							return []db.CreatedVolume{
								volume1,
								volume2,
								volume3,
								volume4,
								volume5,
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
								"base_resource_type": null,
								"pipeline_name": "",
								"job_name": "",
								"step_name": ""
							},
							{
								"id": "some-import-handle",
								"worker_name": "some-worker",
								"type": "resource-type",
								"container_handle": "",
								"path": "",
								"parent_handle": "",
								"resource_type": null,
								"base_resource_type": {
									"name": "some-base-resource-type",
									"version": "some-base-version"
								},
								"pipeline_name": "",
								"job_name": "",
								"step_name": ""
							},
							{
								"id": "some-output-handle",
								"worker_name": "some-other-worker",
								"type": "container",
								"container_handle": "some-container-handle",
								"path": "some-path",
								"parent_handle": "some-parent-handle",
								"resource_type": null,
								"base_resource_type": null,
								"pipeline_name": "",
								"job_name": "",
								"step_name": ""
							},
							{
								"id": "some-cow-handle",
								"worker_name": "some-worker",
								"type": "container",
								"container_handle": "some-container-handle",
								"parent_handle": "",
								"path": "some-path",
								"resource_type": null,
								"base_resource_type": null,
								"pipeline_name": "",
								"job_name": "",
								"step_name": ""
							},
							{
								"id": "some-task-cache-handle",
								"worker_name": "some-worker",
								"type": "task-cache",
								"container_handle": "",
								"parent_handle": "",
								"path": "",
								"resource_type": null,
								"base_resource_type": null,
								"pipeline_name": "some-pipeline",
								"job_name": "some-job",
								"step_name": "some-task"
							}
						]`,
						))
					})
				})

				Context("when getting all volumes fails", func() {
					BeforeEach(func() {
						fakeVolumeFactory.GetTeamVolumesReturns([]db.CreatedVolume{}, errors.New("oh no!"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when a volume is deleted during the request", func() {
					BeforeEach(func() {
						someOtherFakeWorker := new(dbfakes.FakeWorker)
						someOtherFakeWorker.NameReturns("some-other-worker")

						fakeVolumeFactory.GetTeamVolumesStub = func(teamID int) ([]db.CreatedVolume, error) {
							volume1 := new(dbfakes.FakeCreatedVolume)
							volume1.ResourceTypeReturns(nil, errors.New("Something"))

							volume2 := new(dbfakes.FakeCreatedVolume)
							volume2.HandleReturns("some-import-handle")
							volume2.WorkerNameReturns(fakeWorker.Name())
							volume2.TypeReturns(db.VolumeTypeResourceType)
							volume2.BaseResourceTypeReturns(&db.UsedWorkerBaseResourceType{
								Name:    "some-base-resource-type",
								Version: "some-base-version",
							}, nil)
							return []db.CreatedVolume{
								volume1,
								volume2,
							}, nil
						}
					})

					It("returns a partial list of volumes", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"id": "some-import-handle",
								"worker_name": "some-worker",
								"type": "resource-type",
								"container_handle": "",
								"path": "",
								"parent_handle": "",
								"resource_type": null,
								"base_resource_type": {
									"name": "some-base-resource-type",
									"version": "some-base-version"
								},
								"pipeline_name": "",
								"job_name": "",
								"step_name": ""
							}]`))
					})
					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})
			})
		})
	})
})
