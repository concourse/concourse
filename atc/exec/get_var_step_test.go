package exec_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	gocache "github.com/patrickmn/go-cache"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"github.com/concourse/concourse/vars/varsfakes"
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
		state      exec.RunState

		fakeGlobalSecrets      *credsfakes.FakeSecrets
		fakeVarSourceVariables *varsfakes.FakeVariables
		fakeVarSourcePool      *credsfakes.FakeVarSourcePool
		fakeManagerFactory     *credsfakes.FakeManagerFactory
		fakeSecrets            *credsfakes.FakeSecrets
		fakeLockFactory        *lockfakes.FakeLockFactory
		fakeLock               *lockfakes.FakeLock
		secretCacheConfig      creds.SecretCacheConfig
		varSourceConfigs       atc.VarSourceConfigs

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
		enableRedaction bool
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("var-step-test")
		ctx, cancel = context.WithCancel(context.Background())
		ctx = lagerctx.NewContext(ctx, testLogger)

		enableRedaction = true

		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegate.StdoutReturns(stdout)
		fakeDelegate.StderrReturns(stderr)

		fakeVarSourcePool = new(credsfakes.FakeVarSourcePool)
		fakeVarSourceVariables = new(varsfakes.FakeVariables)

		spanCtx = context.Background()
		fakeDelegate.StartSpanReturns(spanCtx, tracing.NoopSpan)
		fakeDelegate.VariablesReturns(fakeVarSourceVariables)

		fakeDelegateFactory = new(execfakes.FakeBuildStepDelegateFactory)
		fakeDelegateFactory.BuildStepDelegateReturns(fakeDelegate)

		getVarPlan = atc.GetVarPlan{
			Name: "some-source",
			Path: "some-var",
			Type: "some-type",
			Source: atc.Source{
				"some": "source",
			},
		}

		varSourceConfigs = atc.VarSourceConfigs{
			{
				Name: "some-source",
				Type: "some-type",
				Config: map[string]interface{}{
					"some": "source",
				},
			},
		}

		fakeGlobalSecrets = new(credsfakes.FakeSecrets)
		fakeManagerFactory = new(credsfakes.FakeManagerFactory)
		fakeSecrets = new(credsfakes.FakeSecrets)
		fakeVarSourcePool.FindOrCreateReturns(fakeSecrets, nil)
		fakeSecrets.NewSecretLookupPathsReturns(nil)

		fakeLock = new(lockfakes.FakeLock)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeLockFactory.AcquireReturns(fakeLock, true, nil)

		secretCacheConfig = creds.SecretCacheConfig{
			Enabled: false,
		}

		creds.Register("some-source", fakeManagerFactory)

		cache = gocache.New(0, 0)
	})

	AfterEach(func() {
		cancel()
	})

	Context("Single get_var step", func() {
		Context("when the source of the var is the local variables", func() {
			BeforeEach(func() {
				getVarPlan = atc.GetVarPlan{
					Name: ".",
					Path: "some-var",
					Type: "some-type",
					Source: atc.Source{
						"some": "source",
					},
				}

				state = exec.NewRunState(noopStepper, varSourceConfigs, enableRedaction)
			})

			JustBeforeEach(func() {
				step = exec.NewGetVarStep(
					planID,
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					secretCacheConfig,
					cache,
					fakeLockFactory,
					fakeVarSourcePool,
					fakeGlobalSecrets,
				)

				stepOk, stepErr = step.Run(ctx, state)
			})

			Context("when the var is stored in the local variables", func() {
				BeforeEach(func() {
					state.LocalVariables().SetVar(".", "some-var", "some-value", true)
				})

				It("gets the var from the local vars and stores it as the step result", func() {
					Expect(stepErr).ToNot(HaveOccurred())
					Expect(stepOk).To(BeTrue())

					var value string
					state.Result(planID, &value)
					Expect(value).To(Equal("some-value"))
				})

				It("does not cache the result", func() {
					hash, err := exec.HashVarIdentifier(getVarPlan.Path, getVarPlan.Type, getVarPlan.Source, 123)
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

			Context("when the var is not stored in the local variables", func() {
				It("fails with local var not found error", func() {
					Expect(stepErr).To(HaveOccurred())
					Expect(stepErr).To(Equal(exec.LocalVarNotFound{getVarPlan.Path}))
					Expect(stepOk).To(BeFalse())
				})
			})
		})

		Context("when the source of the var is empty", func() {
			BeforeEach(func() {
				getVarPlan = atc.GetVarPlan{
					Path: "some-var",
					Type: "some-type",
					Source: atc.Source{
						"some": "source",
					},
				}

				fakeSecrets.NewSecretLookupPathsReturns(nil)
				state = exec.NewRunState(noopStepper, varSourceConfigs, enableRedaction)
			})

			JustBeforeEach(func() {
				step = exec.NewGetVarStep(
					planID,
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					secretCacheConfig,
					cache,
					fakeLockFactory,
					fakeVarSourcePool,
					fakeGlobalSecrets,
				)

				stepOk, stepErr = step.Run(ctx, state)
			})

			Context("when the var is stored in the global credential manager", func() {
				BeforeEach(func() {
					fakeGlobalSecrets.GetReturns("some-value", nil, true, nil)
				})

				It("gets the var from the global vars and stores it as the step result", func() {
					Expect(stepErr).ToNot(HaveOccurred())
					Expect(stepOk).To(BeTrue())

					var value string
					state.Result(planID, &value)
					Expect(value).To(Equal("some-value"))
				})

				It("does not cache the result", func() {
					hash, err := exec.HashVarIdentifier(getVarPlan.Path, getVarPlan.Type, getVarPlan.Source, 123)
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

			Context("when the var is not stored in the global credential manager", func() {
				BeforeEach(func() {
					fakeGlobalSecrets.GetReturns(nil, nil, false, errors.New("not found!"))
				})

				It("fails with global var not found error", func() {
					Expect(stepErr).To(HaveOccurred())
					Expect(stepErr).To(Equal(exec.GlobalVarNotFoundError{getVarPlan.Path}))
					Expect(stepOk).To(BeFalse())
				})
			})
		})

		Context("when the source of the var is a var source", func() {
			BeforeEach(func() {
				getVarPlan = atc.GetVarPlan{
					Name: "some-source",
					Path: "some-var",
					Type: "some-type",
					Source: atc.Source{
						"some": "source",
					},
				}
			})

			JustBeforeEach(func() {
				state = exec.NewRunState(noopStepper, varSourceConfigs, enableRedaction)

				step = exec.NewGetVarStep(
					planID,
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					secretCacheConfig,
					cache,
					fakeLockFactory,
					fakeVarSourcePool,
					fakeGlobalSecrets,
				)

				stepOk, stepErr = step.Run(ctx, state)
			})

			Context("when caching is enabled", func() {
				BeforeEach(func() {
					secretCacheConfig.Enabled = true
				})

				Context("when the var is found in the cache", func() {
					BeforeEach(func() {
						varSourceConfigs = atc.VarSourceConfigs{
							{
								Name: "some-source",
								Type: "some-type",
								Config: map[string]interface{}{
									"some": "source",
								},
							},
						}

						fakeVarSourcePool.FindOrCreateReturns(fakeSecrets, nil)
						fakeSecrets.NewSecretLookupPathsReturns(nil)
						fakeSecrets.GetReturns("some-value", nil, true, nil)

						creds.Register("some-source", fakeManagerFactory)
					})

					It("uses the cached var and does not refetch", func() {
						Expect(stepErr).ToNot(HaveOccurred())
						Expect(stepOk).To(BeTrue())

						Expect(fakeSecrets.GetCallCount()).To(Equal(1))

						stepOk, stepErr = step.Run(ctx, state)
						Expect(stepErr).ToNot(HaveOccurred())
						Expect(stepOk).To(BeTrue())

						Expect(fakeSecrets.GetCallCount()).To(Equal(1), "should only fetch from Secrets the first Run")
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
										"some": "((var-source-config))",
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
							BeforeEach(func() {
								creds.Register("some-source", fakeManagerFactory)
							})

							It("will evaluate the source of the var source using a list of var source configs not including the evaluating var source", func() {
								Expect(fakeDelegate.VariablesCallCount()).To(Equal(1))
								_, actualVarSourceConfigs := fakeDelegate.VariablesArgsForCall(0)
								Expect(actualVarSourceConfigs).To(Equal(atc.VarSourceConfigs{
									{
										Name: "some-other-source",
										Type: "some-other-type",
										Config: map[string]interface{}{
											"some": "other-source",
										},
									},
								}))
							})

							Context("when the var source config evaluates successfully", func() {
								BeforeEach(func() {
									fakeVarSourceVariables.GetReturns("source", true, nil)
								})

								It("attempts to find or create the var source within the pool", func() {
									Expect(fakeVarSourcePool.FindOrCreateCallCount()).To(Equal(1))
								})

								Context("when finding or creating the var source pool succeeds", func() {
									BeforeEach(func() {
										fakeVarSourcePool.FindOrCreateReturns(fakeSecrets, nil)
									})

									It("fetches the secret lookup paths for the var source", func() {
										Expect(fakeSecrets.NewSecretLookupPathsCallCount()).To(Equal(1))
										teamName, pipelineName, allowRootPath := fakeSecrets.NewSecretLookupPathsArgsForCall(0)
										Expect(teamName).To(Equal(stepMetadata.TeamName))
										Expect(pipelineName).To(Equal(stepMetadata.PipelineName))
										Expect(allowRootPath).To(BeTrue())
									})

									Context("when there are no secret lookup paths configured", func() {
										BeforeEach(func() {
											fakeSecrets.NewSecretLookupPathsReturns(nil)
										})

										It("fetches the var using only the path", func() {
											Expect(fakeSecrets.GetCallCount()).To(Equal(1))
											Expect(fakeSecrets.GetArgsForCall(0)).To(Equal(getVarPlan.Path))
										})

										Context("when fetching the var from the secret succeeds with no expiry", func() {
											BeforeEach(func() {
												secretCacheConfig.Duration = 10 * time.Minute
												fakeSecrets.GetReturns("some-value", nil, true, nil)
											})

											It("caches the var using the default cache ttl", func() {
												hash, err := exec.HashVarIdentifier(getVarPlan.Path, getVarPlan.Type, getVarPlan.Source, 123)
												Expect(err).ToNot(HaveOccurred())

												value, expiration, found := cache.GetWithExpiration(hash)
												Expect(found).To(BeTrue())
												Expect(expiration).Should(BeTemporally("~", time.Now().Add(secretCacheConfig.Duration), time.Minute))
												Expect(value).To(Equal(exec.NewCacheEntry("some-value", nil, true)))
											})
										})

										Context("when fetching the var returns an expiry sooner than default ttl", func() {
											var expiry time.Time
											var nowTime time.Time

											BeforeEach(func() {
												secretCacheConfig.Duration = 10 * time.Minute

												nowTime = time.Now()
												expiry = nowTime.Add(1 * time.Minute)

												fakeSecrets.GetReturns("some-value", &expiry, true, nil)
											})

											It("caches the var using set expiry", func() {
												hash, err := exec.HashVarIdentifier(getVarPlan.Path, getVarPlan.Type, getVarPlan.Source, 123)
												Expect(err).ToNot(HaveOccurred())

												value, expiration, found := cache.GetWithExpiration(hash)
												Expect(found).To(BeTrue())
												Expect(expiration).ToNot(Equal(secretCacheConfig.Duration))
												Expect(expiration).Should(BeTemporally("~", expiry, time.Second))
												Expect(value).To(Equal(exec.NewCacheEntry("some-value", &expiry, true)))
											})
										})

										Context("when fetching the var returns an expiry later than default ttl", func() {
											var expiry time.Time
											var nowTime time.Time

											BeforeEach(func() {
												secretCacheConfig.Duration = 10 * time.Minute

												nowTime = time.Now()
												expiry = nowTime.Add(15 * time.Minute)

												fakeSecrets.GetReturns("some-value", &expiry, true, nil)
											})

											It("caches the var using set expiry", func() {
												hash, err := exec.HashVarIdentifier(getVarPlan.Path, getVarPlan.Type, getVarPlan.Source, 123)
												Expect(err).ToNot(HaveOccurred())

												value, expiration, found := cache.GetWithExpiration(hash)
												Expect(found).To(BeTrue())
												Expect(expiration).Should(BeTemporally("~", time.Now().Add(secretCacheConfig.Duration), time.Minute))
												Expect(value).To(Equal(exec.NewCacheEntry("some-value", &expiry, true)))
											})
										})

										Context("when the var is not found in the secrets", func() {
											BeforeEach(func() {
												secretCacheConfig.DurationNotFound = 10 * time.Minute

												fakeSecrets.GetReturns(nil, nil, false, nil)
											})

											It("caches the var with duration not found boolean", func() {
												hash, err := exec.HashVarIdentifier(getVarPlan.Path, getVarPlan.Type, getVarPlan.Source, 123)
												Expect(err).ToNot(HaveOccurred())

												value, expiration, found := cache.GetWithExpiration(hash)
												Expect(found).To(BeTrue())
												Expect(expiration).Should(BeTemporally("~", time.Now().Add(secretCacheConfig.DurationNotFound), time.Minute))
												Expect(value).To(Equal(exec.NewCacheEntry(nil, nil, false)))
											})
										})
									})

									Context("when there are multiple secret lookup paths", func() {
										BeforeEach(func() {
											lookupPath1 := creds.NewSecretLookupWithPrefix("test/")
											lookupPath2 := creds.NewSecretLookupWithPrefix("test-2/")
											fakeSecrets.NewSecretLookupPathsReturns([]creds.SecretLookupPath{lookupPath1, lookupPath2})

											fakeSecrets.GetReturnsOnCall(0, nil, nil, false, nil)
											fakeSecrets.GetReturnsOnCall(1, "some-value", nil, true, nil)
										})

										It("attempts to fetch the var using both paths", func() {
											Expect(fakeSecrets.GetCallCount()).To(Equal(2))
											Expect(fakeSecrets.GetArgsForCall(0)).To(Equal("test/" + getVarPlan.Path))
											Expect(fakeSecrets.GetArgsForCall(1)).To(Equal("test-2/" + getVarPlan.Path))
										})
									})

									Context("when vars are fetched successfully", func() {
										BeforeEach(func() {
											fakeSecrets.NewSecretLookupPathsReturns(nil)
											fakeSecrets.GetReturns("some-value", nil, true, nil)
										})

										It("tracks the var", func() {
											trackedVars := vars.TrackedVarsMap{}
											state.IterateInterpolatedCreds(trackedVars)
											Expect(trackedVars["some-source:some-var"]).To(Equal("some-value"))
										})

										It("releases the lock", func() {
											Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
										})
									})
								})

								Context("when finding or creating the var source pool fails", func() {
									BeforeEach(func() {
										fakeVarSourcePool.FindOrCreateReturns(nil, errors.New("whoops"))
									})

									It("fails the get var step", func() {
										Expect(stepErr).To(HaveOccurred())
										Expect(stepOk).To(BeFalse())
									})
								})
							})

							Context("when evaluating the var source config fails", func() {
								BeforeEach(func() {
									varSourceConfigs = atc.VarSourceConfigs{
										{
											Name: "some-source",
											Type: "some-type",
											Config: map[string]interface{}{
												"some": "((unknown-var))",
											},
										},
									}

									fakeVarSourceVariables.GetReturns(nil, false, nil)
								})

								It("fails the get var step", func() {
									Expect(stepErr).To(HaveOccurred())
									Expect(stepOk).To(BeFalse())
								})
							})
						})

						Context("when the manager factory does not exist", func() {
							BeforeEach(func() {
								getVarPlan.Name = "bogus"
							})

							It("fails to run the get var step", func() {
								Expect(stepErr).To(HaveOccurred())
								Expect(stepOk).To(BeFalse())
							})
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
							Expect(stepErr).To(HaveOccurred())
							Expect(stepOk).To(BeFalse())
							Expect(stepErr).To(Equal(vars.MissingSourceError{Name: "some-source:some-var", Source: "some-source"}))
						})
					})
				})

				Context("when there are fields in the get var plan", func() {
					BeforeEach(func() {
						getVarPlan.Fields = []string{"field"}
					})

					Context("when the field exists within the fetched var value", func() {
						var expectedValue map[string]interface{}
						BeforeEach(func() {
							expectedValue = map[string]interface{}{
								"field":       "some-field-value",
								"other-field": "other-value",
							}

							fakeSecrets.GetReturns(expectedValue, nil, true, nil)
						})

						It("stores the field value in the result", func() {
							Expect(stepErr).ToNot(HaveOccurred())
							Expect(stepOk).To(BeTrue())

							var value string
							state.Result(planID, &value)
							Expect(value).To(Equal("some-field-value"))
						})

						It("saves the entire value in the cache", func() {
							hash, err := exec.HashVarIdentifier(getVarPlan.Path, getVarPlan.Type, getVarPlan.Source, 123)
							Expect(err).ToNot(HaveOccurred())

							value, found := cache.Get(hash)
							Expect(found).To(BeTrue())
							Expect(value).To(Equal(exec.NewCacheEntry(expectedValue, nil, true)))
						})
					})

					Context("when the field is not found in the fetched var value", func() {
						BeforeEach(func() {
							fakeSecrets.GetReturns(map[string]interface{}{
								"other-field": "other-value",
							}, nil, true, nil)
						})

						It("fails to run the step", func() {
							Expect(stepErr).To(HaveOccurred())
							Expect(stepOk).To(BeFalse())
						})
					})

					Context("when the var is found in the cache", func() {
						BeforeEach(func() {
							hash, err := exec.HashVarIdentifier("some-var", "some-type", atc.Source{"some": "source"}, 123)
							Expect(err).ToNot(HaveOccurred())

							cache.Set(hash, exec.NewCacheEntry(map[string]interface{}{
								"field": "some-cached-value",
							}, nil, true), 1*time.Minute)
						})

						It("uses the value in the cache", func() {
							Expect(stepErr).ToNot(HaveOccurred())
							Expect(stepOk).To(BeTrue())

							Expect(fakeSecrets.GetCallCount()).To(Equal(0))

							var value string
							state.Result(planID, &value)
							Expect(value).To(Equal("some-cached-value"))
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

				It("does not fetch from the cache", func() {
					Expect(stepErr).ToNot(HaveOccurred())
					Expect(stepOk).To(BeTrue())

					Expect(fakeSecrets.GetCallCount()).To(Equal(1))

					var value string
					state.Result(planID, &value)
					Expect(value).To(Equal("some-value"))
				})
			})

			Context("when caching is not enabled and we have succeeded in fetching the var", func() {
				BeforeEach(func() {
					secretCacheConfig.Enabled = false

					fakeSecrets.GetReturns("some-value", nil, true, nil)
				})

				It("does not save the var in the cache", func() {
					Expect(stepErr).ToNot(HaveOccurred())
					Expect(stepOk).To(BeTrue())

					Expect(fakeSecrets.GetCallCount()).To(Equal(1))

					var value string
					state.Result(planID, &value)
					Expect(value).To(Equal("some-value"))

					cacheItems := cache.Items()
					Expect(cacheItems).To(BeEmpty())
				})
			})

			Context("when the var does not exist", func() {
				BeforeEach(func() {
					fakeSecrets.GetReturns(nil, nil, false, nil)
				})

				It("fails with var not found error", func() {
					Expect(stepErr).To(HaveOccurred())
					Expect(stepErr).To(Equal(exec.VarNotFoundError{Source: "some-source", Path: "some-var"}))
					Expect(stepOk).To(BeFalse())
				})

				It("releases the lock", func() {
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
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
					secretCacheConfig,
					cache,
					fakeLockFactory,
					fakeVarSourcePool,
					fakeGlobalSecrets,
				)

				step2 = exec.NewGetVarStep(
					"2",
					getVarPlan,
					stepMetadata,
					fakeDelegateFactory,
					secretCacheConfig,
					cache,
					fakeLockFactory,
					fakeVarSourcePool,
					fakeGlobalSecrets,
				)

				state = exec.NewRunState(noopStepper, varSourceConfigs, enableRedaction)
				fakeSecrets.GetReturns("some-value", nil, true, nil)
			})

			It("one of the two get_var's should wait for the lock to release", func() {
				step1Ok, step1Err := step1.Run(ctx, state)
				Expect(step1Err).ToNot(HaveOccurred())
				Expect(step1Ok).To(BeTrue())

				Expect(fakeSecrets.GetCallCount()).To(Equal(1))

				var value string
				state.Result("1", &value)
				Expect(value).To(Equal("some-value"))
				Expect(state.Result("2", &value)).To(BeFalse())

				step2Err := make(chan error)
				go func() {
					ok, err := step2.Run(ctx, state)
					Expect(ok).To(BeTrue())
					step2Err <- err
				}()

				Consistently(step2Err).ShouldNot(Receive())

				By("releasing the lock")
				acquired = false

				Eventually(step2Err).Should(Receive(BeNil()))
			})
		})
	})
})
