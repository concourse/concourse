package engine_test

import (
	"context"
	"errors"
	"io"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("BuildStepDelegate", func() {
	var (
		logger            *lagertest.TestLogger
		fakeBuild         *dbfakes.FakeBuild
		fakeClock         *fakeclock.FakeClock
		planID            atc.PlanID
		runState          *execfakes.FakeRunState
		fakePolicyChecker *policyfakes.FakeChecker
		fakeSecrets       *credsfakes.FakeSecrets

		now = time.Date(1991, 6, 3, 5, 30, 0, 0, time.UTC)

		delegate exec.BuildStepDelegate
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeBuild = new(dbfakes.FakeBuild)
		fakeClock = fakeclock.NewFakeClock(now)
		planID = "some-plan-id"

		runState = new(execfakes.FakeRunState)
		runState.RedactionEnabledReturns(true)

		repo := build.NewRepository()
		runState.ArtifactRepositoryReturns(repo)

		fakePolicyChecker = new(policyfakes.FakeChecker)

		fakeSecrets = new(credsfakes.FakeSecrets)
		delegate = engine.NewBuildStepDelegate(fakeBuild, planID, runState, fakeClock, fakePolicyChecker, fakeSecrets)
	})

	Describe("Initializing", func() {
		JustBeforeEach(func() {
			delegate.Initializing(logger)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("initialize")))
		})
	})

	Describe("Finished", func() {
		JustBeforeEach(func() {
			delegate.Finished(logger, true)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			event := fakeBuild.SaveEventArgsForCall(0)
			Expect(event.EventType()).To(Equal(atc.EventType("finish")))
		})
	})

	Describe("FetchImage", func() {
		var expectedCheckPlan, expectedGetPlan atc.Plan
		var fakeArtifact *runtimefakes.FakeArtifact
		var fakeResourceCache *dbfakes.FakeUsedResourceCache

		var childState *execfakes.FakeRunState
		var imageResource atc.ImageResource
		var types atc.VersionedResourceTypes
		var privileged bool

		var imageSpec worker.ImageSpec
		var fetchErr error

		BeforeEach(func() {
			repo := build.NewRepository()
			runState.ArtifactRepositoryReturns(repo)

			childState = new(execfakes.FakeRunState)
			runState.NewScopeReturns(childState)

			fakeArtifact = new(runtimefakes.FakeArtifact)
			childState.ArtifactRepositoryReturns(repo.NewScope())
			childState.ArtifactRepository().RegisterArtifact("image", fakeArtifact)

			buildVariables := build.NewVariables(nil, vars.NewTracker(true))
			buildVariables.SetVar("some-source", "source-var", "super-secret-source", true)
			buildVariables.SetVar("some-source", "params-var", "super-secret-params", true)
			runState.LocalVariablesReturns(buildVariables)

			imageResource = atc.ImageResource{
				Type:   "docker",
				Source: atc.Source{"some": "((source-var))"},
				Params: atc.Params{"some": "((params-var))"},
				Tags:   atc.Tags{"some", "tags"},
			}

			types = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "some-custom-type",
						Type:   "another-custom-type",
						Source: atc.Source{"some-custom": "((source-var))"},
						Params: atc.Params{"some-custom": "((params-var))"},
					},
					Version: atc.Version{"some-custom": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:       "another-custom-type",
						Type:       "registry-image",
						Source:     atc.Source{"another-custom": "((source-var))"},
						Privileged: true,
					},
					Version: atc.Version{"another-custom": "version"},
				},
			}

			expectedCheckPlan = atc.Plan{
				ID: planID + "/image-check",
				Check: &atc.CheckPlan{
					Name:                   "image",
					Type:                   "docker",
					Source:                 atc.Source{"some": "((source-var))"},
					VersionedResourceTypes: types,
					Tags:                   atc.Tags{"some", "tags"},
				},
			}

			expectedGetPlan = atc.Plan{
				ID: planID + "/image-get",
				Get: &atc.GetPlan{
					Name:                   "image",
					Type:                   "docker",
					Source:                 atc.Source{"some": "((source-var))"},
					Version:                &atc.Version{"some": "version"},
					Params:                 atc.Params{"some": "((params-var))"},
					VersionedResourceTypes: types,
					Tags:                   atc.Tags{"some", "tags"},
				},
			}

			fakeResourceCache = new(dbfakes.FakeUsedResourceCache)

			childState.ResultStub = func(planID atc.PlanID, to interface{}) bool {
				switch planID {
				case expectedCheckPlan.ID:
					switch x := to.(type) {
					case *atc.Version:
						*x = atc.Version{"some": "version"}
					default:
						Fail("unexpected target type")
					}
				case expectedGetPlan.ID:
					switch x := to.(type) {
					case *db.UsedResourceCache:
						*x = fakeResourceCache
					default:
						Fail("unexpected target type")
					}
				default:
					Fail("unknown result key: " + planID.String())
				}

				return true
			}

			privileged = false

			childState.RunReturns(true, nil)
		})

		JustBeforeEach(func() {
			imageSpec, fetchErr = delegate.FetchImage(context.TODO(), imageResource, types, privileged)
		})

		It("succeeds", func() {
			Expect(fetchErr).ToNot(HaveOccurred())
		})

		It("returns an image spec containing the artifact", func() {
			Expect(imageSpec).To(Equal(worker.ImageSpec{
				ImageArtifact: fakeArtifact,
				Privileged:    false,
			}))
		})

		It("runs a CheckPlan to get the image version", func() {
			Expect(childState.RunCallCount()).To(Equal(2))

			_, plan := childState.RunArgsForCall(0)
			Expect(plan).To(Equal(expectedCheckPlan))

			_, plan = childState.RunArgsForCall(1)
			Expect(plan).To(Equal(expectedGetPlan))
		})

		It("records the resource cache as an image resource for the build", func() {
			Expect(fakeBuild.SaveImageResourceVersionCallCount()).To(Equal(1))
			Expect(fakeBuild.SaveImageResourceVersionArgsForCall(0)).To(Equal(fakeResourceCache))
		})

		Context("when privileged", func() {
			BeforeEach(func() {
				privileged = true
			})

			It("returns a privileged image spec", func() {
				Expect(imageSpec).To(Equal(worker.ImageSpec{
					ImageArtifact: fakeArtifact,
					Privileged:    true,
				}))
			})
		})

		Describe("policy checking", func() {
			BeforeEach(func() {
				fakeBuild.TeamNameReturns("some-team")
				fakeBuild.PipelineNameReturns("some-pipeline")
			})

			Context("when the action does not need to be checked", func() {
				BeforeEach(func() {
					fakePolicyChecker.ShouldCheckActionReturns(false)
				})

				It("succeeds", func() {
					Expect(fetchErr).ToNot(HaveOccurred())
				})

				It("checked if ActionUseImage is enabled", func() {
					Expect(fakePolicyChecker.ShouldCheckActionCallCount()).To(Equal(1))
					action := fakePolicyChecker.ShouldCheckActionArgsForCall(0)
					Expect(action).To(Equal(policy.ActionUseImage))
				})

				It("does not check", func() {
					Expect(fakePolicyChecker.CheckCallCount()).To(Equal(0))
				})
			})

			Context("when the action needs to be checked", func() {
				BeforeEach(func() {
					fakePolicyChecker.ShouldCheckActionReturns(true)
				})

				Context("when the check is allowed", func() {
					BeforeEach(func() {
						fakePolicyChecker.CheckReturns(policy.PolicyCheckOutput{
							Allowed: true,
						}, nil)
					})

					It("succeeds", func() {
						Expect(fetchErr).ToNot(HaveOccurred())
					})

					It("checked with the right values", func() {
						Expect(fakePolicyChecker.CheckCallCount()).To(Equal(1))
						input := fakePolicyChecker.CheckArgsForCall(0)
						Expect(input).To(Equal(policy.PolicyCheckInput{
							Action:   policy.ActionUseImage,
							Team:     "some-team",
							Pipeline: "some-pipeline",
							Data: map[string]interface{}{
								"image_type":   "docker",
								"image_source": atc.Source{"some": "((source-var))"},
								"privileged":   false,
							},
						}))
					})

					Context("when the image source contains credentials", func() {
						BeforeEach(func() {
							imageResource.Source = atc.Source{"some": "super-secret-source"}

							runState.IterateInterpolatedCredsStub = func(iter vars.TrackedVarsIterator) {
								iter.YieldCred("source-var", "super-secret-source")
							}
						})

						It("redacts the value prior to checking", func() {
							Expect(fakePolicyChecker.CheckCallCount()).To(Equal(1))
							input := fakePolicyChecker.CheckArgsForCall(0)
							Expect(input).To(Equal(policy.PolicyCheckInput{
								Action:   policy.ActionUseImage,
								Team:     "some-team",
								Pipeline: "some-pipeline",
								Data: map[string]interface{}{
									"image_type":   "docker",
									"image_source": atc.Source{"some": "((redacted))"},
									"privileged":   false,
								},
							}))
						})
					})

					Context("when privileged", func() {
						BeforeEach(func() {
							privileged = true
						})

						It("checks with privileged", func() {
							Expect(fakePolicyChecker.CheckCallCount()).To(Equal(1))
							input := fakePolicyChecker.CheckArgsForCall(0)
							Expect(input).To(Equal(policy.PolicyCheckInput{
								Action:   policy.ActionUseImage,
								Team:     "some-team",
								Pipeline: "some-pipeline",
								Data: map[string]interface{}{
									"image_type":   "docker",
									"image_source": atc.Source{"some": "((source-var))"},
									"privileged":   true,
								},
							}))
						})
					})
				})
			})
		})

		Describe("ordering", func() {
			BeforeEach(func() {
				fakeBuild.SaveEventStub = func(ev atc.Event) error {
					switch ev.(type) {
					case event.ImageCheck:
						Expect(childState.RunCallCount()).To(Equal(0))
					case event.ImageGet:
						Expect(childState.RunCallCount()).To(Equal(1))
					default:
						Fail("unknown event type")
					}
					return nil
				}
			})

			It("sends events before each run", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(2))
				e := fakeBuild.SaveEventArgsForCall(0)
				Expect(e).To(Equal(event.ImageCheck{
					Time: 675927000,
					Origin: event.Origin{
						ID: event.OriginID(planID),
					},
					PublicPlan: expectedCheckPlan.Public(),
				}))

				e = fakeBuild.SaveEventArgsForCall(1)
				Expect(e).To(Equal(event.ImageGet{
					Time: 675927000,
					Origin: event.Origin{
						ID: event.OriginID(planID),
					},
					PublicPlan: expectedGetPlan.Public(),
				}))
			})
		})

		Context("when a version is already provided", func() {
			BeforeEach(func() {
				imageResource.Version = atc.Version{"some": "version"}
			})

			It("does not run a CheckPlan", func() {
				Expect(childState.RunCallCount()).To(Equal(1))
				_, plan := childState.RunArgsForCall(0)
				Expect(plan).To(Equal(expectedGetPlan))

				Expect(childState.ResultCallCount()).To(Equal(1))
				planID, _ := childState.ResultArgsForCall(0)
				Expect(planID).To(Equal(expectedGetPlan.ID))
			})

			It("only saves an ImageGet event", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				e := fakeBuild.SaveEventArgsForCall(0)
				Expect(e).To(Equal(event.ImageGet{
					Time: 675927000,
					Origin: event.Origin{
						ID: event.OriginID(planID),
					},
					PublicPlan: expectedGetPlan.Public(),
				}))
			})
		})

		Context("when an image name is provided", func() {
			var namedArtifact *runtimefakes.FakeArtifact

			BeforeEach(func() {
				imageResource.Name = "some-name"
				expectedCheckPlan.Check.Name = "some-name"
				expectedGetPlan.Get.Name = "some-name"

				namedArtifact = new(runtimefakes.FakeArtifact)
				childState.ArtifactRepositoryReturns(runState.ArtifactRepository().NewScope())
				childState.ArtifactRepository().RegisterArtifact("some-name", namedArtifact)
			})

			It("uses it for the step names", func() {
				Expect(childState.RunCallCount()).To(Equal(2))
				_, plan := childState.RunArgsForCall(0)
				Expect(plan.Check.Name).To(Equal("some-name"))
				_, plan = childState.RunArgsForCall(1)
				Expect(plan.Get.Name).To(Equal("some-name"))

				Expect(imageSpec.ImageArtifact).To(Equal(namedArtifact))
			})
		})

		Context("when checking the image fails", func() {
			BeforeEach(func() {
				childState.RunStub = func(ctx context.Context, plan atc.Plan) (bool, error) {
					if plan.ID == expectedCheckPlan.ID {
						return false, nil
					}

					return true, nil
				}
			})

			It("errors", func() {
				Expect(fetchErr).To(MatchError("image check failed"))
			})
		})

		Context("when no version is returned by the check", func() {
			BeforeEach(func() {
				childState.ResultReturns(false)
			})

			It("errors", func() {
				Expect(fetchErr).To(MatchError("check did not return a version"))
			})
		})
	})

	Describe("Get", func() {
		var (
			stepVariables      vars.Variables
			buildVariables     *build.Variables
			sources            atc.VarSourceConfigs
			varRef             vars.Reference
			getVarID           atc.PlanID
			expectedGetVarPlan atc.Plan

			childState *execfakes.FakeRunState

			value    interface{}
			fetched  bool
			fetchErr error
		)

		BeforeEach(func() {
			stepVariables = delegate.Variables(context.TODO(), sources)

			buildVariables = build.NewVariables(sources, vars.NewTracker(true))
			runState.LocalVariablesReturns(buildVariables)

			sources = atc.VarSourceConfigs{
				{
					Name: "some-var-source",
					Type: "registry-image",
					Config: map[string]interface{}{
						"var": "config",
					},
				},
				{
					Name: "other-var-source",
					Type: "registry-image",
					Config: map[string]interface{}{
						"var": "other-config",
					},
				},
			}

			getVarID = planID + "/get-var/some-var-source/path"

			expectedGetVarPlan = atc.Plan{
				ID: getVarID,
				GetVar: &atc.GetVarPlan{
					Name:   "some-var-source",
					Path:   "path",
					Type:   "registry-image",
					Source: atc.Source{"var": "config"},
				},
			}

			varRef = vars.Reference{
				Source: "some-var-source",
				Path:   "path",
			}

			childState = new(execfakes.FakeRunState)
			runState.NewScopeReturns(childState)
			runState.RunReturns(true, nil)
			runState.ResultStub = func(planID atc.PlanID, to interface{}) bool {
				Expect(planID).To(Equal(getVarID))
				to = "fetched-value"

				return true
			}
		})

		JustBeforeEach(func() {
			value, fetched, fetchErr = stepVariables.Get(varRef)
		})

		Context("when the var does not have a source (global vars)", func() {
			BeforeEach(func() {
				varRef.Source = ""

				fakeSecrets.NewSecretLookupPathsReturns(nil)
			})

			It("calls get off the global secrets", func() {
				Expect(fakeSecrets.GetCallCount()).To(Equal(1))
				Expect(fakeSecrets.GetArgsForCall(0)).To(Equal(varRef.Path))
			})
		})

		Context("when the var is found in the build vars", func() {
			BeforeEach(func() {
				buildVariables.SetVar("some-var-source", "path", "fetched-value", true)
			})

			It("succeeds", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetched).To(BeTrue())
			})

			It("returns the value", func() {
				Expect(value).To(Equal("fetched-value"))
			})

			It("did not spawn get var sub step", func() {
				Expect(childState.RunCallCount()).To(Equal(0))
			})
		})

		Context("when the var is not found in the build vars", func() {
			It("creates a new scope for the get var substep", func() {
				Expect(childState.NewScopeCallCount()).To(Equal(1))
			})

			It("sets new var source configs for the child state", func() {
				Expect(childState.SetVarSourceConfigsCallCount()).To(Equal(1))
				Expect(childState.SetVarSourceConfigsArgsForCall(0)).To(Equal(sources))
			})

			It("saves a build event for the sub get var plan", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				e := fakeBuild.SaveEventArgsForCall(0)
				Expect(e).To(Equal(event.SubGetVar{
					Time: 675927000,
					Origin: event.Origin{
						ID: event.OriginID(planID),
					},
					PublicPlan: expectedGetVarPlan.Public(),
				}))
			})

			It("runs a GetVar plan to get the var value", func() {
				Expect(runState.RunCallCount()).To(Equal(1))

				_, plan := runState.RunArgsForCall(0)
				Expect(plan).To(Equal(expectedGetVarPlan))
			})

			It("succeeds", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetched).To(BeTrue())
			})

			It("returns the value", func() {
				Expect(value).To(Equal("fetched-value"))
			})

			Context("when the var source is not found", func() {
				BeforeEach(func() {
					sources = atc.VarSourceConfigs{
						{
							Name: "other-var-source",
							Type: "registry-image",
							Config: map[string]interface{}{
								"var": "other-config",
							},
						},
					}

					buildVariables = build.NewVariables(sources, vars.NewTracker(true))
				})

				It("returns no matching var source error", func() {
					Expect(fetchErr).To(Equal(engine.ErrNoMatchingVarSource{"some-var-source"}))
				})
			})
		})

		Context("when running the get var step fails", func() {
			BeforeEach(func() {
				runState.RunStub = func(ctx context.Context, plan atc.Plan) (bool, error) {
					return false, nil
				}
			})

			It("errors", func() {
				Expect(fetchErr).To(MatchError("get var failed"))
			})
		})

		Context("when no result is returned by the get var step", func() {
			BeforeEach(func() {
				runState.ResultReturns(false)
			})

			It("errors", func() {
				Expect(fetchErr).To(MatchError("get var did not return a value"))
			})
		})
	})

	Describe("Stdout", func() {
		var writer io.Writer

		BeforeEach(func() {
			writer = delegate.Stdout()
		})

		Describe("writing to the writer", func() {
			var writtenBytes int
			var writeErr error

			JustBeforeEach(func() {
				writtenBytes, writeErr = writer.Write([]byte("hello\nworld"))
				writer.(io.Closer).Close()
			})

			Context("when saving the event succeeds", func() {
				BeforeEach(func() {
					fakeBuild.SaveEventReturns(nil)
				})

				It("returns the length of the string, and no error", func() {
					Expect(writtenBytes).To(Equal(len("hello\nworld")))
					Expect(writeErr).ToNot(HaveOccurred())
				})

				It("saves a log event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(2))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "hello\n",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
					Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "world",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
				})
			})

			Context("when saving the event fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBuild.SaveEventReturns(disaster)
				})

				It("returns 0 length, and the error", func() {
					Expect(writtenBytes).To(Equal(0))
					Expect(writeErr).To(Equal(disaster))
				})
			})
		})
	})

	Describe("Stderr", func() {
		var writer io.Writer

		BeforeEach(func() {
			writer = delegate.Stderr()
		})

		Describe("writing to the writer", func() {
			var writtenBytes int
			var writeErr error

			JustBeforeEach(func() {
				writtenBytes, writeErr = writer.Write([]byte("hello\n"))
				writer.(io.Closer).Close()
			})

			Context("when saving the event succeeds", func() {
				BeforeEach(func() {
					fakeBuild.SaveEventReturns(nil)
				})

				It("returns the length of the string, and no error", func() {
					Expect(writtenBytes).To(Equal(len("hello\n")))
					Expect(writeErr).ToNot(HaveOccurred())
				})

				It("saves a log event", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "hello\n",
						Origin: event.Origin{
							Source: event.OriginSourceStderr,
							ID:     "some-plan-id",
						},
					}))
				})
			})

			Context("when saving the event fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBuild.SaveEventReturns(disaster)
				})

				It("returns 0 length, and the error", func() {
					Expect(writtenBytes).To(Equal(0))
					Expect(writeErr).To(Equal(disaster))
				})
			})
		})
	})

	Describe("Errored", func() {
		JustBeforeEach(func() {
			delegate.Errored(logger, "fake error message")
		})

		Context("when saving the event succeeds", func() {
			BeforeEach(func() {
				fakeBuild.SaveEventReturns(nil)
			})

			It("saves it with the current time", func() {
				Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
				Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Error{
					Time:    now.Unix(),
					Message: "fake error message",
					Origin: event.Origin{
						ID: "some-plan-id",
					},
				}))
			})
		})

		Context("when saving the event fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeBuild.SaveEventReturns(disaster)
			})

			It("logs an error", func() {
				logs := logger.Logs()
				Expect(len(logs)).To(Equal(1))
				Expect(logs[0].Message).To(Equal("test.failed-to-save-error-event"))
				Expect(logs[0].Data).To(Equal(lager.Data{"error": "nope"}))
			})
		})
	})

	Describe("No line buffer without secrets redaction", func() {
		var runState exec.RunState

		BeforeEach(func() {
			runState = exec.NewRunState(noopStepper, nil, false)
			delegate = engine.NewBuildStepDelegate(fakeBuild, "some-plan-id", runState, fakeClock, fakePolicyChecker, fakeSecrets)
		})

		Context("Stdout", func() {
			It("should not buffer lines", func() {
				writer := delegate.Stdout()
				writtenBytes, writeErr := writer.Write([]byte("1\r"))
				Expect(writeErr).To(BeNil())
				Expect(writtenBytes).To(Equal(len("1\r")))
				writtenBytes, writeErr = writer.Write([]byte("2\r"))
				Expect(writeErr).To(BeNil())
				Expect(writtenBytes).To(Equal(len("2\r")))
				writtenBytes, writeErr = writer.Write([]byte("3\r"))
				Expect(writeErr).To(BeNil())
				Expect(writtenBytes).To(Equal(len("3\r")))
				writeErr = writer.(io.Closer).Close()
				Expect(writeErr).To(BeNil())

				Expect(fakeBuild.SaveEventCallCount()).To(Equal(3))
				Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
					Time:    now.Unix(),
					Payload: "1\r",
					Origin: event.Origin{
						Source: event.OriginSourceStdout,
						ID:     "some-plan-id",
					},
				}))
				Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
					Time:    now.Unix(),
					Payload: "2\r",
					Origin: event.Origin{
						Source: event.OriginSourceStdout,
						ID:     "some-plan-id",
					},
				}))
				Expect(fakeBuild.SaveEventArgsForCall(2)).To(Equal(event.Log{
					Time:    now.Unix(),
					Payload: "3\r",
					Origin: event.Origin{
						Source: event.OriginSourceStdout,
						ID:     "some-plan-id",
					},
				}))
			})
		})

		Context("Stderr", func() {
			It("should not buffer lines", func() {
				writer := delegate.Stderr()
				writtenBytes, writeErr := writer.Write([]byte("1\r"))
				Expect(writeErr).To(BeNil())
				Expect(writtenBytes).To(Equal(len("1\r")))
				writtenBytes, writeErr = writer.Write([]byte("2\r"))
				Expect(writeErr).To(BeNil())
				Expect(writtenBytes).To(Equal(len("2\r")))
				writtenBytes, writeErr = writer.Write([]byte("3\r"))
				Expect(writeErr).To(BeNil())
				Expect(writtenBytes).To(Equal(len("3\r")))
				writeErr = writer.(io.Closer).Close()
				Expect(writeErr).To(BeNil())

				Expect(fakeBuild.SaveEventCallCount()).To(Equal(3))
				Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
					Time:    now.Unix(),
					Payload: "1\r",
					Origin: event.Origin{
						Source: event.OriginSourceStderr,
						ID:     "some-plan-id",
					},
				}))
				Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
					Time:    now.Unix(),
					Payload: "2\r",
					Origin: event.Origin{
						Source: event.OriginSourceStderr,
						ID:     "some-plan-id",
					},
				}))
				Expect(fakeBuild.SaveEventArgsForCall(2)).To(Equal(event.Log{
					Time:    now.Unix(),
					Payload: "3\r",
					Origin: event.Origin{
						Source: event.OriginSourceStderr,
						ID:     "some-plan-id",
					},
				}))
			})
		})
	})

	Describe("Secrets redaction", func() {
		var (
			runState     exec.RunState
			writer       io.Writer
			writtenBytes int
			writeErr     error
		)

		BeforeEach(func() {
			runState = exec.NewRunState(noopStepper, nil, true)
			delegate = engine.NewBuildStepDelegate(fakeBuild, "some-plan-id", runState, fakeClock, fakePolicyChecker, fakeSecrets)

			runState.LocalVariables().SetVar(".", "source-param", "super-secret-source", true)
			runState.LocalVariables().SetVar(".", "git-key", "{\n123\n456\n789\n}\n", true)
		})

		Context("Stdout", func() {
			Context("single-line secret", func() {
				JustBeforeEach(func() {
					writer = delegate.Stdout()
					writtenBytes, writeErr = writer.Write([]byte("ok super-secret-source ok"))
					writer.(io.Closer).Close()
				})

				It("should be redacted", func() {
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len("ok super-secret-source ok")))
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "ok ((redacted)) ok",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
				})
			})

			Context("multi-line secret", func() {
				var logLines string

				JustBeforeEach(func() {
					logLines = "ok123ok\nok456ok\nok789ok\n"
					writer = delegate.Stdout()
					writtenBytes, writeErr = writer.Write([]byte(logLines))
					writer.(io.Closer).Close()
				})

				It("should be redacted", func() {
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len(logLines)))
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "ok((redacted))ok\nok((redacted))ok\nok((redacted))ok\n",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
				})
			})

			Context("multi-line secret with random log chunk", func() {
				JustBeforeEach(func() {
					writer = delegate.Stdout()
					writtenBytes, writeErr = writer.Write([]byte("ok123ok\nok4"))
					writtenBytes, writeErr = writer.Write([]byte("56ok\nok789ok\n"))
					writer.(io.Closer).Close()
				})

				It("should be redacted", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(2))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "ok((redacted))ok\n",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
					Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "ok((redacted))ok\nok((redacted))ok\n",
						Origin: event.Origin{
							Source: event.OriginSourceStdout,
							ID:     "some-plan-id",
						},
					}))
				})
			})
		})

		Context("Stderr", func() {
			Context("single-line secret", func() {
				JustBeforeEach(func() {
					writer = delegate.Stderr()
					writtenBytes, writeErr = writer.Write([]byte("ok super-secret-source ok"))
					writer.(io.Closer).Close()
				})

				It("should be redacted", func() {
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len("ok super-secret-source ok")))
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "ok ((redacted)) ok",
						Origin: event.Origin{
							Source: event.OriginSourceStderr,
							ID:     "some-plan-id",
						},
					}))
				})
			})

			Context("multi-line secret", func() {
				var logLines string

				JustBeforeEach(func() {
					logLines = "{\nok123ok\nok456ok\nok789ok\n}\n"
					writer = delegate.Stderr()
					writtenBytes, writeErr = writer.Write([]byte(logLines))
					writer.(io.Closer).Close()
				})

				It("should be redacted", func() {
					Expect(writeErr).To(BeNil())
					Expect(writtenBytes).To(Equal(len(logLines)))
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "{\nok((redacted))ok\nok((redacted))ok\nok((redacted))ok\n}\n",
						Origin: event.Origin{
							Source: event.OriginSourceStderr,
							ID:     "some-plan-id",
						},
					}))
				})
			})

			Context("multi-line secret with random log chunk", func() {
				JustBeforeEach(func() {
					writer = delegate.Stderr()
					writtenBytes, writeErr = writer.Write([]byte("ok123ok\nok4"))
					writtenBytes, writeErr = writer.Write([]byte("56ok\nok789ok\n"))
					writer.(io.Closer).Close()
				})

				It("should be redacted", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(2))
					Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "ok((redacted))ok\n",
						Origin: event.Origin{
							Source: event.OriginSourceStderr,
							ID:     "some-plan-id",
						},
					}))
					Expect(fakeBuild.SaveEventArgsForCall(1)).To(Equal(event.Log{
						Time:    now.Unix(),
						Payload: "ok((redacted))ok\nok((redacted))ok\n",
						Origin: event.Origin{
							Source: event.OriginSourceStderr,
							ID:     "some-plan-id",
						},
					}))
				})
			})
		})
	})
})
