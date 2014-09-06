package callbacks_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	TurbineBuilds "github.com/concourse/turbine/api/builds"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/callbacks"
	"github.com/concourse/atc/callbacks/handler/fakes"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/logfanout"
	logfakes "github.com/concourse/atc/logfanout/fakes"
)

var _ = Describe("Callbacks", func() {
	var buildDB *fakes.FakeBuildDB
	var logDB *logfakes.FakeLogDB

	var server *httptest.Server
	var client *http.Client

	var tracker *logfanout.Tracker

	BeforeEach(func() {
		buildDB = new(fakes.FakeBuildDB)
		logDB = new(logfakes.FakeLogDB)

		tracker = logfanout.NewTracker(logDB)

		handler, err := callbacks.NewHandler(lagertest.NewTestLogger("callbacks"), buildDB, tracker)
		Ω(err).ShouldNot(HaveOccurred())

		server = httptest.NewServer(handler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("PUT /builds/:job/:build", func() {
		var build builds.Build
		var turbineBuild TurbineBuilds.Build

		var response *http.Response

		version1 := TurbineBuilds.Version{"ver": "1"}
		version2 := TurbineBuilds.Version{"ver": "2"}

		BeforeEach(func() {
			build = builds.Build{
				ID: 42,
			}

			buildDB.GetBuildReturns(build, nil)

			turbineBuild = TurbineBuilds.Build{
				Inputs: []TurbineBuilds.Input{
					{
						Name:    "some-input",
						Type:    "git",
						Version: version1,
					},
					{
						Name:    "some-other-input",
						Type:    "git",
						Version: version2,
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "meta1", Value: "value1"},
							{Name: "meta2", Value: "value2"},
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(turbineBuild)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("PUT", server.URL+"/builds/some-job/42", bytes.NewBuffer(reqPayload))
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("with status 'started'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusStarted
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusStarted))
			})

			It("saves the build's inputs", func() {
				Ω(buildDB.SaveBuildInputCallCount()).Should(Equal(2))

				job, id, input := buildDB.SaveBuildInputArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(input).Should(Equal(builds.VersionedResource{
					Name:     "some-input",
					Type:     "git",
					Version:  builds.Version(version1),
					Metadata: []builds.MetadataField{},
				}))

				job, id, input = buildDB.SaveBuildInputArgsForCall(1)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(input).Should(Equal(builds.VersionedResource{
					Name:    "some-other-input",
					Type:    "git",
					Version: builds.Version(version2),
					Metadata: []builds.MetadataField{
						{Name: "meta1", Value: "value1"},
						{Name: "meta2", Value: "value2"},
					},
				}))
			})
		})

		Context("with status 'succeeded'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusSucceeded

				turbineBuild.Outputs = []TurbineBuilds.Output{
					{
						Name: "some-output",
						Type: "git",

						Source:  TurbineBuilds.Source{"source": "1"},
						Version: TurbineBuilds.Version{"ver": "123"},
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "meta1", Value: "value1"},
						},
					},
					{
						Name: "some-other-output",
						Type: "git",

						Source:  TurbineBuilds.Source{"source": "2"},
						Version: TurbineBuilds.Version{"ver": "456"},
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "meta2", Value: "value2"},
						},
					},
				}
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusSucceeded))
			})

			It("saves each output version", func() {
				Ω(buildDB.SaveBuildOutputCallCount()).Should(Equal(2))

				job, id, vr := buildDB.SaveBuildOutputArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(vr).Should(Equal(builds.VersionedResource{
					Name:    "some-output",
					Type:    "git",
					Source:  config.Source{"source": "1"},
					Version: builds.Version{"ver": "123"},
					Metadata: []builds.MetadataField{
						{Name: "meta1", Value: "value1"},
					},
				}))

				job, id, vr = buildDB.SaveBuildOutputArgsForCall(1)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(vr).Should(Equal(builds.VersionedResource{
					Name:    "some-other-output",
					Type:    "git",
					Source:  config.Source{"source": "2"},
					Version: builds.Version{"ver": "456"},
					Metadata: []builds.MetadataField{
						{Name: "meta2", Value: "value2"},
					},
				}))
			})
		})

		Context("with status 'failed'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusFailed
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusFailed))
			})
		})

		Context("with status 'errored'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusErrored
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(job).Should(Equal("some-job"))
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusErrored))
			})

			Context("when the build has been aborted", func() {
				BeforeEach(func() {
					build.Status = builds.StatusAborted
					buildDB.GetBuildReturns(build, nil)
				})

				It("does not update the build's status", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))

					Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

					job, id, status := buildDB.SaveBuildStatusArgsForCall(0)
					Ω(job).Should(Equal("some-job"))
					Ω(id).Should(Equal(42))
					Ω(status).Should(Equal(builds.StatusAborted))
				})
			})
		})
	})
})
