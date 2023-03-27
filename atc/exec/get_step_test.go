package exec_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"github.com/onsi/gomega/gbytes"
	"go.opentelemetry.io/otel/oteltest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetStep", func() {
	var (
		ctx       context.Context
		cancel    func()
		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		fakePool        *execfakes.FakePool
		chosenWorker    *runtimetest.Worker
		chosenContainer *runtimetest.WorkerContainer
		getVolume       *runtimetest.Volume

		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeResourceCache        *dbfakes.FakeResourceCache

		fakeDelegate        *execfakes.FakeGetDelegate
		fakeDelegateFactory *execfakes.FakeGetDelegateFactory

		fakeLockFactory *lockfakes.FakeLockFactory

		spanCtx context.Context

		getPlan *atc.GetPlan

		runState           exec.RunState
		artifactRepository *build.Repository

		getStep exec.Step
		stepOk  bool
		stepErr error

		containerMetadata = db.ContainerMetadata{
			WorkingDirectory: resource.ResourcesDir("get"),
			PipelineID:       4567,
			Type:             db.ContainerTypeGet,
			StepName:         "some-step",
		}

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		planID = atc.PlanID("56")

		expectedOwner = db.NewBuildStepContainerOwner(stepMetadata.BuildID, planID, stepMetadata.TeamID)

		defaultGetTimeout time.Duration = 0
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		chosenWorker = runtimetest.NewWorker("worker").
			WithContainer(
				expectedOwner,
				runtimetest.NewContainer().WithProcess(
					runtime.ProcessSpec{
						ID:   "resource",
						Path: "/opt/resource/in",
						Args: []string{resource.ResourcesDir("get")},
					},
					runtimetest.ProcessStub{},
				),
				nil,
			)
		chosenContainer = chosenWorker.Containers[0]
		getVolume = runtimetest.NewVolume("get-volume")
		chosenContainer.Mounts = []runtime.VolumeMount{
			{
				Volume:    getVolume,
				MountPath: resource.ResourcesDir("get"),
			},
		}

		fakePool = new(execfakes.FakePool)
		fakePool.FindOrSelectWorkerReturns(chosenWorker, nil)

		fakeLockFactory = lockOnAttempt(1)

		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceCache = new(dbfakes.FakeResourceCache)

		runState = exec.NewRunState(noopStepper, vars.StaticVariables{
			"source-var": "super-secret-source",
			"params-var": "super-secret-params",
		}, false)
		artifactRepository = runState.ArtifactRepository()

		fakeDelegate = new(execfakes.FakeGetDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)
		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, tracing.NoopSpan)
		fakeDelegate.ContainerOwnerReturns(expectedOwner)

		fakeDelegateFactory = new(execfakes.FakeGetDelegateFactory)
		fakeDelegateFactory.GetDelegateReturns(fakeDelegate)

		getPlan = &atc.GetPlan{
			Name: "some-name",
			Type: "some-base-type",
			TypeImage: atc.TypeImage{
				BaseType: "some-base-type",
			},
			Resource: "some-resource",
			Source:   atc.Source{"some": "((source-var))"},
			Params:   atc.Params{"some": "((params-var))"},
			Version:  &atc.Version{"some": "version"},
		}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:  atc.PlanID(planID),
			Get: getPlan,
		}

		fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeResourceCache, nil)

		getStep = exec.NewGetStep(
			plan.ID,
			*plan.Get,
			stepMetadata,
			containerMetadata,
			fakeLockFactory,
			fakeResourceCacheFactory,
			nil,
			fakeDelegateFactory,
			fakePool,
			defaultGetTimeout,
		)

		stepOk, stepErr = getStep.Run(ctx, runState)
	})

	It("constructs the resource cache correctly", func() {
		_, typ, ver, source, params, imageResourceCache := fakeResourceCacheFactory.FindOrCreateResourceCacheArgsForCall(0)
		Expect(typ).To(Equal("some-base-type"))
		Expect(ver).To(Equal(atc.Version{"some": "version"}))
		Expect(source).To(Equal(atc.Source{"some": "super-secret-source"}))
		Expect(params).To(Equal(atc.Params{"some": "super-secret-params"}))
		Expect(imageResourceCache).To(BeNil())
	})

	Context("when using a dynamic version source", func() {
		versionPlanID := atc.PlanID("some-plan-id")

		BeforeEach(func() {
			getPlan.Version = nil
			getPlan.VersionFrom = &versionPlanID
		})

		Context("when the version exists in the build results", func() {
			var version atc.Version

			BeforeEach(func() {
				version = atc.Version{"foo": "bar"}
				runState.StoreResult(versionPlanID, version)
			})

			It("uses the version to create a resource cache", func() {
				Expect(fakeResourceCacheFactory.FindOrCreateResourceCacheCallCount()).To(Equal(1))
				_, _, ver, _, _, _ := fakeResourceCacheFactory.FindOrCreateResourceCacheArgsForCall(0)
				Expect(ver).To(Equal(version))
			})
		})

		Context("when the version does not exist in the build results", func() {
			It("can't resolve version and errors", func() {
				Expect(stepErr).To(Equal(exec.ErrResultMissing))
			})
		})
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

	It("runs with the correct ContainerSpec", func() {
		Expect(chosenContainer.Spec).To(Equal(
			&runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ResourceType: "some-base-type",
				},
				TeamID:         stepMetadata.TeamID,
				TeamName:       stepMetadata.TeamName,
				Type:           containerMetadata.Type,
				Env:            stepMetadata.Env(),
				Dir:            resource.ResourcesDir("get"),
				CertsBindMount: true,
			},
		))
	})

	Describe("retrieve from cache or run get step", func() {
		BeforeEach(func() {
			exec.GetResourceLockInterval = 10 * time.Millisecond
		})

		Context("when caching streamed volumes", func() {
			BeforeEach(func() {
				atc.EnableCacheStreamedVolumes = true
			})

			Context("when the cache is present on any worker", func() {
				var cacheVolume *runtimetest.Volume

				BeforeEach(func() {
					fakeLockFactory = neverLock()

					chosenContainer.ProcessDefs[0].Stub.Err = "should not run"

					cacheVolume = runtimetest.NewVolume("cache-volume")
					fakePool.FindResourceCacheVolumeReturns(cacheVolume, true, nil)
					fakeResourceCacheFactory.ResourceCacheMetadataReturns(db.ResourceConfigMetadataFields{
						{Name: "some", Value: "metadata"},
					}, nil)
				})

				It("succeeds", func() {
					Expect(stepErr).ToNot(HaveOccurred())
				})

				It("registers the volume as an artifact", func() {
					artifact, fromCache, found := artifactRepository.ArtifactFor(build.ArtifactName(getPlan.Name))
					Expect(artifact).To(Equal(cacheVolume))
					Expect(found).To(BeTrue())
					Expect(fromCache).To(BeTrue())
				})

				It("stores the resource cache as the step result", func() {
					var val interface{}
					Expect(runState.Result(planID, &val)).To(BeTrue())
					Expect(val).To(Equal(exec.GetResult{Name: getPlan.Name, ResourceCache: fakeResourceCache}))
				})

				It("doesn't select a worker", func() {
					Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(0))
				})

				It("doesn't initialize the get volume", func() {
					Expect(getVolume.ResourceCacheInitialized).To(BeFalse())
				})

				It("updates the resource version", func() {
					Expect(fakeDelegate.UpdateResourceVersionCallCount()).To(Equal(1))
				})

				It("does not update the resource cache metadata", func() {
					Expect(fakeResourceCacheFactory.UpdateResourceCacheMetadataCallCount()).To(Equal(0))
				})

				It("finishes with the correct version result", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, exitStatus, versionResult := fakeDelegate.FinishedArgsForCall(0)
					Expect(exitStatus).To(Equal(exec.ExitStatus(0)))
					Expect(versionResult.Metadata).To(Equal([]atc.MetadataField{
						{Name: "some", Value: "metadata"},
					}))
				})

				It("logs a message to stderr", func() {
					Expect(stderrBuf).To(gbytes.Say(`INFO.*found.*cache`))
				})
			})

			Context("when the cache is missing from all workers", func() {
				BeforeEach(func() {
					fakeLockFactory = lockOnAttempt(1)

					chosenContainer.ProcessDefs[0].Stub.Output = resource.VersionResult{
						Version:  atc.Version{"some": "version"},
						Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
					}
				})

				It("selects a worker", func() {
					Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
				})

				It("initializes the get volume", func() {
					Expect(getVolume.ResourceCacheInitialized).To(BeTrue())
				})

				It("updates the version", func() {
					Expect(fakeDelegate.UpdateResourceVersionCallCount()).To(Equal(1))
				})

				Context("when the get step is not for a named resource", func() {
					BeforeEach(func() {
						getPlan.Resource = ""
					})

					It("does not update the version", func() {
						Expect(fakeDelegate.UpdateResourceVersionCallCount()).To(Equal(0))
					})
				})

				It("updates the resource cache metadata", func() {
					Expect(fakeResourceCacheFactory.UpdateResourceCacheMetadataCallCount()).To(Equal(1))
				})

				It("finishes the step via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, status, info := fakeDelegate.FinishedArgsForCall(0)
					Expect(status).To(Equal(exec.ExitStatus(0)))
					Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
					Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
				})

				It("does not log any info messages", func() {
					Expect(stderrBuf).ToNot(gbytes.Say("INFO"))
				})

				Context("when the lock isn't initially acquired", func() {
					BeforeEach(func() {
						fakeLockFactory = lockOnAttempt(3)
					})

					It("logs a message to stderr", func() {
						Expect(stderrBuf).To(gbytes.Say(`INFO.*waiting.*lock`))
					})

					It("eventually selects a worker", func() {
						Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
					})
				})
			})
		})

		Context("when not caching streamed volumes", func() {
			BeforeEach(func() {
				atc.EnableCacheStreamedVolumes = false
			})

			AfterEach(func() {
				// always select a worker
				Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
			})

			Context("when the cache is present on the selected worker", func() {
				var cacheVolume *runtimetest.Volume

				BeforeEach(func() {
					fakeLockFactory = neverLock()

					chosenContainer.ProcessDefs[0].Stub.Err = "should not run"

					cacheVolume = runtimetest.NewVolume("cache-volume")
					fakePool.FindResourceCacheVolumeOnWorkerReturns(cacheVolume, true, nil)
					fakeResourceCacheFactory.ResourceCacheMetadataReturns(db.ResourceConfigMetadataFields{
						{Name: "some", Value: "metadata"},
					}, nil)
				})

				It("succeeds", func() {
					Expect(stepErr).ToNot(HaveOccurred())
				})

				It("logs a message to stderr", func() {
					Expect(stderrBuf).To(gbytes.Say(`INFO.*found.*cache`))
				})
			})

			Context("when the cache is missing from the selected worker", func() {
				BeforeEach(func() {
					fakeLockFactory = lockOnAttempt(1)

					chosenContainer.ProcessDefs[0].Stub.Output = resource.VersionResult{
						Version:  atc.Version{"some": "version"},
						Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
					}
				})

				It("succeeds", func() {
					Expect(stepErr).ToNot(HaveOccurred())
				})

				It("stores the resource cache as the step result", func() {
					var val interface{}
					Expect(runState.Result(planID, &val)).To(BeTrue())
					Expect(val).To(Equal(exec.GetResult{Name: getPlan.Name, ResourceCache: fakeResourceCache}))
				})

				It("finishes the step via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, status, info := fakeDelegate.FinishedArgsForCall(0)
					Expect(status).To(Equal(exec.ExitStatus(0)))
					Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
					Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
				})

				It("does not log any info messages", func() {
					Expect(stderrBuf).ToNot(gbytes.Say("INFO"))
				})

				Context("when the lock isn't initially acquired", func() {
					BeforeEach(func() {
						fakeLockFactory = lockOnAttempt(3)
					})

					It("succeeds", func() {
						Expect(stepErr).ToNot(HaveOccurred())
					})

					It("logs a message to stderr", func() {
						Expect(stderrBuf).To(gbytes.Say(`INFO.*waiting.*lock`))
					})
				})
			})
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

		It("get resource cache owner from delegate", func() {
			Expect(fakeDelegate.ResourceCacheUserCallCount()).To(Equal(1))
		})

		It("get container owner from delegate", func() {
			Expect(fakeDelegate.ContainerOwnerCallCount()).To(Equal(1))
		})

		It("emits a BeforeSelectWorker event", func() {
			Expect(fakeDelegate.BeforeSelectWorkerCallCount()).To(Equal(1))
		})

		It("calls SelectWorker with the correct WorkerSpec", func() {
			Expect(workerSpec).To(Equal(
				worker.Spec{
					ResourceType: "some-base-type",
					TeamID:       stepMetadata.TeamID,
				},
			))
		})

		It("emits a SelectedWorker event", func() {
			Expect(fakeDelegate.SelectedWorkerCallCount()).To(Equal(1))
			_, workerName := fakeDelegate.SelectedWorkerArgsForCall(0)
			Expect(workerName).To(Equal("worker"))
		})

		Context("when the plan specifies tags", func() {
			BeforeEach(func() {
				getPlan.Tags = atc.Tags{"some", "tags"}
			})

			It("sets them in the WorkerSpec", func() {
				Expect(workerSpec.Tags).To(Equal([]string{"some", "tags"}))
			})
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

	Context("when the plan specifies a timeout", func() {
		BeforeEach(func() {
			getPlan.Timeout = "1ms"

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

		Context("when the timeout is bogus", func() {
			BeforeEach(func() {
				getPlan.Timeout = "bogus"
			})

			It("fails miserably", func() {
				Expect(stepErr).To(MatchError("parse timeout: time: invalid duration \"bogus\""))
			})
		})
	})

	Context("when there is default get timeout", func() {
		BeforeEach(func() {
			defaultGetTimeout = time.Minute * 30
		})

		It("enforces it on the get", func() {
			t, ok := chosenContainer.ContextOfRun().Deadline()
			Expect(ok).To(BeTrue())
			Expect(t).To(BeTemporally("~", time.Now().Add(time.Minute*30), time.Minute))
		})
	})

	Context("when there is default get timeout and the plan specifies a timeout also", func() {
		BeforeEach(func() {
			defaultGetTimeout = time.Minute * 30
			getPlan.Timeout = "1h"
		})

		It("enforces the plan's timeout on the get step", func() {
			t, ok := chosenContainer.ContextOfRun().Deadline()
			Expect(ok).To(BeTrue())
			Expect(t).To(BeTemporally("~", time.Now().Add(time.Hour), time.Minute))
		})
	})

	Context("when using a custom resource type", func() {
		var (
			fetchedImageSpec       runtime.ImageSpec
			fakeImageResourceCache *dbfakes.FakeResourceCache
		)

		BeforeEach(func() {
			getPlan.TypeImage.GetPlan = &atc.Plan{
				ID: "1/image-get",
				Get: &atc.GetPlan{
					Name:   "some-custom-type",
					Type:   "another-custom-type",
					Source: atc.Source{"some-custom": "((source-var))"},
					Params: atc.Params{"some-custom": "((params-var))"},
				},
			}

			getPlan.TypeImage.CheckPlan = &atc.Plan{
				ID: "1/image-check",
				Check: &atc.CheckPlan{
					Name:   "some-custom-type",
					Type:   "another-custom-type",
					Source: atc.Source{"some-custom": "((source-var))"},
				},
			}

			getPlan.Type = "some-custom-type"
			getPlan.TypeImage.BaseType = "registry-image"

			fetchedImageSpec = runtime.ImageSpec{
				ImageArtifact: runtimetest.NewVolume("some-volume"),
			}

			fakeImageResourceCache = new(dbfakes.FakeResourceCache)
			fakeImageResourceCache.IDReturns(123)

			fakeDelegate.FetchImageReturns(fetchedImageSpec, fakeImageResourceCache, nil)
		})

		It("uses the same imageResourceCache to create the resourceCache", func() {
			_, _, _, _, _, rc := fakeResourceCacheFactory.FindOrCreateResourceCacheArgsForCall(0)
			Expect(rc.ID()).To(Equal(123))
		})

		It("fetches the resource type image and uses it for the container", func() {
			Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
			_, actualGetImagePlan, actualCheckImagePlan, privileged := fakeDelegate.FetchImageArgsForCall(0)
			Expect(actualGetImagePlan).To(Equal(*getPlan.TypeImage.GetPlan))
			Expect(actualCheckImagePlan).To(Equal(getPlan.TypeImage.CheckPlan))
			Expect(privileged).To(BeFalse())
		})

		It("sets the bottom-most type in the worker spec", func() {
			Expect(fakePool.FindOrSelectWorkerCallCount()).To(Equal(1))
			_, _, _, workerSpec, _, _ := fakePool.FindOrSelectWorkerArgsForCall(0)

			Expect(workerSpec).To(Equal(
				worker.Spec{
					TeamID:       stepMetadata.TeamID,
					ResourceType: "registry-image",
				},
			))
		})

		It("runs with the correct ImageSpec", func() {
			Expect(chosenContainer.Spec.ImageSpec).To(Equal(fetchedImageSpec))
		})

		Context("when the resource type is privileged", func() {
			BeforeEach(func() {
				getPlan.TypeImage.Privileged = true
			})

			It("fetches the image with privileged", func() {
				Expect(fakeDelegate.FetchImageCallCount()).To(Equal(1))
				_, _, _, privileged := fakeDelegate.FetchImageArgsForCall(0)
				Expect(privileged).To(BeTrue())
			})
		})
	})

	Context("when running the script returns an err", func() {
		disaster := errors.New("oh no")

		BeforeEach(func() {
			chosenContainer.ProcessDefs[0].Stub.Err = disaster.Error()
		})

		It("returns an err", func() {
			Expect(chosenContainer.RunningProcesses()).To(HaveLen(1))
			Expect(stepErr).To(MatchError(disaster))
			Expect(stepOk).To(BeFalse())
		})
	})

	Context("when the script succeeds", func() {
		BeforeEach(func() {
			chosenContainer.ProcessDefs[0].Stub.Output = resource.VersionResult{
				Version:  atc.Version{"some": "version"},
				Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
			}
		})

		It("registers the resulting artifact in the RunState.ArtifactRepository", func() {
			artifact, fromCache, found := artifactRepository.ArtifactFor(build.ArtifactName(getPlan.Name))
			Expect(artifact).To(Equal(getVolume))
			Expect(found).To(BeTrue())
			Expect(fromCache).To(BeFalse())
		})

		It("initializes the resource cache on the get volume", func() {
			Expect(getVolume.ResourceCacheInitialized).To(BeTrue())
		})

		It("stores the resource cache as the step result", func() {
			var val interface{}
			Expect(runState.Result(planID, &val)).To(BeTrue())
			Expect(val).To(Equal(exec.GetResult{Name: getPlan.Name, ResourceCache: fakeResourceCache}))
		})

		It("marks the step as succeeded", func() {
			Expect(stepOk).To(BeTrue())
		})

		It("finishes the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, status, info := fakeDelegate.FinishedArgsForCall(0)
			Expect(status).To(Equal(exec.ExitStatus(0)))
			Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
			Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
		})

		It("saves the version for the resource", func() {
			Expect(fakeDelegate.UpdateResourceVersionCallCount()).To(Equal(1))
		})

		It("does not return an err", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})
	})

	Context("when get script fails", func() {
		BeforeEach(func() {
			chosenContainer.ProcessDefs[0].Stub.ExitStatus = 1
		})

		It("does NOT mark the step as succeeded", func() {
			Expect(stepOk).To(BeFalse())
		})

		It("finishes the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, actualExitStatus, actualVersionResult := fakeDelegate.FinishedArgsForCall(0)
			Expect(actualExitStatus).ToNot(Equal(exec.ExitStatus(0)))
			Expect(actualVersionResult).To(BeZero())
		})

		It("does not return an err", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})

		It("does not update the resource version", func() {
			Expect(fakeDelegate.UpdateResourceVersionCallCount()).To(Equal(0))
		})
	})
})

func lockOnAttempt(attemptNumber int) *lockfakes.FakeLockFactory {
	fakeLockFactory := new(lockfakes.FakeLockFactory)
	fakeLockFactory.AcquireStub = func(lager.Logger, lock.LockID) (lock.Lock, bool, error) {
		attemptNumber--
		if attemptNumber <= 0 {
			return new(lockfakes.FakeLock), true, nil
		}
		return nil, false, nil
	}

	return fakeLockFactory
}

func neverLock() *lockfakes.FakeLockFactory {
	fakeLockFactory := new(lockfakes.FakeLockFactory)
	fakeLockFactory.AcquireStub = func(lager.Logger, lock.LockID) (lock.Lock, bool, error) {
		panic("expected lock to not be acquired")
	}
	return fakeLockFactory
}
