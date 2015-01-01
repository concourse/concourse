package engine_test

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/event"
	"github.com/concourse/turbine"
	tevent "github.com/concourse/turbine/event"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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

		engine = NewTurbineEngine(
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
			build     db.Build
			buildPlan atc.BuildPlan

			createdBuild Build
			createErr    error
		)

		BeforeEach(func() {
			build = db.Build{
				ID: 1,
			}

			buildPlan = atc.BuildPlan{
				Config: &atc.BuildConfig{
					Image: "some-image",

					Params: map[string]string{
						"FOO": "1",
						"BAR": "2",
					},

					Run: atc.BuildRunConfig{
						Path: "some-script",
						Args: []string{"arg1", "arg2"},
					},
				},
			}
		})

		JustBeforeEach(func() {
			createdBuild, createErr = engine.CreateBuild(build, buildPlan)
		})

		successfulBuildStart := func(build atc.BuildPlan) http.HandlerFunc {
			return ghttp.CombineHandlers(
				ghttp.VerifyJSONRepresenting(build),
				func(w http.ResponseWriter, r *http.Request) {
					w.Header().Add("X-Turbine-Endpoint", turbineServer.URL())
				},
				ghttp.RespondWithJSONEncoded(201, turbine.Build{
					Guid: "some-build-guid",
				}),
			)
		}

		Context("when the turbine server successfully executes", func() {
			BeforeEach(func() {
				turbineServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/builds"),
						successfulBuildStart(buildPlan),
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
							turbineServer.CloseClientConnections()
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
				buildModel.EngineMetadata = `{"endpoint":"http://example.com"}`
			})

			It("returns an error", func() {
				Ω(lookupErr).Should(HaveOccurred())
			})
		})

		Context("when the build's metadata is missing endpoint", func() {
			BeforeEach(func() {
				buildModel.EngineMetadata = `{"guid":"abc"}`
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
			if buildEndpoint != nil {
				// tests that close it nil it out to prevent double-closing
				buildEndpoint.Close()
			}
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

		Describe("Hijack", func() {
			var (
				buildModel db.Build

				stdoutBuf *gbytes.Buffer
				stderrBuf *gbytes.Buffer

				process   garden.Process
				hijackErr error
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

				stdoutBuf = gbytes.NewBuffer()
				stderrBuf = gbytes.NewBuffer()
			})

			JustBeforeEach(func() {
				build, err := engine.LookupBuild(buildModel)
				Ω(err).ShouldNot(HaveOccurred())

				process, hijackErr = build.Hijack(garden.ProcessSpec{
					Path: "ls",
				}, garden.ProcessIO{
					Stdout: stdoutBuf,
					Stderr: stderrBuf,
					Stdin:  bytes.NewBufferString("marco"),
				})
			})

			Context("when hijacking from the turbine succeeds", func() {
				BeforeEach(func() {
					buildEndpoint.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/builds/some-guid/hijack"),
							ghttp.VerifyJSONRepresenting(garden.ProcessSpec{
								Path: "ls",
							}),
							func(w http.ResponseWriter, r *http.Request) {
								w.WriteHeader(http.StatusOK)

								sconn, sbr, err := w.(http.Hijacker).Hijack()
								Ω(err).ShouldNot(HaveOccurred())

								defer sconn.Close()

								decoder := gob.NewDecoder(sbr)

								payload := turbine.HijackPayload{}
								err = decoder.Decode(&payload)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(payload).Should(Equal(turbine.HijackPayload{
									Stdin: []byte("marco"),
								}))

								_, err = fmt.Fprintf(sconn, "polo\n")
								Ω(err).ShouldNot(HaveOccurred())

								payload = turbine.HijackPayload{}
								err = decoder.Decode(&payload)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(payload).Should(Equal(turbine.HijackPayload{
									TTYSpec: &garden.TTYSpec{
										WindowSize: &garden.WindowSize{
											Columns: 123,
											Rows:    456,
										},
									},
								}))

								_, err = fmt.Fprintf(sconn, "got new tty\n")
								Ω(err).ShouldNot(HaveOccurred())
							},
						),
					)
				})

				It("handles all stdio, tty setting, and waits for the stream to end", func() {
					Ω(hijackErr).ShouldNot(HaveOccurred())

					Eventually(stdoutBuf).Should(gbytes.Say("polo"))

					err := process.SetTTY(garden.TTYSpec{
						WindowSize: &garden.WindowSize{
							Columns: 123,
							Rows:    456,
						},
					})
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(stdoutBuf).Should(gbytes.Say("got new tty"))

					Ω(process.Wait()).Should(Equal(0))
				})
			})

			Context("when the turbine returns a non-200 response", func() {
				BeforeEach(func() {
					buildEndpoint.AppendHandlers(
						ghttp.RespondWith(http.StatusNotFound, ""),
					)
				})

				It("returns ErrBadResponse", func() {
					Ω(hijackErr).Should(Equal(ErrBadResponse))
				})
			})

			Context("when the turbine request is interrupted", func() {
				BeforeEach(func() {
					buildEndpoint.AppendHandlers(
						func(w http.ResponseWriter, r *http.Request) {
							buildEndpoint.CloseClientConnections()
						},
					)
				})

				It("returns an error", func() {
					Ω(hijackErr).Should(HaveOccurred())
				})
			})

			Context("when the turbine is not listening", func() {
				BeforeEach(func() {
					buildEndpoint.Close()
					buildEndpoint = nil
				})

				It("returns an error", func() {
					Ω(hijackErr).Should(HaveOccurred())
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
					events []tevent.Event
				)

				BeforeEach(func() {
					events = []tevent.Event{}

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

				itSavesTheEvent := func(id uint, event atc.Event) {
					It("saves the event", func() {
						savedEvents := fakeDB.SaveBuildEventCallCount()

						buildEvents := make([]atc.Event, savedEvents)
						for i := 0; i < savedEvents; i++ {
							buildID, buildEvent := fakeDB.SaveBuildEventArgsForCall(i)
							buildEvents[i] = buildEvent
							Ω(buildID).Should(Equal(1))
						}

						Ω(buildEvents).Should(ContainElement(event))
					})

					It("saves the metadata with the new ID", func() {
						Ω(fakeDB.SaveBuildEngineMetadataCallCount()).Should(BeNumerically("==", id))

						metadataPayload, err := json.Marshal(TurbineMetadata{
							Guid:        "some-guid",
							Endpoint:    buildEndpoint.URL(),
							LastEventID: &id,
						})
						Ω(err).ShouldNot(HaveOccurred())

						savedBuildID, savedMetadata := fakeDB.SaveBuildEngineMetadataArgsForCall(int(id) - 1)
						Ω(savedBuildID).Should(Equal(1))
						Ω(savedMetadata).Should(Equal(string(metadataPayload)))
					})

					Context("when saving the event fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							var num uint

							fakeDB.SaveBuildEventStub = func(int, atc.Event) error {
								num++

								if num == id {
									return disaster
								}

								return nil
							}
						})

						It("returns an error", func() {
							Ω(resumeErr).Should(Equal(disaster))
						})
					})

					Context("when saving the engine metadata fails", func() {
						disaster := errors.New("oh no!")

						BeforeEach(func() {
							var num uint

							fakeDB.SaveBuildEngineMetadataStub = func(int, string) error {
								num++

								if num == id {
									return disaster
								}

								return nil
							}
						})

						It("returns an error", func() {
							Ω(resumeErr).Should(Equal(disaster))
						})
					})

					Context("when resuming from after the event id", func() {
						BeforeEach(func() {
							metadata := TurbineMetadata{
								Guid:        "some-guid",
								Endpoint:    buildEndpoint.URL(),
								LastEventID: &id,
							}

							metadataPayload, err := json.Marshal(metadata)
							Ω(err).ShouldNot(HaveOccurred())

							buildModel = db.Build{
								ID:             1,
								Engine:         engine.Name(),
								EngineMetadata: string(metadataPayload),
							}
						})

						It("does not save the event", func() {
							savedEvents := fakeDB.SaveBuildEventCallCount()

							buildEvents := make([]atc.Event, savedEvents)
							for i := 0; i < savedEvents; i++ {
								buildID, buildEvent := fakeDB.SaveBuildEventArgsForCall(i)
								buildEvents[i] = buildEvent
								Ω(buildID).Should(Equal(1))
							}

							Ω(buildEvents).ShouldNot(ContainElement(event))
						})
					})
				}

				for _, v1version := range []string{"1.0", "1.1"} {
					version := v1version

					Context("and they're v"+version, func() {
						BeforeEach(func() {
							events = append(events, tevent.Version(version))
						})

						It("does not save the version", func() {
							Ω(fakeDB.SaveBuildEventCallCount()).Should(BeZero())
						})

						Context("and a status event appears", func() {
							Context("and it's started", func() {
								BeforeEach(func() {
									events = append(events, tevent.Status{
										Status: turbine.StatusStarted,
										Time:   1234,
									})
								})

								itSavesTheEvent(1, event.Status{
									Status: atc.StatusStarted,
									Time:   1234,
								})

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
									events = append(events, tevent.Status{
										Status: turbine.StatusSucceeded,
										Time:   1234,
									})
								})

								itSavesTheEvent(1, event.Status{
									Status: atc.StatusSucceeded,
									Time:   1234,
								})

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
							var input turbine.Input

							BeforeEach(func() {
								input = turbine.Input{
									Name:       "some-input-name",
									Resource:   "some-input-resource",
									Type:       "some-type",
									Source:     turbine.Source{"input-source": "some-source"},
									Params:     turbine.Params{"input-param": "some-param"},
									Version:    turbine.Version{"version": "input-version"},
									ConfigPath: "foo/build.yml",
									Metadata: []turbine.MetadataField{
										{Name: "input-meta", Value: "some-value"},
									},
								}
							})

							Context("and the input corresponds to a resource", func() {
								BeforeEach(func() {
									input.Resource = "some-input-resource"

									events = append(events, tevent.Input{
										Input: input,
									})
								})

								itSavesTheEvent(1, event.Input{
									Plan: atc.InputPlan{
										Name:       "some-input-name",
										Resource:   "some-input-resource",
										Type:       "some-type",
										Version:    atc.Version{"version": "input-version"}, // preserved as it may have been requested
										Source:     atc.Source{"input-source": "some-source"},
										Params:     atc.Params{"input-param": "some-param"},
										ConfigPath: "foo/build.yml",
									},
									FetchedVersion: atc.Version{"version": "input-version"},
									FetchedMetadata: []atc.MetadataField{
										{Name: "input-meta", Value: "some-value"},
									},
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
										events = append(events, tevent.Status{
											Status: turbine.StatusSucceeded,
											Time:   1234,
										})
									})

									itSavesTheEvent(2, event.Status{
										Status: atc.StatusSucceeded,
										Time:   1234,
									})

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
										events = append(events, tevent.Output{
											Output: turbine.Output{
												Name:    "some-input-resource", // TODO rename Output.Name to Output.Resource
												Type:    "some-type",
												On:      turbine.OutputConditions{turbine.OutputConditionFailure},
												Source:  turbine.Source{"input-source": "some-source"},
												Params:  turbine.Params{"output-param": "some-param"},
												Version: turbine.Version{"version": "explicit-input-version"},
												Metadata: []turbine.MetadataField{
													{Name: "input-meta", Value: "some-value"},
												},
											},
										})
									})

									itSavesTheEvent(2, event.Output{
										Plan: atc.OutputPlan{
											Name:   "some-input-resource",
											Type:   "some-type",
											On:     atc.OutputConditions{atc.OutputConditionFailure},
											Source: atc.Source{"input-source": "some-source"},
											Params: atc.Params{"output-param": "some-param"},
										},
										CreatedVersion: atc.Version{"version": "explicit-input-version"},
										CreatedMetadata: []atc.MetadataField{
											{Name: "input-meta", Value: "some-value"},
										},
									})

									Context("and a successful status event appears", func() {
										BeforeEach(func() {
											events = append(events, tevent.Status{
												Status: turbine.StatusSucceeded,
												Time:   1234,
											})
										})

										itSavesTheEvent(3, event.Status{
											Status: atc.StatusSucceeded,
											Time:   1234,
										})

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

							Context("and the input does not correspond to a resource (one-off)", func() {
								BeforeEach(func() {
									input.Resource = ""

									events = append(events, tevent.Input{
										Input: input,
									})
								})

								itSavesTheEvent(1, event.Input{
									Plan: atc.InputPlan{
										Name:       "some-input-name",
										Resource:   "",
										Type:       "some-type",
										Version:    atc.Version{"version": "input-version"}, // preserved as it may have been requested
										Source:     atc.Source{"input-source": "some-source"},
										Params:     atc.Params{"input-param": "some-param"},
										ConfigPath: "foo/build.yml",
									},
									FetchedVersion: atc.Version{"version": "input-version"},
									FetchedMetadata: []atc.MetadataField{
										{Name: "input-meta", Value: "some-value"},
									},
								})

								It("does not save an input", func() {
									Consistently(fakeDB.SaveBuildInputCallCount).Should(Equal(0))
								})
							})
						})

						Context("and an output event appears", func() {
							BeforeEach(func() {
								events = append(events, tevent.Output{
									Output: turbine.Output{
										Name:    "some-output-name",
										Type:    "some-type",
										On:      turbine.OutputConditions{turbine.OutputConditionFailure},
										Source:  turbine.Source{"output-source": "some-source"},
										Params:  turbine.Params{"output-param": "some-param"},
										Version: turbine.Version{"version": "output-version"},
										Metadata: []turbine.MetadataField{
											{Name: "output-meta", Value: "some-value"},
										},
									},
								})
							})

							itSavesTheEvent(1, event.Output{
								Plan: atc.OutputPlan{
									Name:   "some-output-name",
									Type:   "some-type",
									On:     atc.OutputConditions{atc.OutputConditionFailure},
									Source: atc.Source{"output-source": "some-source"},
									Params: atc.Params{"output-param": "some-param"},
								},
								CreatedVersion: atc.Version{"version": "output-version"},
								CreatedMetadata: []atc.MetadataField{
									{Name: "output-meta", Value: "some-value"},
								},
							})

							It("does not save the output immediately", func() {
								Consistently(fakeDB.SaveBuildOutputCallCount).Should(BeZero())
							})

							Context("and a successful status event appears", func() {
								BeforeEach(func() {
									events = append(events, tevent.Status{
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
									events = append(events, tevent.Status{
										Status: turbine.StatusErrored,
									})
								})

								It("does not save the build's output", func() {
									Consistently(fakeDB.SaveBuildOutputCallCount).Should(BeZero())
								})
							})
						})

						Context("and an end event appears", func() {
							var buildDeleted <-chan struct{}

							BeforeEach(func() {
								events = append(events, tevent.End{})

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

							It("marks the build as completed", func() {
								Ω(fakeDB.CompleteBuildCallCount()).Should(Equal(1))
								Ω(fakeDB.CompleteBuildArgsForCall(0)).Should(Equal(buildModel.ID))
							})

							Context("when marking the build as completed fails", func() {
								disaster := errors.New("oh no!")

								BeforeEach(func() {
									fakeDB.CompleteBuildReturns(disaster)
								})

								It("returns the error", func() {
									Ω(resumeErr).Should(Equal(disaster))
								})
							})
						})
					})
				}

				Context("and they're some other version", func() {
					BeforeEach(func() {
						events = append(
							events,
							tevent.Version("2.0"),
							tevent.Status{Status: turbine.StatusSucceeded},
						)
					})

					It("ignores the events", func() {
						Consistently(fakeDB.SaveBuildEventCallCount).Should(Equal(0))
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
