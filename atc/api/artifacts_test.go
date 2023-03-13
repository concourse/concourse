package api_test

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	. "github.com/concourse/concourse/atc/testhelpers"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/worker/baggageclaim"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArtifactRepository API", func() {
	Describe("POST /api/v1/teams/:team_name/artifacts", func() {
		var request *http.Request
		var response *http.Response

		var tarContents runtimetest.VolumeContent

		BeforeEach(func() {
			fakeAccess.IsAuthenticatedReturns(true)
		})

		JustBeforeEach(func() {
			body, err := tarContents.StreamOut(context.Background(), ".", baggageclaim.GzipEncoding)
			Expect(err).NotTo(HaveOccurred())

			request, err = http.NewRequest("POST", server.URL+"/api/v1/teams/some-team/artifacts", body)
			Expect(err).NotTo(HaveOccurred())

			request.Header.Set("Content-Type", "application/json")

			q := url.Values{}
			q.Add("platform", "some-platform")
			request.URL.RawQuery = q.Encode()

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
			})

			It("returns 403 Forbidden", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when authorized", func() {
			var volume *runtimetest.Volume
			var workerArtifact *dbfakes.FakeWorkerArtifact

			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)

				tarContents = runtimetest.VolumeContent{
					"some/file": {Data: []byte("some contents")},
				}

				volume = runtimetest.NewVolume("some-artifact")

				workerArtifact = new(dbfakes.FakeWorkerArtifact)
				workerArtifact.IDReturns(0)
				workerArtifact.CreatedAtReturns(time.Unix(42, 0))

				fakeWorkerPool.CreateVolumeForArtifactReturns(volume, workerArtifact, nil)
			})

			It("creates the volume", func() {
				Expect(fakeWorkerPool.CreateVolumeForArtifactCallCount()).To(Equal(1))

				_, workerSpec := fakeWorkerPool.CreateVolumeForArtifactArgsForCall(0)
				Expect(workerSpec).To(Equal(worker.Spec{
					TeamID:   734,
					Platform: "some-platform",
				}))
			})

			It("streams into the volume", func() {
				Expect(volume.Content).To(Equal(tarContents))
			})

			It("returns 201 Created", func() {
				Expect(response.StatusCode).To(Equal(http.StatusCreated))
			})

			It("returns Content-Type 'application/json'", func() {
				expectedHeaderEntries := map[string]string{
					"Content-Type": "application/json",
				}
				Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
			})

			It("returns the artifact record", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`{
					"id": 0,
					"name": "",
					"build_id": 0,
					"created_at": 42
				}`))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/artifacts/:artifact_id", func() {
		var response *http.Response

		BeforeEach(func() {
			fakeAccess.IsAuthenticatedReturns(true)
		})

		JustBeforeEach(func() {
			var err error
			response, err = http.Get(server.URL + "/api/v1/teams/some-team/artifacts/18")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
			})

			It("returns 403 Forbidden", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)
			})

			It("uses the artifactID to fetch the db volume record", func() {
				Expect(dbTeam.FindVolumeForWorkerArtifactCallCount()).To(Equal(1))

				artifactID := dbTeam.FindVolumeForWorkerArtifactArgsForCall(0)
				Expect(artifactID).To(Equal(18))
			})

			Context("when retrieving db artifact volume fails", func() {
				BeforeEach(func() {
					dbTeam.FindVolumeForWorkerArtifactReturns(nil, false, errors.New("nope"))
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the db artifact volume is not found", func() {
				BeforeEach(func() {
					dbTeam.FindVolumeForWorkerArtifactReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when the db artifact volume is found", func() {
				var fakeVolume *dbfakes.FakeCreatedVolume

				BeforeEach(func() {
					fakeVolume = new(dbfakes.FakeCreatedVolume)
					fakeVolume.HandleReturns("some-handle")

					dbTeam.FindVolumeForWorkerArtifactReturns(fakeVolume, true, nil)
				})

				It("uses the volume handle to lookup the worker volume", func() {
					Expect(fakeWorkerPool.LocateVolumeCallCount()).To(Equal(1))

					_, teamID, handle := fakeWorkerPool.LocateVolumeArgsForCall(0)
					Expect(handle).To(Equal("some-handle"))
					Expect(teamID).To(Equal(734))
				})

				Context("when the worker client errors", func() {
					BeforeEach(func() {
						fakeWorkerPool.LocateVolumeReturns(nil, nil, false, errors.New("nope"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the worker client can't find the volume", func() {
					BeforeEach(func() {
						fakeWorkerPool.LocateVolumeReturns(nil, nil, false, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when the worker client finds the volume", func() {
					var volume *runtimetest.Volume

					BeforeEach(func() {
						volume = runtimetest.NewVolume("volume").
							WithContent(runtimetest.VolumeContent{
								"some/file": {Data: []byte("some content")},
							})

						fakeWorkerPool.LocateVolumeReturns(volume, runtimetest.NewWorker("worker"), true, nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/octet-stream'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/octet-stream",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("streams out the contents of the volume from the root path", func() {
						tarStream := runtimetest.VolumeContent{}

						err := tarStream.StreamIn(context.Background(), ".", baggageclaim.GzipEncoding, response.Body)
						Expect(err).ToNot(HaveOccurred())

						Expect(tarStream).To(Equal(volume.Content))
					})
				})
			})
		})
	})
})
