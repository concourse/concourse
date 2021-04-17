package exec_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"go.opentelemetry.io/otel/oteltest"
	"go.opentelemetry.io/otel/trace"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckStep", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc

		planID                    atc.PlanID
		runState                  exec.RunState
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeResourceConfig        *dbfakes.FakeResourceConfig
		fakeResourceConfigScope   *dbfakes.FakeResourceConfigScope
		fakeDelegate              *execfakes.FakeCheckDelegate
		fakeDelegateFactory       *execfakes.FakeCheckDelegateFactory
		spanCtx                   context.Context
		defaultTimeout            = time.Hour

		fakePool        *execfakes.FakePool
		chosenWorker    *runtimetest.Worker
		chosenContainer *runtimetest.WorkerContainer

		fakeStdout, fakeStderr io.Writer

		stepMetadata      exec.StepMetadata
		checkStep         exec.Step
		checkPlan         atc.CheckPlan
		containerMetadata db.ContainerMetadata

		stepOk  bool
		stepErr error

		expectedOwner db.ContainerOwner
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		planID = "some-plan-id"

		runState = exec.NewRunState(noopStepper, vars.StaticVariables{"source-var": "super-secret-source"}, false)
		fakeDelegateFactory = new(execfakes.FakeCheckDelegateFactory)
		fakeDelegate = new(execfakes.FakeCheckDelegate)

		stepMetadata = exec.StepMetadata{
			TeamID:  345,
			BuildID: 678,
		}
		expectedOwner = db.NewBuildStepContainerOwner(stepMetadata.BuildID, planID, stepMetadata.TeamID)

		chosenWorker = runtimetest.NewWorker("worker").
			WithContainer(
				expectedOwner,
				runtimetest.NewContainer().WithProcess(
					runtime.ProcessSpec{
						Path: "/opt/resource/check",
					},
					runtimetest.ProcessStub{},
				),
				nil,
			)
		chosenContainer = chosenWorker.Containers[0]
		fakePool = new(execfakes.FakePool)
		fakePool.FindOrSelectWorkerReturns(chosenWorker, nil)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, tracing.NoopSpan)

		fakeStdout = bytes.NewBufferString("out")
		fakeDelegate.StdoutReturns(fakeStdout)

		fakeStderr = bytes.NewBufferString("err")
		fakeDelegate.StderrReturns(fakeStderr)

		containerMetadata = db.ContainerMetadata{}

		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeResourceConfig = new(dbfakes.FakeResourceConfig)
		fakeResourceConfig.IDReturns(501)
		fakeResourceConfig.OriginBaseResourceTypeReturns(&db.UsedBaseResourceType{
			ID:   502,
			Name: "some-base-type",
		})
		fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(fakeResourceConfig, nil)

		fakeResourceConfigScope = new(dbfakes.FakeResourceConfigScope)
		fakeDelegate.FindOrCreateScopeReturns(fakeResourceConfigScope, nil)

		fakeDelegateFactory.CheckDelegateReturns(fakeDelegate)

		checkPlan = atc.CheckPlan{
			Name:   "some-name",
			Type:   "some-base-type",
			Source: atc.Source{"some": "((source-var))"},
			VersionedResourceTypes: atc.VersionedResourceTypes{
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
			},
		}

		containerMetadata = db.ContainerMetadata{
			User: "test-user",
		}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		checkStep = exec.NewCheckStep(
			planID,
			checkPlan,
			stepMetadata,
			fakeResourceConfigFactory,
			containerMetadata,
			nil,
			fakePool,
			fakeDelegateFactory,
			defaultTimeout,
		)

		stepOk, stepErr = checkStep.Run(ctx, runState)
	})

	Context("with a reasonable configuration", func() {
		It("emits an Initializing event", func() {
			Expect(fakeDelegate.InitializingCallCount()).To(Equal(1))
		})

		Context("when not running", func() {
			BeforeEach(func() {
				fakeDelegate.WaitToRunReturns(nil, false, nil)
			})

			It("doesn't run the step and succeeds", func() {
				Expect(stepErr).ToNot(HaveOccurred())
				Expect(stepOk).To(BeTrue())

				Expect(chosenContainer.Processes).To(BeEmpty())
			})

			Context("when there is a latest version", func() {
				BeforeEach(func() {
					fakeVersion := new(dbfakes.FakeResourceConfigVersion)
					fakeVersion.VersionReturns(db.Version{"some": "latest-version"})
					fakeResourceConfigScope.LatestVersionReturns(fakeVersion, true, nil)
				})

				It("stores the latest version as the step result", func() {
					var val atc.Version
					Expect(runState.Result(planID, &val)).To(BeTrue())
					Expect(val).To(Equal(atc.Version{"some": "latest-version"}))
				})
			})

			Context("when there is no version", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LatestVersionReturns(nil, false, nil)
				})

				It("does not store a version", func() {
					var dst interface{}
					Expect(runState.Result(planID, &dst)).To(BeFalse())
				})
			})
		})

		Context("running", func() {
			var fakeLock *lockfakes.FakeLock

			var invokedResource resource.Resource

			BeforeEach(func() {
				fakeLock = new(lockfakes.FakeLock)
				fakeDelegate.WaitToRunReturns(fakeLock, true, nil)

				invokedResource = resource.Resource{}

				chosenContainer.ProcessDefs[0].Stub.Do = func(_ context.Context, p *runtimetest.Process) error {
					return json.NewDecoder(p.Stdin()).Decode(&invokedResource)
				}
			})

			Context("when given a from version", func() {
				BeforeEach(func() {
					checkPlan.FromVersion = atc.Version{"from": "version"}
				})

				It("constructs the resource with the version", func() {
					Expect(invokedResource.Version).To(Equal(checkPlan.FromVersion))
				})
			})

			Context("when not given a from version", func() {
				var fakeVersion *dbfakes.FakeResourceConfigVersion

				BeforeEach(func() {
					checkPlan.FromVersion = nil

					fakeVersion = new(dbfakes.FakeResourceConfigVersion)
					fakeVersion.VersionReturns(db.Version{"latest": "version"})
					fakeResourceConfigScope.LatestVersionStub = func() (db.ResourceConfigVersion, bool, error) {
						Expect(fakeDelegate.WaitToRunCallCount()).To(
							Equal(1),
							"should have gotten latest version after waiting, not before",
						)

						return fakeVersion, true, nil
					}
				})

				It("finds the latest version itself - it's a strong, independent check step who dont need no plan", func() {
					Expect(invokedResource.Version).To(Equal(atc.Version{"latest": "version"}))
				})
			})

			Describe("worker selection", func() {
				var ctx context.Context
				var workerSpec worker.Spec

				JustBeforeEach(func() {
					Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
					ctx, _, _, workerSpec, _, _ = fakePool.FindOrSelectWorkerArgsForCall(0)
				})

				It("doesn't enforce a timeout", func() {
					_, ok := ctx.Deadline()
					Expect(ok).To(BeFalse())
				})

				Describe("calls SelectWorker with the correct WorkerSpec", func() {
					It("with resource type", func() {
						Expect(workerSpec.ResourceType).To(Equal("some-base-type"))
					})

					It("with teamid", func() {
						Expect(workerSpec.TeamID).To(Equal(345))
					})

					Context("when the plan specifies tags", func() {
						BeforeEach(func() {
							checkPlan.Tags = atc.Tags{"some", "tags"}
						})

						It("sets them in the WorkerSpec", func() {
							Expect(workerSpec.Tags).To(Equal([]string{"some", "tags"}))
						})
					})
				})

				It("emits a SelectedWorker event", func() {
					Expect(fakeDelegate.SelectedWorkerCallCount()).To(Equal(1))
					_, workerName := fakeDelegate.SelectedWorkerArgsForCall(0)
					Expect(workerName).To(Equal("worker"))
				})

				Context("when selecting a worker fails", func() {
					BeforeEach(func() {
						fakePool.FindOrSelectWorkerReturns(nil, errors.New("nope"))
					})

					It("returns an err", func() {
						Expect(stepErr).To(MatchError(ContainSubstring("nope")))
					})
				})
			})

			Describe("running the check step", func() {
				Context("when the plan is for a resource", func() {
					BeforeEach(func() {
						checkPlan.Resource = "some-resource"

						expectedOwner = db.NewResourceConfigCheckSessionContainerOwner(
							501,
							502,
							db.ContainerOwnerExpiries{Min: 5 * time.Minute, Max: 1 * time.Hour},
						)

						chosenWorker = runtimetest.NewWorker("worker").
							WithContainer(
								expectedOwner,
								runtimetest.NewContainer().WithProcess(
									runtime.ProcessSpec{
										Path: "/opt/resource/check",
									},
									runtimetest.ProcessStub{},
								),
								nil,
							)
						chosenContainer = chosenWorker.Containers[0]
						fakePool.FindOrSelectWorkerReturns(chosenWorker, nil)
					})

					It("uses ResourceConfigCheckSessionOwner", func() {
						Expect(chosenContainer.Processes).To(HaveLen(1))
					})
				})

				Context("when the plan specifies a timeout", func() {
					BeforeEach(func() {
						checkPlan.Timeout = "1ms"

						chosenContainer.ProcessDefs[0].Stub.Do = func(ctx context.Context, _ *runtimetest.Process) error {
							select {
							case <-ctx.Done():
								return fmt.Errorf("wrapped: %w", ctx.Err())
							case <-time.After(100 * time.Millisecond):
								return nil
							}
						}
					})

					It("fails without error", func() {
						Expect(stepOk).To(BeFalse())
						Expect(stepErr).To(BeNil())
					})

					It("emits an Errored event", func() {
						Expect(fakeDelegate.ErroredCallCount()).To(Equal(1))
						_, status := fakeDelegate.ErroredArgsForCall(0)
						Expect(status).To(Equal(exec.TimeoutLogMessage))
					})
				})

				Context("uses containerspec", func() {
					It("with certs volume mount", func() {
						Expect(chosenContainer.Spec.CertsBindMount).To(BeTrue())
					})

					It("uses base type for image", func() {
						Expect(chosenContainer.Spec.ImageSpec).To(Equal(runtime.ImageSpec{
							ResourceType: "some-base-type",
						}))
					})

					Context("when tracing is enabled", func() {
						BeforeEach(func() {
							tracing.ConfigureTraceProvider(oteltest.NewTracerProvider())

							spanCtx, buildSpan := tracing.StartSpan(ctx, "build", nil)
							fakeDelegate.StartSpanReturns(spanCtx, buildSpan)

							chosenContainer.ProcessDefs[0].Stub.Do = func(ctx context.Context, _ *runtimetest.Process) error {
								defer GinkgoRecover()
								// Properly propagates span context
								Expect(tracing.FromContext(ctx)).To(Equal(buildSpan))
								return nil
							}
						})

						AfterEach(func() {
							tracing.Configured = false
						})

						It("populates the TRACEPARENT env var", func() {
							Expect(chosenContainer.Spec.Env).To(ContainElement(MatchRegexp(`TRACEPARENT=.+`)))
						})
					})
				})

				Context("when using a custom resource type", func() {
					var fetchedImageSpec runtime.ImageSpec

					BeforeEach(func() {
						checkPlan.Type = "some-custom-type"

						fetchedImageSpec = runtime.ImageSpec{
							ImageVolume: "some-volume",
						}

						fakeDelegate.FetchImageReturns(fetchedImageSpec, nil)
					})

					It("fetches the resource type image and uses it for the container", func() {
						Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))

						_, imageResource, types, privileged := fakeDelegate.FetchImageArgsForCall(0)

						By("fetching the type image")
						Expect(imageResource).To(Equal(atc.ImageResource{
							Name:    "some-custom-type",
							Type:    "another-custom-type",
							Source:  atc.Source{"some-custom": "((source-var))"},
							Params:  atc.Params{"some-custom": "((params-var))"},
							Version: atc.Version{"some-custom": "version"},
						}))

						By("excluding the type from the FetchImage call")
						Expect(types).To(Equal(atc.VersionedResourceTypes{
							{
								ResourceType: atc.ResourceType{
									Name:       "another-custom-type",
									Type:       "registry-image",
									Source:     atc.Source{"another-custom": "((source-var))"},
									Privileged: true,
								},
								Version: atc.Version{"another-custom": "version"},
							},
						}))

						By("not being privileged")
						Expect(privileged).To(BeFalse())
					})

					It("sets the bottom-most type in the worker spec", func() {
						Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
						_, _, _, workerSpec, _, _ := fakePool.FindOrSelectWorkerArgsForCall(0)

						Expect(workerSpec).To(Equal(worker.Spec{
							TeamID:       stepMetadata.TeamID,
							ResourceType: "registry-image",
						}))
					})

					It("sets the image spec in the container spec", func() {
						Expect(chosenContainer.Spec.ImageSpec).To(Equal(fetchedImageSpec))
					})

					Context("when the resource type is privileged", func() {
						BeforeEach(func() {
							checkPlan.Type = "another-custom-type"
						})

						It("fetches the image with privileged", func() {
							Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
							_, _, _, privileged := fakeDelegate.FetchImageArgsForCall(0)
							Expect(privileged).To(BeTrue())
						})
					})

					Context("when the plan configures tags", func() {
						BeforeEach(func() {
							checkPlan.Tags = atc.Tags{"plan", "tags"}
						})

						It("fetches using the tags", func() {
							Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
							_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
							Expect(imageResource.Tags).To(Equal(atc.Tags{"plan", "tags"}))
						})
					})

					Context("when the resource type configures tags", func() {
						BeforeEach(func() {
							taggedType, found := checkPlan.VersionedResourceTypes.Lookup("some-custom-type")
							Expect(found).To(BeTrue())

							taggedType.Tags = atc.Tags{"type", "tags"}

							newTypes := checkPlan.VersionedResourceTypes.Without("some-custom-type")
							newTypes = append(newTypes, taggedType)

							checkPlan.VersionedResourceTypes = newTypes
						})

						It("fetches using the type tags", func() {
							Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
							_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
							Expect(imageResource.Tags).To(Equal(atc.Tags{"type", "tags"}))
						})

						Context("when the plan ALSO configures tags", func() {
							BeforeEach(func() {
								checkPlan.Tags = atc.Tags{"plan", "tags"}
							})

							It("fetches using only the type tags", func() {
								Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
								_, imageResource, _, _ := fakeDelegate.FetchImageArgsForCall(0)
								Expect(imageResource.Tags).To(Equal(atc.Tags{"type", "tags"}))
							})
						})
					})
				})
			})

			Context("with tracing configured", func() {
				var buildSpan trace.Span

				BeforeEach(func() {
					tracing.ConfigureTraceProvider(oteltest.NewTracerProvider())

					spanCtx, buildSpan = tracing.StartSpan(context.Background(), "fake-operation", nil)
					fakeDelegate.StartSpanReturns(spanCtx, buildSpan)
				})

				AfterEach(func() {
					tracing.Configured = false
				})

				It("propagates span context to scope", func() {
					Expect(fakeResourceConfigScope.SaveVersionsCallCount()).To(Equal(1))
					spanContext, _ := fakeResourceConfigScope.SaveVersionsArgsForCall(0)
					traceID := buildSpan.SpanContext().TraceID().String()
					traceParent := spanContext.Get("traceparent")
					Expect(traceParent).To(ContainSubstring(traceID))
				})
			})

			Context("having RunCheckStep succeed", func() {
				BeforeEach(func() {
					chosenContainer.ProcessDefs[0].Stub.Output = []atc.Version{
						{"version": "1"},
						{"version": "2"},
					}
				})

				It("succeeds", func() {
					Expect(stepOk).To(BeTrue())
				})

				It("saves the versions to the config scope", func() {
					Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
					type_, source, types := fakeResourceConfigFactory.FindOrCreateResourceConfigArgsForCall(0)
					Expect(type_).To(Equal("some-base-type"))
					Expect(source).To(Equal(atc.Source{"some": "super-secret-source"}))
					Expect(types).To(Equal(atc.VersionedResourceTypes{
						{
							ResourceType: atc.ResourceType{
								Name:   "some-custom-type",
								Type:   "another-custom-type",
								Source: atc.Source{"some-custom": "super-secret-source"},

								// params don't need to be interpolated because it's used for
								// fetching, not constructing the resource config
								Params: atc.Params{"some-custom": "((params-var))"},
							},
							Version: atc.Version{"some-custom": "version"},
						},
						{
							ResourceType: atc.ResourceType{
								Name:       "another-custom-type",
								Type:       "registry-image",
								Source:     atc.Source{"another-custom": "super-secret-source"},
								Privileged: true,
							},
							Version: atc.Version{"another-custom": "version"},
						},
					}))

					Expect(fakeDelegate.FindOrCreateScopeCallCount()).To(Equal(1))
					config := fakeDelegate.FindOrCreateScopeArgsForCall(0)
					Expect(config).To(Equal(fakeResourceConfig))

					spanContext, versions := fakeResourceConfigScope.SaveVersionsArgsForCall(0)
					Expect(spanContext).To(Equal(db.SpanContext{}))
					Expect(versions).To(Equal([]atc.Version{
						{"version": "1"},
						{"version": "2"},
					}))
				})

				It("stores the latest version as the step result", func() {
					var val atc.Version
					Expect(runState.Result(planID, &val)).To(BeTrue())
					Expect(val).To(Equal(atc.Version{"version": "2"}))
				})

				It("emits a successful Finished event", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, succeeded := fakeDelegate.FinishedArgsForCall(0)
					Expect(succeeded).To(BeTrue())
				})

				Context("when no versions are returned", func() {
					BeforeEach(func() {
						chosenContainer.ProcessDefs[0].Stub.Output = []atc.Version{}
					})

					It("succeeds", func() {
						Expect(stepErr).ToNot(HaveOccurred())
						Expect(stepOk).To(BeTrue())
					})

					It("does not store a version", func() {
						var dst interface{}
						Expect(runState.Result(planID, &dst)).To(BeFalse())
					})
				})

				Context("before running the check", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.UpdateLastCheckStartTimeStub = func() (bool, error) {
							Expect(chosenContainer.Processes).To(BeEmpty())
							return true, nil
						}
					})

					It("updates the scope's last check start time", func() {
						Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))
						Expect(chosenContainer.Processes).To(HaveLen(1))
					})
				})

				Context("after saving", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.SaveVersionsStub = func(db.SpanContext, []atc.Version) error {
							Expect(fakeDelegate.PointToCheckedConfigCallCount()).To(BeZero())
							Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(0))
							return nil
						}
					})

					It("updates the scope's last check end time", func() {
						Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(1))
					})

					It("points the resource or resource type to the scope", func() {
						Expect(fakeResourceConfigScope.SaveVersionsCallCount()).To(Equal(1))
						Expect(fakeDelegate.PointToCheckedConfigCallCount()).To(Equal(1))
						scope := fakeDelegate.PointToCheckedConfigArgsForCall(0)
						Expect(scope).To(Equal(fakeResourceConfigScope))
					})
				})

				Context("after pointing the resource type to the scope", func() {
					BeforeEach(func() {
						fakeDelegate.PointToCheckedConfigStub = func(db.ResourceConfigScope) error {
							Expect(fakeLock.ReleaseCallCount()).To(Equal(0))
							return nil
						}
					})

					It("releases the lock", func() {
						Expect(fakeDelegate.PointToCheckedConfigCallCount()).To(Equal(1))
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})
			})

			Context("having the check step erroring", func() {
				BeforeEach(func() {
					chosenContainer.ProcessDefs[0].Stub.Err = "run-check-step-err"
				})

				It("errors", func() {
					Expect(stepErr).To(MatchError(ContainSubstring("run-check-step-err")))
				})

				It("points the resource or resource type to the scope", func() {
					// even though we failed to check, we should still point to the new
					// scope; it'd be kind of weird leave the resource pointing to the old
					// scope for a substantial config change that also happens to be
					// broken.
					Expect(fakeDelegate.PointToCheckedConfigCallCount()).To(Equal(1))
					scope := fakeDelegate.PointToCheckedConfigArgsForCall(0)
					Expect(scope).To(Equal(fakeResourceConfigScope))
				})

				It("updates the scope's last check end time", func() {
					Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(1))
				})

				// Finished is for script success/failure, whereas this is an error
				It("does not emit a Finished event", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(0))
				})
			})

			Context("with a script failure", func() {
				BeforeEach(func() {
					chosenContainer.ProcessDefs[0].Stub.ExitStatus = 42
				})

				It("does not error", func() {
					// don't return an error - the script output has already been
					// printed, and emitting an errored event would double it up
					Expect(stepErr).ToNot(HaveOccurred())
				})

				It("updates the scope's last check end time", func() {
					Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(1))
				})

				It("emits a failed Finished event", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, succeeded := fakeDelegate.FinishedArgsForCall(0)
					Expect(succeeded).To(BeFalse())
				})
			})

			Context("having SaveVersions failing", func() {
				var expectedErr error

				BeforeEach(func() {
					expectedErr = errors.New("save-versions-err")

					fakeResourceConfigScope.SaveVersionsReturns(expectedErr)
				})

				It("errors", func() {
					Expect(stepErr).To(HaveOccurred())
					Expect(errors.Is(stepErr, expectedErr)).To(BeTrue())
				})
			})
		})
	})

	Context("having credentials in the config", func() {
		BeforeEach(func() {
			checkPlan.Source = atc.Source{"some": "((missing-cred))"}
		})

		Context("having cred evaluation failing", func() {
			It("errors", func() {
				Expect(stepErr).To(HaveOccurred())
			})
		})
	})

	Context("having credentials in a resource type", func() {
		BeforeEach(func() {
			resTypes := atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Source: atc.Source{
							"some-custom": "((missing-cred))",
						},
					},
				},
			}

			checkPlan.VersionedResourceTypes = resTypes
		})

		Context("having cred evaluation failing", func() {
			It("errors", func() {
				Expect(stepErr).To(HaveOccurred())
			})
		})
	})

	Context("when a bogus timeout is given", func() {
		BeforeEach(func() {
			checkPlan.Timeout = "bogus"
		})

		It("fails miserably", func() {
			Expect(stepErr).To(MatchError("parse timeout: time: invalid duration \"bogus\""))
		})
	})
})
