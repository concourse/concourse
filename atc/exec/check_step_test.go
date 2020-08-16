package exec_test

import (
	"context"
	"errors"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
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

		fakeRunState        *execfakes.FakeRunState
		fakeResourceFactory *resourcefakes.FakeResourceFactory
		fakeResource        *resourcefakes.FakeResource
		fakePool            *workerfakes.FakePool
		fakeStrategy        *workerfakes.FakeContainerPlacementStrategy
		fakeDelegate        *execfakes.FakeCheckDelegate
		fakeClient          *workerfakes.FakeClient

		stepMetadata      exec.StepMetadata
		checkStep         *exec.CheckStep
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

		stepMetadata = exec.StepMetadata{}
		containerMetadata = db.ContainerMetadata{}

		fakeResourceFactory.NewResourceReturns(fakeResource)
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
				Source: atc.Source{"some": "((super-secret-source))"},
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
		BeforeEach(func() {
			resTypes := atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Source: atc.Source{
							"foo": "((bar))",
						},
					},
				},
			}

			checkPlan = atc.CheckPlan{
				Timeout:                "10s",
				Type:                   "resource-type",
				Tags:                   []string{"tag"},
				VersionedResourceTypes: resTypes,
			}

			containerMetadata = db.ContainerMetadata{
				User: "test-user",
			}

			stepMetadata = exec.StepMetadata{
				TeamID:             345,
				ResourceConfigID:   501,
				BaseResourceTypeID: 502,
			}

			fakeDelegate.VariablesReturns(vars.NewBuildVariables(vars.StaticVariables{"bar": "caz"}, false))
		})

		It("uses ResourceConfigCheckSessionOwner", func() {
			_, _, owner, _, _, _, _, _, _, _ := fakeClient.RunCheckStepArgsForCall(0)
			expected := db.NewResourceConfigCheckSessionContainerOwner(
				501,
				502,
				db.ContainerOwnerExpiries{Min: 5 * time.Minute, Max: 1 * time.Hour},
			)

			Expect(owner).To(Equal(expected))
		})

		Context("uses containerspec", func() {
			var containerSpec worker.ContainerSpec

			JustBeforeEach(func() {
				_, _, _, containerSpec, _, _, _, _, _, _ = fakeClient.RunCheckStepArgsForCall(0)
			})

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
				var buildSpan trace.Span

				BeforeEach(func() {
					tracing.ConfigureTraceProvider(testTraceProvider{})
					ctx, buildSpan = tracing.StartSpan(ctx, "lidar", nil)
				})

				It("propagates span context to the worker client", func() {
					ctx, _, _, _, _, _, _, _, _, _ := fakeClient.RunCheckStepArgsForCall(0)
					span, ok := tracing.FromContext(ctx).(*testtrace.Span)
					Expect(ok).To(BeTrue(), "no testtrace.Span in context")
					Expect(span.ParentSpanID()).To(Equal(buildSpan.SpanContext().SpanID))
				})

				It("populates the TRACEPARENT env var", func() {
					Expect(containerSpec.Env).To(ContainElement(MatchRegexp(`TRACEPARENT=.+`)))
				})

				AfterEach(func() {
					tracing.Configured = false
				})
			})
		})

		Context("uses workerspec", func() {
			var workerSpec worker.WorkerSpec

			JustBeforeEach(func() {
				_, _, _, _, workerSpec, _, _, _, _, _ = fakeClient.RunCheckStepArgsForCall(0)
			})

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
			_, _, _, _, _, strategy, _, _, _, _ := fakeClient.RunCheckStepArgsForCall(0)
			Expect(strategy).To(Equal(fakeStrategy))
		})

		It("uses container metadata", func() {
			_, _, _, _, _, _, metadata, _, _, _ := fakeClient.RunCheckStepArgsForCall(0)
			Expect(metadata).To(Equal(containerMetadata))
		})

		It("uses interpolated resource types", func() {
			_, _, _, _, _, _, _, imageSpec, _, _ := fakeClient.RunCheckStepArgsForCall(0)

			Expect(imageSpec.ResourceTypes).To(HaveLen(1))
			interpolatedResourceType := imageSpec.ResourceTypes[0]

			Expect(interpolatedResourceType.Source).To(Equal(atc.Source{"foo": "caz"}))
		})

		It("uses the timeout parsed", func() {
			_, _, _, _, _, _, _, _, timeout, _ := fakeClient.RunCheckStepArgsForCall(0)
			Expect(timeout).To(Equal(10 * time.Second))
		})

		It("uses the resource created", func() {
			_, _, _, _, _, _, _, _, _, resource := fakeClient.RunCheckStepArgsForCall(0)
			Expect(resource).To(Equal(fakeResource))
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

			It("propagates span context to delegate", func() {
				spanContext, _ := fakeDelegate.SaveVersionsArgsForCall(0)
				traceID := span.SpanContext().TraceIDString()
				traceParent := spanContext.Get(propagators.TraceparentHeader)
				Expect(traceParent).To(ContainSubstring(traceID))
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
		})

		Context("having SaveVersions failing", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("save-versions-err")

				fakeDelegate.SaveVersionsReturns(expectedErr)
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, expectedErr)).To(BeTrue())
			})
		})
	})
})
