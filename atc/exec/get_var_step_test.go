package exec_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	gocache "github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel/api/trace"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("GetVarStep", func() {
	var (
		ctx        context.Context
		cancel     func()
		testLogger *lagertest.TestLogger

		fakeDelegate        *execfakes.FakeBuildStepDelegate
		fakeDelegateFactory *execfakes.FakeBuildStepDelegateFactory

		spanCtx context.Context

		getVarPlan atc.GetVarPlan
		state      *execfakes.FakeRunState

		fakeVarSourcePool  *credsfakes.FakeVarSourcePool
		fakeManagerFactory *credsfakes.FakeManagerFactory
		fakeManager        *credsfakes.FakeManager
		fakeSecretsFactory *credsfakes.FakeSecretsFactory
		fakeSecrets        *credsfakes.FakeSecrets
		fakeLockFactory    *lockfakes.FakeLockFactory
		fakeLock           *lockfakes.FakeLock
		secretCacheConfig  creds.SecretCacheConfig
		tracker            *vars.Tracker
		varSourceConfigs   atc.VarSourceConfigs

		cache *gocache.Cache

		step    exec.Step
		stepOk  bool
		stepErr error

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		stdout, stderr *gbytes.Buffer

		planID          atc.PlanID = "56"
		buildVariables  *build.Variables
		enableRedaction bool
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("var-step-test")
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		enableRedaction = true
		tracker = vars.NewTracker(enableRedaction)
		buildVariables = build.NewVariables(nil, tracker)
		state = new(execfakes.FakeRunState)
		state.LocalVariablesReturns(buildVariables)
		state.VarSourceConfigsReturns(varSourceConfigs)

		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StdoutReturns(stdout)
		fakeDelegate.StderrReturns(stderr)

		fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, trace.NoopSpan{})

		fakeDelegateFactory = new(execfakes.FakeBuildStepDelegateFactory)
		fakeDelegateFactory.BuildStepDelegateReturns(fakeDelegate)

		getVarPlan = atc.GetVarPlan{
			Name: "some-source-name",
			Path: "some-var",
			Type: "some-type",
			Source: atc.Source{
				"some": "source",
			},
		}

		fakeManagerFactory = new(credsfakes.FakeManagerFactory)
		fakeManager = new(credsfakes.FakeManager)
		fakeManagerFactory.NewInstanceReturns(fakeManager, nil)

		fakeSecretsFactory = new(credsfakes.FakeSecretsFactory)
		fakeSecrets = new(credsfakes.FakeSecrets)
		fakeSecretsFactory.NewSecretsReturns(fakeSecrets)
		fakeManager.NewSecretsFactoryReturns(fakeSecretsFactory, nil)

		fakeLock = new(lockfakes.FakeLock)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeLockFactory.AcquireReturns(fakeLock, true, nil)

		secretCacheConfig = creds.SecretCacheConfig{
			Enabled: false,
		}

		cache = gocache.New(0, 0)

		creds.Register("some-type", fakeManagerFactory)
	})

	AfterEach(func() {
		cancel()
	})

	Context("Single get_var step", func() {
		JustBeforeEach(func() {
			step = exec.NewGetVarStep(
				planID,
				getVarPlan,
				stepMetadata,
				fakeDelegateFactory,
				cache,
				fakeLockFactory,
				fakeVarSourcePool,
				secretCacheConfig,
			)

			stepOk, stepErr = step.Run(ctx, state)
		})

		Context("when the var is stored in the local variables", func() {
			BeforeEach(func() {
				getVarPlan = atc.GetVarPlan{
					Name: ".",
					Path: "some-var",
					Type: "some-type",
					Source: atc.Source{
						"some": "source",
					},
				}

				buildVariables.SetVar(".", "some-var", "some-value", true)
			})

			It("gets the var from the local vars and stores it as the step result", func() {
				Expect(stepOk).To(BeTrue())
				Expect(stepErr).ToNot(HaveOccurred())

				Expect(state.StoreResultCallCount()).To(Equal(1))
				actualPlanID, varValue := state.StoreResultArgsForCall(0)
				Expect(actualPlanID).To(Equal(planID))
				Expect(varValue).To(Equal("some-value"))
			})

			It("does not cache the result", func() {
				hash, err := exec.HashVarIdentifier(getVarPlan.Name, getVarPlan.Type, getVarPlan.Source, 123)
				Expect(err).ToNot(HaveOccurred())

				_, found := cache.Get(hash)
				Expect(found).To(BeFalse())
			})

			It("does not fetch from var source", func() {
				Expect(fakeSecrets.GetCallCount()).To(Equal(0))
			})

			It("releases the lock", func() {
				Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
			})
		})

		Context("when the var is stored in a var source", func() {
			BeforeEach(func() {
				getVarPlan = atc.GetVarPlan{
					Name: ".",
					Path: "some-var",
					Type: "some-type",
					Source: atc.Source{
						"some": "source",
					},
				}
			})

			Context("when caching is enabled", func() {
				BeforeEach(func() {
					secretCacheConfig.Enabled = true
				})

				Context("when the var is found in the cache", func() {
					BeforeEach(func() {
						hash, err := exec.HashVarIdentifier("some-var", "some-type", atc.Source{"some": "source"}, 123)
						Expect(err).ToNot(HaveOccurred())

						cache.Set(hash, "some-cached-value", 1*time.Minute)
					})

					It("uses the cached var and does not refetch", func() {
						Expect(stepOk).To(BeTrue())
						Expect(stepErr).ToNot(HaveOccurred())

						Expect(fakeSecrets.GetCallCount()).To(Equal(1))
						Expect(state.StoreResultCallCount()).To(Equal(1))

						actualPlanID, value := state.StoreResultArgsForCall(0)
						Expect(actualPlanID).To(Equal(planID))
						Expect(value).To(Equal("some-cached-value"))
					})

					It("tracks the var", func() {
						Expect(state.TrackCallCount()).To(Equal(1))
						actualRef, actualValue := state.TrackArgsForCall(0)
						Expect(actualRef).To(Equal(vars.Reference{
							Source: "some-source",
							Path:   "some-var",
						}))
						Expect(actualValue).To(Equal("some-cached-value"))
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})

				Context("when the var is not found in the cache", func() {
					Context("when the var source config is found", func() {
						BeforeEach(func() {
							varSourceConfigs = atc.VarSourceConfigs{
								{
									Name: "some-source",
									Type: "some-type",
									Config: map[string]interface{}{
										"some": "source",
									},
								},
								{
									Name: "some-other-source",
									Type: "some-other-type",
									Config: map[string]interface{}{
										"some": "other-source",
									},
								},
							}
						})

						Context("when the manager factory exists", func() {
							It("will evaluate the source of the var source using a list of var source configs not including the evaluating var source", func() {
							})
						})

						Context("when the manager factory does not exist", func() {
						})
					})

					Context("when the var source does not exist", func() {
						BeforeEach(func() {
							varSourceConfigs = atc.VarSourceConfigs{
								{
									Name: "some-other-source",
									Type: "some-other-type",
									Config: map[string]interface{}{
										"some": "other-source",
									},
								},
							}
						})

						It("fails with missing source error", func() {
							Expect(stepOk).To(BeFalse())
							Expect(stepErr).To(HaveOccurred())
							Expect(stepErr).To(Equal(vars.MissingSourceError{Name: "some-source:some-var", Source: "some-source"}))
						})
					})
				})
			})

			Context("when caching is not enabled", func() {
				BeforeEach(func() {
					secretCacheConfig.Enabled = false

					hash, err := exec.HashVarIdentifier("some-var", "some-type", atc.Source{"some": "source"}, 123)
					Expect(err).ToNot(HaveOccurred())

					cache.Set(hash, "some-cached-value", 1*time.Minute)

					fakeSecrets.GetReturns("some-value", nil, true, nil)
				})

				It("does not use the cache", func() {
					Expect(stepOk).To(BeTrue())
					Expect(stepErr).ToNot(HaveOccurred())

					Expect(fakeSecrets.GetCallCount()).To(Equal(1))
					Expect(state.StoreResultCallCount()).To(Equal(1))

					actualPlanID, value := state.StoreResultArgsForCall(0)
					Expect(actualPlanID).To(Equal(planID))
					Expect(value).To(Equal("some-value"))
				})
			})
		})

		Context("when reveal is true", func() {
			BeforeEach(func() {
				getVarPlan.Reveal = true
			})

			It("does not redact the var", func() {
				mapit := vars.TrackedVarsMap{}
				buildVariables.IterateInterpolatedCreds(mapit)
				Expect(mapit).ToNot(HaveKey("some-var"))
			})
		})

		Context("when the var does not exist", func() {
			BeforeEach(func() {
				fakeSecrets.GetReturns("", nil, false, nil)
			})

			It("fails with var not found error", func() {
				Expect(stepOk).To(BeFalse())
				Expect(stepErr).To(HaveOccurred())
				Expect(stepErr).To(Equal(exec.VarNotFoundError{Name: "some-var"}))
			})

			It("releases the lock", func() {
				Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
			})
		})
	})

	Context("Multiple get_var steps", func() {
		Context("when the same var is accessed by two separate get_var's in the same build", func() {
			var (
				acquired bool
				fakeLock *lockfakes.FakeLock
				step1    exec.Step
				step2    exec.Step
			)

			BeforeEach(func() {
				acquired = false

				fakeLock = new(lockfakes.FakeLock)
				fakeLock.ReleaseStub = func() error { return nil }

				fakeLockFactory.AcquireStub = func(lager.Logger, lock.LockID) (lock.Lock, bool, error) {
					if !acquired {
						acquired = true
						return fakeLock, true, nil
					} else {
						return nil, false, nil
					}
				}

				step1 = exec.NewGetVarStep(
					"1",
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					cache,
					fakeLockFactory,
				)

				step2 = exec.NewGetVarStep(
					"2",
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					cache,
					fakeLockFactory,
				)
			})

			It("one of the two get_var's should wait for the lock to release", func() {
				step1Ok, step1Err := step1.Run(ctx, state)
				Expect(step1Ok).To(BeTrue())
				Expect(step1Err).ToNot(HaveOccurred())

				go func() {
					step2Ok, step2Err := step2.Run(ctx, state)
					Expect(step2Ok).To(BeTrue())
					Expect(step2Err).ToNot(HaveOccurred())
				}()

				Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
				Expect(fakeSecrets.GetCallCount()).To(Equal(1))
				Expect(state.StoreResultCallCount()).To(Equal(1))

				By("releasing the lock")
				acquired = false

				Eventually(fakeLockFactory.AcquireCallCount).Should(Equal(2))

				By("does not fetch the var twice")
				Expect(fakeSecrets.GetCallCount()).To(Equal(1))
				Expect(state.StoreResultCallCount()).To(Equal(2))
			})
		})
	})
})
