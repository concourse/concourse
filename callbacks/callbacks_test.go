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

	Describe("PUT /builds/:build", func() {
		var build builds.Build
		var turbineBuild TurbineBuilds.Build

		var response *http.Response

		BeforeEach(func() {
			build = builds.Build{
				Name: "42",
			}

			turbineBuild = TurbineBuilds.Build{
				Inputs: []TurbineBuilds.Input{
					{
						Name:    "input-only-resource",
						Type:    "git",
						Source:  TurbineBuilds.Source{"input-source": "some-source"},
						Version: TurbineBuilds.Version{"version": "input-version"},
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "input-meta", Value: "some-value"},
						},
					},
					{
						Name:    "input-and-output-resource",
						Type:    "git",
						Source:  TurbineBuilds.Source{"input-and-output-source": "some-source"},
						Version: TurbineBuilds.Version{"version": "input-and-output-version"},
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "input-and-output-meta", Value: "some-value"},
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(turbineBuild)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("PUT", server.URL+"/builds/42", bytes.NewBuffer(reqPayload))
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

				id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusStarted))
			})

			It("saves the build's inputs", func() {
				Ω(buildDB.SaveBuildInputCallCount()).Should(Equal(2))

				id, input := buildDB.SaveBuildInputArgsForCall(0)
				Ω(id).Should(Equal(42))
				Ω(input).Should(Equal(builds.VersionedResource{
					Name:    "input-only-resource",
					Type:    "git",
					Source:  config.Source{"input-source": "some-source"},
					Version: builds.Version{"version": "input-version"},
					Metadata: []builds.MetadataField{
						{Name: "input-meta", Value: "some-value"},
					},
				}))

				id, input = buildDB.SaveBuildInputArgsForCall(1)
				Ω(id).Should(Equal(42))
				Ω(input).Should(Equal(builds.VersionedResource{
					Name:    "input-and-output-resource",
					Type:    "git",
					Source:  config.Source{"input-and-output-source": "some-source"},
					Version: builds.Version{"version": "input-and-output-version"},
					Metadata: []builds.MetadataField{
						{Name: "input-and-output-meta", Value: "some-value"},
					},
				}))
			})
		})

		Context("with status 'succeeded'", func() {
			BeforeEach(func() {
				turbineBuild.Status = TurbineBuilds.StatusSucceeded

				turbineBuild.Outputs = []TurbineBuilds.Output{
					{
						Name:    "input-and-output-resource",
						Type:    "git",
						Source:  TurbineBuilds.Source{"input-and-output-source": "some-source"},
						Version: TurbineBuilds.Version{"version": "new-input-and-output-version"},
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "input-and-output-meta", Value: "some-value"},
						},
					},
					{
						Name:    "output-only-resource",
						Type:    "git",
						Source:  TurbineBuilds.Source{"output-source": "some-source"},
						Version: TurbineBuilds.Version{"version": "output-version"},
						Metadata: []TurbineBuilds.MetadataField{
							{Name: "output-meta", Value: "some-value"},
						},
					},
				}
			})

			It("updates the build's status", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				Ω(buildDB.SaveBuildStatusCallCount()).Should(Equal(1))

				id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusSucceeded))
			})

			It("saves each input and output version", func() {
				Ω(buildDB.SaveBuildOutputCallCount()).Should(Equal(3))

				savedOutputs := []builds.VersionedResource{}

				id, vr := buildDB.SaveBuildOutputArgsForCall(0)
				Ω(id).Should(Equal(42))
				savedOutputs = append(savedOutputs, vr)

				id, vr = buildDB.SaveBuildOutputArgsForCall(1)
				Ω(id).Should(Equal(42))
				savedOutputs = append(savedOutputs, vr)

				id, vr = buildDB.SaveBuildOutputArgsForCall(2)
				Ω(id).Should(Equal(42))
				savedOutputs = append(savedOutputs, vr)

				Ω(savedOutputs).Should(ContainElement(builds.VersionedResource{
					Name:    "input-only-resource",
					Type:    "git",
					Source:  config.Source{"input-source": "some-source"},
					Version: builds.Version{"version": "input-version"},
					Metadata: []builds.MetadataField{
						{Name: "input-meta", Value: "some-value"},
					},
				}))

				Ω(savedOutputs).Should(ContainElement(builds.VersionedResource{
					Name:    "input-and-output-resource",
					Type:    "git",
					Source:  config.Source{"input-and-output-source": "some-source"},
					Version: builds.Version{"version": "new-input-and-output-version"},
					Metadata: []builds.MetadataField{
						{Name: "input-and-output-meta", Value: "some-value"},
					},
				}))

				Ω(savedOutputs).Should(ContainElement(builds.VersionedResource{
					Name:    "output-only-resource",
					Type:    "git",
					Source:  config.Source{"output-source": "some-source"},
					Version: builds.Version{"version": "output-version"},
					Metadata: []builds.MetadataField{
						{Name: "output-meta", Value: "some-value"},
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

				id, status := buildDB.SaveBuildStatusArgsForCall(0)
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

				id, status := buildDB.SaveBuildStatusArgsForCall(0)
				Ω(id).Should(Equal(42))
				Ω(status).Should(Equal(builds.StatusErrored))
			})
		})
	})
})
