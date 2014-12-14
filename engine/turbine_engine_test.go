package engine_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/turbine"
	"github.com/concourse/turbine/event"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TurbineEngine", func() {
	var (
		fakeDB *fakes.FakeEngineDB

		engine Engine

		turbineServer *ghttp.Server
	)

	BeforeEach(func() {
		fakeDB = new(fakes.FakeEngineDB)

		turbineServer = ghttp.NewServer()

		engine = NewTurbine(
			rata.NewRequestGenerator(turbineServer.URL(), turbine.Routes),
			fakeDB,
		)
	})

	AfterEach(func() {
		turbineServer.Close()
	})

	Describe("Name", func() {
		It("returns 'turbine'", func() {
			Ω(engine.Name()).Should(Equal("turbine"))
		})
	})

	Describe("CreateBuild", func() {
		var (
			build        db.Build
			turbineBuild turbine.Build

			createdBuild Build
			createErr    error
		)

		BeforeEach(func() {
			build = db.Build{
				ID: 1,
			}

			turbineBuild = turbine.Build{
				Config: turbine.Config{
					Image: "some-image",

					Params: map[string]string{
						"FOO": "1",
						"BAR": "2",
					},

					Run: turbine.RunConfig{
						Path: "some-script",
						Args: []string{"arg1", "arg2"},
					},
				},
			}
		})

		JustBeforeEach(func() {
			createdBuild, createErr = engine.CreateBuild(build, turbineBuild)
		})

		successfulBuildStart := func(build turbine.Build) http.HandlerFunc {
			createdBuild := build
			createdBuild.Guid = "some-build-guid"

			return ghttp.CombineHandlers(
				ghttp.VerifyJSONRepresenting(build),
				func(w http.ResponseWriter, r *http.Request) {
					w.Header().Add("X-Turbine-Endpoint", turbineServer.URL())
				},
				ghttp.RespondWithJSONEncoded(201, createdBuild),
			)
		}

		Context("when the turbine server successfully executes", func() {
			BeforeEach(func() {
				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						successfulBuildStart(turbineBuild),
					),
				)
			})

			It("succeeds", func() {
				Ω(createErr).ShouldNot(HaveOccurred())
			})

			It("returns a build with the correct metadata", func() {
				var metadata TurbineMetadata
				err := json.Unmarshal([]byte(createdBuild.Metadata()), &metadata)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(metadata.Guid).Should(Equal("some-build-guid"))
				Ω(metadata.Endpoint).Should(Equal(turbineServer.URL()))
			})
		})

		Context("when the turbine server is unreachable", func() {
			BeforeEach(func() {
				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						func(w http.ResponseWriter, r *http.Request) {
							turbineServer.HTTPTestServer.CloseClientConnections()
						},
					),
				)
			})

			It("returns an error", func() {
				Ω(createErr).Should(HaveOccurred())
			})
		})

		Context("when the turbine server returns non-201", func() {
			BeforeEach(func() {
				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						ghttp.RespondWith(400, ""),
					),
				)
			})

			It("returns an error", func() {
				Ω(createErr).Should(HaveOccurred())
			})
		})
	})

	Describe("LookupBuild", func() {
		var (
			buildModel db.Build

			build     Build
			lookupErr error
		)

		BeforeEach(func() {
			buildModel = db.Build{
				ID:     1,
				Engine: engine.Name(),
			}
		})

		JustBeforeEach(func() {
			build, lookupErr = engine.LookupBuild(buildModel)
		})

		Context("when the build has Turbine metadata", func() {
			BeforeEach(func() {
				buildModel.EngineMetadata = `{"guid":"abc","endpoint":"http://example.com"}`
			})

			It("does not error", func() {
				Ω(lookupErr).ShouldNot(HaveOccurred())
			})

			It("returns a Build", func() {
				Ω(build).ShouldNot(BeNil())
			})
		})

		Context("when the build's metadata is missing guid", func() {
			BeforeEach(func() {
				buildModel.EngineMetadata = ``
			})

			It("returns an error", func() {
				Ω(lookupErr).Should(HaveOccurred())
			})
		})

		Context("when the build's metadata is missing endpoint", func() {
			BeforeEach(func() {
				buildModel.EngineMetadata = ``
			})

			It("returns an error", func() {
				Ω(lookupErr).Should(HaveOccurred())
			})
		})

		Context("when the build's metadata is blank", func() {
			BeforeEach(func() {
				buildModel.EngineMetadata = ``
			})

			It("returns an error", func() {
				Ω(lookupErr).Should(HaveOccurred())
			})
		})
	})

	Describe("Builds", func() {
		var (
			buildEndpoint *ghttp.Server

			build Build
		)

		BeforeEach(func() {
			buildEndpoint = ghttp.NewServer()
		})

		AfterEach(func() {
			buildEndpoint.Close()
		})

		Describe("Abort", func() {
			var abortErr error

			BeforeEach(func() {
				metadata := TurbineMetadata{
					Guid:     "some-guid",
					Endpoint: buildEndpoint.URL(),
				}

				metadataPayload, err := json.Marshal(metadata)
				Ω(err).ShouldNot(HaveOccurred())

				build, err = engine.LookupBuild(db.Build{
					Engine:         engine.Name(),
					EngineMetadata: string(metadataPayload),
				})
				Ω(err).ShouldNot(HaveOccurred())
			})

			JustBeforeEach(func() {
				abortErr = build.Abort()
			})

			Context("when the Turbine endpoint succeeds", func() {
				BeforeEach(func() {
					buildEndpoint.AppendHandlers(
						ghttp.VerifyRequest("POST", "/builds/some-guid/abort"),
					)
				})

				It("does not error", func() {
					Ω(abortErr).ShouldNot(HaveOccurred())
				})

				It("aborts via the Turbine API", func() {
					Ω(buildEndpoint.ReceivedRequests()).Should(HaveLen(1))
				})
			})

			Context("when the Turbine endpoint succeeds", func() {
				BeforeEach(func() {
					buildEndpoint.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/builds/some-guid/abort"),
							ghttp.RespondWith(http.StatusInternalServerError, ""),
						),
					)
				})

				It("returns an error", func() {
					Ω(abortErr).Should(HaveOccurred())
				})
			})
		})

		Describe("Resume", func() {
			var (
				buildModel db.Build

				resumeErr error
			)

			BeforeEach(func() {
				metadata := TurbineMetadata{
					Guid:     "some-guid",
					Endpoint: buildEndpoint.URL(),
				}

				metadataPayload, err := json.Marshal(metadata)
				Ω(err).ShouldNot(HaveOccurred())

				buildModel = db.Build{
					ID:             1,
					Engine:         engine.Name(),
					EngineMetadata: string(metadataPayload),
				}
			})

			JustBeforeEach(func() {
				build, err := engine.LookupBuild(buildModel)
				Ω(err).ShouldNot(HaveOccurred())

				resumeErr = build.Resume(lagertest.NewTestLogger("test"))
			})

			Context("when the build's turbine returns events", func() {
				var (
					events []event.Event
				)

				BeforeEach(func() {
					events = []event.Event{}

					buildEndpoint.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/builds/some-guid/events"),
							func(w http.ResponseWriter, r *http.Request) {
								flusher := w.(http.Flusher)

								w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
								w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
								w.Header().Add("Connection", "keep-alive")

								w.WriteHeader(http.StatusOK)

								flusher.Flush()

								for id, e := range events {
									payload, err := json.Marshal(e)
									Ω(err).ShouldNot(HaveOccurred())

									event := sse.Event{
										ID:   fmt.Sprintf("%d", id),
										Name: string(e.EventType()),
										Data: []byte(payload),
									}

									err = event.Write(w)
									if err != nil {
										return
									}

									flusher.Flush()
								}
							},
						),
					)
				})

				itSavesTheEvent := func(count int) {
					It("saves the event", func() {
						Eventually(fakeDB.SaveBuildEventCallCount).Should(BeNumerically(">=", count))

						event := events[count-1]

						payload, err := json.Marshal(event)
						Ω(err).ShouldNot(HaveOccurred())

						buildID, buildEvent := fakeDB.SaveBuildEventArgsForCall(count - 1)
						Ω(buildID).Should(Equal(1))
						Ω(buildEvent).Should(Equal(db.BuildEvent{
							ID:      count - 1,
							Type:    string(event.EventType()),
							Payload: string(payload),
						}))
					})

					Context("when saving the event fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							num := 0

							fakeDB.SaveBuildEventStub = func(int, db.BuildEvent) error {
								num++

								if num == count {
									return disaster
								}

								return nil
							}
						})

						It("returns an error", func() {
							Ω(resumeErr).Should(Equal(disaster))
						})
					})
				}

				for _, v1version := range []string{"1.0", "1.1"} {
					version := v1version

					Context("and they're v"+version, func() {
						BeforeEach(func() {
							events = append(events, event.Version(version))
						})

						itSavesTheEvent(1)

						Context("and a status event appears", func() {
							Context("and it's started", func() {
								BeforeEach(func() {
									events = append(events, event.Status{
										Status: turbine.StatusStarted,
										Time:   1234,
									})
								})

								itSavesTheEvent(2)

								It("saves the build's status", func() {
									Eventually(fakeDB.SaveBuildStatusCallCount).Should(Equal(1))

									buildID, status := fakeDB.SaveBuildStatusArgsForCall(0)
									Ω(buildID).Should(Equal(1))
									Ω(status).Should(Equal(db.StatusStarted))
								})

								It("saves the build's start time", func() {
									Eventually(fakeDB.SaveBuildStartTimeCallCount).Should(Equal(1))

									buildID, startTime := fakeDB.SaveBuildStartTimeArgsForCall(0)
									Ω(buildID).Should(Equal(1))
									Ω(startTime.Unix()).Should(Equal(int64(1234)))
								})
							})

							Context("and it's completed", func() {
								BeforeEach(func() {
									events = append(events, event.Status{
										Status: turbine.StatusSucceeded,
										Time:   1234,
									})
								})

								itSavesTheEvent(2)

								It("saves the build's status", func() {
									Eventually(fakeDB.SaveBuildStatusCallCount).Should(Equal(1))

									buildID, status := fakeDB.SaveBuildStatusArgsForCall(0)
									Ω(buildID).Should(Equal(1))
									Ω(status).Should(Equal(db.StatusSucceeded))
								})

								It("saves the build's end time", func() {
									Eventually(fakeDB.SaveBuildEndTimeCallCount).Should(Equal(1))

									buildID, startTime := fakeDB.SaveBuildEndTimeArgsForCall(0)
									Ω(buildID).Should(Equal(1))
									Ω(startTime.Unix()).Should(Equal(int64(1234)))
								})
							})
						})

						Context("and an input event appears", func() {
							BeforeEach(func() {
								events = append(events, event.Input{
									Input: turbine.Input{
										Name:     "some-input-name",
										Resource: "some-input-resource",
										Type:     "some-type",
										Source:   turbine.Source{"input-source": "some-source"},
										Version:  turbine.Version{"version": "input-version"},
										Metadata: []turbine.MetadataField{
											{Name: "input-meta", Value: "some-value"},
										},
									},
								})
							})

							itSavesTheEvent(2)

							Context("and the build is for a job", func() {
								BeforeEach(func() {
									buildModel.JobName = "lol"
								})

								It("saves the build's input", func() {
									Eventually(fakeDB.SaveBuildInputCallCount).Should(Equal(1))

									id, input := fakeDB.SaveBuildInputArgsForCall(0)
									Ω(id).Should(Equal(1))
									Ω(input).Should(Equal(db.BuildInput{
										Name: "some-input-name",
										VersionedResource: db.VersionedResource{
											Resource: "some-input-resource",
											Type:     "some-type",
											Source:   db.Source{"input-source": "some-source"},
											Version:  db.Version{"version": "input-version"},
											Metadata: []db.MetadataField{
												{Name: "input-meta", Value: "some-value"},
											},
										},
									}))
								})

								Context("and a successful status event appears", func() {
									BeforeEach(func() {
										events = append(events, event.Status{
											Status: turbine.StatusSucceeded,
										})
									})

									itSavesTheEvent(3)

									It("saves the build's input as an implicit output", func() {
										Eventually(fakeDB.SaveBuildOutputCallCount).Should(Equal(1))

										id, output := fakeDB.SaveBuildOutputArgsForCall(0)
										Ω(id).Should(Equal(1))
										Ω(output).Should(Equal(db.VersionedResource{
											Resource: "some-input-resource",
											Type:     "some-type",
											Source:   db.Source{"input-source": "some-source"},
											Version:  db.Version{"version": "input-version"},
											Metadata: []db.MetadataField{
												{Name: "input-meta", Value: "some-value"},
											},
										}))
									})
								})

								Context("and an output event appears for the same input resource", func() {
									BeforeEach(func() {
										events = append(events, event.Output{
											Output: turbine.Output{
												Name:    "some-input-resource", // TODO rename Output.Name to Output.Resource
												Type:    "some-type",
												Source:  turbine.Source{"input-source": "some-source"},
												Version: turbine.Version{"version": "explicit-input-version"},
												Metadata: []turbine.MetadataField{
													{Name: "input-meta", Value: "some-value"},
												},
											},
										})
									})

									itSavesTheEvent(3)

									Context("and a successful status event appears", func() {
										BeforeEach(func() {
											events = append(events, event.Status{
												Status: turbine.StatusSucceeded,
											})
										})

										itSavesTheEvent(4)

										It("saves the explicit output instead of the implicit one", func() {
											Eventually(fakeDB.SaveBuildOutputCallCount).Should(Equal(1))

											id, output := fakeDB.SaveBuildOutputArgsForCall(0)
											Ω(id).Should(Equal(1))
											Ω(output).Should(Equal(db.VersionedResource{
												Resource: "some-input-resource",
												Type:     "some-type",
												Source:   db.Source{"input-source": "some-source"},
												Version:  db.Version{"version": "explicit-input-version"},
												Metadata: []db.MetadataField{
													{Name: "input-meta", Value: "some-value"},
												},
											}))
										})
									})
								})
							})

							Context("and the build is not for a job (i.e. one-off)", func() {
								BeforeEach(func() {
									buildModel.JobName = ""
								})

								It("does not save an input", func() {
									Consistently(fakeDB.SaveBuildInputCallCount).Should(Equal(0))
								})
							})
						})

						Context("and an output event appears", func() {
							BeforeEach(func() {
								events = append(events, event.Output{
									Output: turbine.Output{
										Name:    "some-output-name",
										Type:    "some-type",
										Source:  turbine.Source{"output-source": "some-source"},
										Version: turbine.Version{"version": "output-version"},
										Metadata: []turbine.MetadataField{
											{Name: "output-meta", Value: "some-value"},
										},
									},
								})
							})

							Context("and the build is for a job", func() {
								BeforeEach(func() {
									buildModel.JobName = "lol"
								})

								It("does not save the output immediately", func() {
									Consistently(fakeDB.SaveBuildOutputCallCount).Should(BeZero())
								})

								Context("and a successful status event appears", func() {
									BeforeEach(func() {
										events = append(events, event.Status{
											Status: turbine.StatusSucceeded,
										})
									})

									It("saves the build's output", func() {
										Eventually(fakeDB.SaveBuildOutputCallCount).Should(Equal(1))

										id, output := fakeDB.SaveBuildOutputArgsForCall(0)
										Ω(id).Should(Equal(1))
										Ω(output).Should(Equal(db.VersionedResource{
											Resource: "some-output-name",
											Type:     "some-type",
											Source:   db.Source{"output-source": "some-source"},
											Version:  db.Version{"version": "output-version"},
											Metadata: []db.MetadataField{
												{Name: "output-meta", Value: "some-value"},
											},
										}))
									})
								})

								Context("and an errored status event appears", func() {
									BeforeEach(func() {
										events = append(events, event.Status{
											Status: turbine.StatusErrored,
										})
									})

									It("does not save the build's output", func() {
										Consistently(fakeDB.SaveBuildOutputCallCount).Should(BeZero())
									})
								})
							})

							Context("and the build is not for a job (i.e. one-off)", func() {
								BeforeEach(func() {
									buildModel.JobName = ""
								})

								It("does not save an output", func() {
									Consistently(fakeDB.SaveBuildOutputCallCount).Should(Equal(0))
								})
							})
						})

						Context("and an input event appears, with no resource present", func() {
							BeforeEach(func() {
								events = append(events, event.Input{
									Input: turbine.Input{
										Name:    "some-input-name",
										Type:    "some-type",
										Source:  turbine.Source{"input-source": "some-source"},
										Version: turbine.Version{"version": "input-version"},
										Metadata: []turbine.MetadataField{
											{Name: "input-meta", Value: "some-value"},
										},
									},
								})
							})

							itSavesTheEvent(2)

							Context("and the build is for a job", func() {
								BeforeEach(func() {
									buildModel.JobName = "lol"
								})

								It("saves the build's input by its name, for backwards-compatibility", func() {
									Eventually(fakeDB.SaveBuildInputCallCount).Should(Equal(1))

									id, input := fakeDB.SaveBuildInputArgsForCall(0)
									Ω(id).Should(Equal(1))
									Ω(input).Should(Equal(db.BuildInput{
										Name: "some-input-name",
										VersionedResource: db.VersionedResource{
											Resource: "some-input-name",
											Type:     "some-type",
											Source:   db.Source{"input-source": "some-source"},
											Version:  db.Version{"version": "input-version"},
											Metadata: []db.MetadataField{
												{Name: "input-meta", Value: "some-value"},
											},
										},
									}))
								})
							})
						})

						Context("and an end event appears", func() {
							var buildDeleted <-chan struct{}

							BeforeEach(func() {
								events = append(events, event.End{})

								deleted := make(chan struct{})
								buildDeleted = deleted

								buildEndpoint.AppendHandlers(
									ghttp.CombineHandlers(
										ghttp.VerifyRequest("DELETE", "/builds/some-guid"),
										func(w http.ResponseWriter, r *http.Request) {
											close(deleted)
										},
									),
								)
							})

							It("deletes the build from the turbine", func() {
								Eventually(buildDeleted).Should(BeClosed())
							})
						})
					})
				}

				Context("and they're some other version", func() {
					BeforeEach(func() {
						events = append(
							events,
							event.Version("2.0"),
							event.Status{Status: turbine.StatusSucceeded},
						)
					})

					itSavesTheEvent(1)
					itSavesTheEvent(2)

					It("ignores the events", func() {
						Consistently(fakeDB.SaveBuildStatusCallCount).Should(Equal(0))
					})
				})
			})

			Context("when the build's turbine returns 404", func() {
				BeforeEach(func() {
					buildEndpoint.AppendHandlers(
						ghttp.RespondWith(http.StatusNotFound, ""),
					)
				})

				It("does not return an error", func() {
					Ω(resumeErr).ShouldNot(HaveOccurred())
				})

				It("sets the build's status to errored", func() {
					// TODO some way of messaging this?

					Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(1))

					buildID, status := fakeDB.SaveBuildStatusArgsForCall(0)
					Ω(buildID).Should(Equal(1))
					Ω(status).Should(Equal(db.StatusErrored))
				})
			})

			Context("when the build's turbine is inaccessible", func() {
				BeforeEach(func() {
					buildEndpoint.AppendHandlers(
						func(w http.ResponseWriter, r *http.Request) {
							buildEndpoint.CloseClientConnections()
						},
					)
				})

				It("returns an error", func() {
					Ω(resumeErr).Should(HaveOccurred())
				})

				It("does not update the build's status, as the turbine may be temporarily AWOL (i.e. rolling update)", func() {
					Ω(fakeDB.SaveBuildStatusCallCount()).Should(Equal(0))
				})
			})
		})
	})
})
