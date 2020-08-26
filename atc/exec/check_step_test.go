package exec_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"github.com/concourse/concourse/vars/varsfakes"
	"go.opentelemetry.io/otel/api/propagators"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/testtrace"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckStep", func() {

	var (
		ctx    context.Context
		cancel context.CancelFunc

		fakeRunState              *execfakes.FakeRunState
		fakeResourceFactory       *resourcefakes.FakeResourceFactory
		fakeResource              *resourcefakes.FakeResource
		fakeResourceConfigFactory *dbfakes.FakeResourceConfigFactory
		fakeResourceConfig        *dbfakes.FakeResourceConfig
		fakeResourceConfigScope   *dbfakes.FakeResourceConfigScope
		fakePool                  *workerfakes.FakePool
		fakeStrategy              *workerfakes.FakeContainerPlacementStrategy
		fakeDelegate              *execfakes.FakeCheckDelegate
		fakeClient                *workerfakes.FakeClient

		fakeStdout, fakeStderr io.Writer

		stepMetadata      exec.StepMetadata
		checkStep         exec.Step
		checkPlan         atc.CheckPlan
		containerMetadata db.ContainerMetadata

		err error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeRunState = new(execfakes.FakeRunState)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeResource = new(resourcefakes.FakeResource)
		fakePool = new(workerfakes.FakePool)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakeDelegate = new(execfakes.FakeCheckDelegate)
		fakeClient = new(workerfakes.FakeClient)

		fakeStdout = bytes.NewBufferString("out")
		fakeDelegate.StdoutReturns(fakeStdout)

		fakeStderr = bytes.NewBufferString("err")
		fakeDelegate.StderrReturns(fakeStderr)

		stepMetadata = exec.StepMetadata{}
		containerMetadata = db.ContainerMetadata{}

		fakeResourceFactory.NewResourceReturns(fakeResource)

		fakeResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeResourceConfig = new(dbfakes.FakeResourceConfig)
		fakeResourceConfig.IDReturns(501)
		fakeResourceConfig.OriginBaseResourceTypeReturns(&db.UsedBaseResourceType{
			ID:   502,
			Name: "some-base-resource-type",
		})
		fakeResourceConfigFactory.FindOrCreateResourceConfigReturns(fakeResourceConfig, nil)

		fakeResourceConfigScope = new(dbfakes.FakeResourceConfigScope)
		fakeDelegate.FindOrCreateScopeReturns(fakeResourceConfigScope, nil)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		planID := atc.PlanID("some-plan-id")

		checkStep = exec.NewCheckStep(
			planID,
			checkPlan,
			stepMetadata,
			fakeResourceFactory,
			fakeResourceConfigFactory,
			containerMetadata,
			fakeStrategy,
			fakePool,
			fakeDelegate,
			fakeClient,
		)

		err = checkStep.Run(ctx, fakeRunState)
	})

	Context("having credentials in the config", func() {
		BeforeEach(func() {
			checkPlan = atc.CheckPlan{
				Source:  atc.Source{"some": "((super-secret-source))"},
				Timeout: "1m",
			}
		})

		Context("having cred evaluation failing", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("creds-err")

				fakeVariables := new(varsfakes.FakeVariables)
				fakeVariables.GetReturns(nil, false, expectedErr)

				fakeDelegate.VariablesReturns(vars.NewBuildVariables(fakeVariables, false))
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})
		})
	})

	Context("having credentials in a resource type", func() {
		BeforeEach(func() {
			resTypes := atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Source: atc.Source{
							"some-custom": "((super-secret-source))",
						},
					},
				},
			}

			checkPlan = atc.CheckPlan{
				Source:                 atc.Source{"some": "super-secret-source"},
				Timeout:                "1m",
				VersionedResourceTypes: resTypes,
			}
		})

		Context("having cred evaluation failing", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("creds-err")

				fakeVariables := new(varsfakes.FakeVariables)
				fakeVariables.GetReturns(nil, false, expectedErr)

				fakeDelegate.VariablesReturns(vars.NewBuildVariables(fakeVariables, false))
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})
		})
	})

	Context("having a timeout that fails parsing", func() {
		BeforeEach(func() {
			checkPlan = atc.CheckPlan{
				Timeout: "th1s_15_n07_r1gh7",
			}
		})

		It("errors", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid duration"))
		})
	})

	Context("with a reasonable configuration", func() {
		var fakeLock *lockfakes.FakeLock

		BeforeEach(func() {
			fakeLock = new(lockfakes.FakeLock)
			fakeDelegate.WaitAndRunReturns(fakeLock, true, nil)

			resTypes := atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Type: "base-type",
						Source: atc.Source{
							"foo": "((bar))",
						},
					},
					Version: atc.Version{"some": "type-version"},
				},
			}

			checkPlan = atc.CheckPlan{
				Timeout:                "10s",
				Type:                   "resource-type",
				Source:                 atc.Source{"some": "source"},
				Tags:                   []string{"tag"},
				VersionedResourceTypes: resTypes,
			}

			containerMetadata = db.ContainerMetadata{
				User: "test-user",
			}

			stepMetadata = exec.StepMetadata{
				TeamID: 345,
			}

			fakeDelegate.VariablesReturns(vars.NewBuildVariables(vars.StaticVariables{"bar": "caz"}, false))
		})

		It("emits an Initializing event", func() {
			Expect(fakeDelegate.InitializingCallCount()).To(Equal(1))
		})

		Describe("running the check step", func() {
			var runCtx context.Context
			var owner db.ContainerOwner
			var containerSpec worker.ContainerSpec
			var workerSpec worker.WorkerSpec
			var strategy worker.ContainerPlacementStrategy
			var metadata db.ContainerMetadata
			var imageSpec worker.ImageFetcherSpec
			var processSpec runtime.ProcessSpec
			var startEventDelegate runtime.StartingEventDelegate
			var resource resource.Resource
			var timeout time.Duration

			JustBeforeEach(func() {
				Expect(fakeClient.RunCheckStepCallCount()).To(Equal(1), "check step should have run")
				runCtx, _, owner, containerSpec, workerSpec, strategy, metadata, imageSpec, processSpec, startEventDelegate, resource, timeout = fakeClient.RunCheckStepArgsForCall(0)
			})

			It("uses ResourceConfigCheckSessionOwner", func() {
				expected := db.NewResourceConfigCheckSessionContainerOwner(
					501,
					502,
					db.ContainerOwnerExpiries{Min: 5 * time.Minute, Max: 1 * time.Hour},
				)

				Expect(owner).To(Equal(expected))
			})

			It("passes the process spec", func() {
				Expect(processSpec).To(Equal(runtime.ProcessSpec{
					Path:         "/opt/resource/check",
					StdoutWriter: fakeStdout,
					StderrWriter: fakeStderr,
				}))
			})

			It("passes the delegate as the start event delegate", func() {
				Expect(startEventDelegate).To(Equal(fakeDelegate))
			})

			Context("uses containerspec", func() {
				It("with certs volume mount", func() {
					Expect(containerSpec.BindMounts).To(HaveLen(1))
					mount := containerSpec.BindMounts[0]

					_, ok := mount.(*worker.CertsVolumeMount)
					Expect(ok).To(BeTrue())
				})

				It("with imagespec w/ resource type", func() {
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: "resource-type",
					}))
				})

				It("with tags set", func() {
					Expect(containerSpec.Tags).To(ConsistOf("tag"))
				})

				It("with teamid set", func() {
					Expect(containerSpec.TeamID).To(Equal(345))
				})

				It("with env vars", func() {
					Expect(containerSpec.Env).To(ContainElement("BUILD_TEAM_ID=345"))
				})

				Context("when tracing is enabled", func() {
					BeforeEach(func() {
						tracing.ConfigureTraceProvider(testTraceProvider{})
					})

					AfterEach(func() {
						tracing.Configured = false
					})

					It("propagates span context to the worker client", func() {
						spanCtx, buildSpan := tracing.StartSpan(runCtx, "lidar", nil)
						span, ok := tracing.FromContext(spanCtx).(*testtrace.Span)
						Expect(ok).To(BeTrue(), "no testtrace.Span in context")
						Expect(span.ParentSpanID()).To(Equal(buildSpan.SpanContext().SpanID))
					})

					It("populates the TRACEPARENT env var", func() {
						Expect(containerSpec.Env).To(ContainElement(MatchRegexp(`TRACEPARENT=.+`)))
					})
				})
			})

			Context("uses workerspec", func() {
				It("with resource type", func() {
					Expect(workerSpec.ResourceType).To(Equal("resource-type"))
				})

				It("with tags", func() {
					Expect(workerSpec.Tags).To(ConsistOf("tag"))
				})

				It("with resource types", func() {
					Expect(workerSpec.ResourceTypes).To(HaveLen(1))
					interpolatedResourceType := workerSpec.ResourceTypes[0]

					Expect(interpolatedResourceType.Source).To(Equal(atc.Source{"foo": "caz"}))
				})

				It("with teamid", func() {
					Expect(workerSpec.TeamID).To(Equal(345))
				})
			})

			It("uses container placement strategy", func() {
				Expect(strategy).To(Equal(fakeStrategy))
			})

			It("uses container metadata", func() {
				Expect(metadata).To(Equal(containerMetadata))
			})

			It("uses interpolated resource types", func() {
				Expect(imageSpec.ResourceTypes).To(HaveLen(1))
				interpolatedResourceType := imageSpec.ResourceTypes[0]

				Expect(interpolatedResourceType.Source).To(Equal(atc.Source{"foo": "caz"}))
			})

			It("uses the timeout parsed", func() {
				Expect(timeout).To(Equal(10 * time.Second))
			})

			It("uses the resource created", func() {
				Expect(resource).To(Equal(fakeResource))
			})
		})

		Context("with tracing configured", func() {
			var span trace.Span

			BeforeEach(func() {
				tracing.ConfigureTraceProvider(&tracing.TestTraceProvider{})
				ctx, span = tracing.StartSpan(context.Background(), "fake-operation", nil)
			})

			AfterEach(func() {
				tracing.Configured = false
			})

			It("propagates span context to scope", func() {
				Expect(fakeResourceConfigScope.SaveVersionsCallCount()).To(Equal(1))
				spanContext, _ := fakeResourceConfigScope.SaveVersionsArgsForCall(0)
				traceID := span.SpanContext().TraceIDString()
				traceParent := spanContext.Get(propagators.TraceparentHeader)
				Expect(traceParent).To(ContainSubstring(traceID))
			})
		})

		Context("having RunCheckStep succeed", func() {
			BeforeEach(func() {
				fakeClient.RunCheckStepReturns(worker.CheckResult{
					Versions: []atc.Version{
						{"version": "1"},
						{"version": "2"},
					},
				}, nil)
			})

			It("saves the versions to the config scope", func() {
				Expect(fakeResourceConfigFactory.FindOrCreateResourceConfigCallCount()).To(Equal(1))
				type_, source, types := fakeResourceConfigFactory.FindOrCreateResourceConfigArgsForCall(0)
				Expect(type_).To(Equal("resource-type"))
				Expect(source).To(Equal(atc.Source{"some": "source"}))
				Expect(types).To(Equal(atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Type:   "base-type",
							Source: atc.Source{"foo": "caz"},
						},
						Version: atc.Version{"some": "type-version"},
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

			It("sets the check error to nil", func() {
				Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))
				err := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
				Expect(err).To(BeNil())
			})

			It("emits a successful Finished event", func() {
				Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
				_, succeeded := fakeDelegate.FinishedArgsForCall(0)
				Expect(succeeded).To(BeTrue())
			})

			Context("before running the check", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.UpdateLastCheckStartTimeStub = func() (bool, error) {
						Expect(fakeClient.RunCheckStepCallCount()).To(Equal(0))
						return true, nil
					}
				})

				It("updates the scope's last check start time", func() {
					Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))
					Expect(fakeClient.RunCheckStepCallCount()).To(Equal(1))
				})
			})

			Context("after saving", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.SaveVersionsStub = func(db.SpanContext, []atc.Version) error {
						Expect(fakeDelegate.PointToSavedVersionsCallCount()).To(BeZero())
						Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(0))
						return nil
					}
				})

				It("updates the scope's last check end time", func() {
					Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(1))
				})

				It("points the resource or resource type to the scope", func() {
					Expect(fakeResourceConfigScope.SaveVersionsCallCount()).To(Equal(1))
					Expect(fakeDelegate.PointToSavedVersionsCallCount()).To(Equal(1))
					scope := fakeDelegate.PointToSavedVersionsArgsForCall(0)
					Expect(scope).To(Equal(fakeResourceConfigScope))
				})
			})

			Context("after pointing the resource type to the scope", func() {
				BeforeEach(func() {
					fakeDelegate.PointToSavedVersionsStub = func(db.ResourceConfigScope) error {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(0))
						return nil
					}
				})

				It("releases the lock", func() {
					Expect(fakeDelegate.PointToSavedVersionsCallCount()).To(Equal(1))
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
			})
		})

		Context("having RunCheckStep erroring", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("run-check-step-err")
				fakeClient.RunCheckStepReturns(worker.CheckResult{}, expectedErr)
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})

			It("sets the check error", func() {
				Expect(fakeResourceConfigScope.SetCheckErrorCallCount()).To(Equal(1))
				err := fakeResourceConfigScope.SetCheckErrorArgsForCall(0)
				Expect(err).To(Equal(expectedErr))
			})

			// Finished is for script success/failure, whereas this is an error
			It("does not emit a Finished event", func() {
				Expect(fakeDelegate.FinishedCallCount()).To(Equal(0))
			})

			Context("with a script failure", func() {
				BeforeEach(func() {
					fakeClient.RunCheckStepReturns(worker.CheckResult{}, runtime.ErrResourceScriptFailed{
						ExitStatus: 42,
					})
				})

				It("does not error", func() {
					// don't return an error - the script output has already been
					// printed, and emitting an errored event would double it up
					Expect(err).ToNot(HaveOccurred())
				})

				It("emits a failed Finished event", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, succeeded := fakeDelegate.FinishedArgsForCall(0)
					Expect(succeeded).To(BeFalse())
				})
			})
		})

		Context("having SaveVersions failing", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("save-versions-err")

				fakeResourceConfigScope.SaveVersionsReturns(expectedErr)
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})
		})
	})
})
