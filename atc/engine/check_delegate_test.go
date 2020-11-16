package engine_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/enginefakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("CheckDelegate", func() {
	var (
		fakeBuild         *dbfakes.FakeBuild
		fakeClock         *fakeclock.FakeClock
		fakeRateLimiter   *enginefakes.FakeRateLimiter
		fakePolicyChecker *policyfakes.FakeChecker

		state exec.RunState

		now = time.Date(1991, 6, 3, 5, 30, 0, 0, time.UTC)

		plan     atc.Plan
		delegate exec.CheckDelegate

		fakeResourceConfig      *dbfakes.FakeResourceConfig
		fakeResourceConfigScope *dbfakes.FakeResourceConfigScope
	)

	BeforeEach(func() {
		fakeBuild = new(dbfakes.FakeBuild)
		fakeClock = fakeclock.NewFakeClock(now)
		fakeRateLimiter = new(enginefakes.FakeRateLimiter)
		credVars := vars.StaticVariables{
			"source-param": "super-secret-source",
			"git-key":      "{\n123\n456\n789\n}\n",
		}
		state = exec.NewRunState(noopStepper, credVars, true)

		plan = atc.Plan{
			ID:    "some-plan-id",
			Check: &atc.CheckPlan{},
		}

		fakePolicyChecker = new(policyfakes.FakeChecker)

		delegate = engine.NewCheckDelegate(fakeBuild, plan, state, fakeClock, fakeRateLimiter, fakePolicyChecker)

		fakeResourceConfig = new(dbfakes.FakeResourceConfig)
		fakeResourceConfigScope = new(dbfakes.FakeResourceConfigScope)
		fakeResourceConfig.FindOrCreateScopeReturns(fakeResourceConfigScope, nil)
	})

	Describe("FindOrCreateScope", func() {
		var saveErr error
		var scope db.ResourceConfigScope

		BeforeEach(func() {
			saveErr = nil
		})

		JustBeforeEach(func() {
			scope, saveErr = delegate.FindOrCreateScope(fakeResourceConfig)
		})

		Context("without a resource", func() {
			BeforeEach(func() {
				plan.Check.Resource = ""
			})

			It("succeeds", func() {
				Expect(saveErr).ToNot(HaveOccurred())
			})

			It("finds or creates a global scope", func() {
				Expect(fakeResourceConfig.FindOrCreateScopeCallCount()).To(Equal(1))
				resource := fakeResourceConfig.FindOrCreateScopeArgsForCall(0)
				Expect(resource).To(BeNil())
			})

			It("returns the scope", func() {
				Expect(scope).To(Equal(fakeResourceConfigScope))
			})
		})

		Context("with a resource", func() {
			var (
				fakePipeline *dbfakes.FakePipeline
				fakeResource *dbfakes.FakeResource
			)

			BeforeEach(func() {
				plan.Check.Resource = "some-resource"

				fakePipeline = new(dbfakes.FakePipeline)
				fakeBuild.PipelineReturns(fakePipeline, true, nil)

				fakeResource = new(dbfakes.FakeResource)
				fakePipeline.ResourceReturns(fakeResource, true, nil)
			})

			It("succeeds", func() {
				Expect(saveErr).ToNot(HaveOccurred())
			})

			It("looks up the resource on the pipeline", func() {
				Expect(fakePipeline.ResourceCallCount()).To(Equal(1))
				resourceName := fakePipeline.ResourceArgsForCall(0)
				Expect(resourceName).To(Equal("some-resource"))
			})

			It("finds or creates a scope for the resource", func() {
				Expect(fakeResourceConfig.FindOrCreateScopeCallCount()).To(Equal(1))
				resource := fakeResourceConfig.FindOrCreateScopeArgsForCall(0)
				Expect(resource).To(Equal(fakeResource))
			})

			It("returns the scope", func() {
				Expect(scope).To(Equal(fakeResourceConfigScope))
			})

			Context("when the pipeline is not found", func() {
				BeforeEach(func() {
					fakeBuild.PipelineReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(saveErr).To(HaveOccurred())
				})

				It("does not create a scope", func() {
					Expect(fakeResourceConfig.FindOrCreateScopeCallCount()).To(BeZero())
				})
			})

			Context("when the resource is not found", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(saveErr).To(HaveOccurred())
				})

				It("does not create a scope", func() {
					Expect(fakeResourceConfig.FindOrCreateScopeCallCount()).To(BeZero())
				})
			})
		})
	})

	Describe("WaitToRun", func() {
		var runLock lock.Lock
		var run bool
		var runErr error

		BeforeEach(func() {
			run = false
		})

		JustBeforeEach(func() {
			runLock, run, runErr = delegate.WaitToRun(context.TODO(), fakeResourceConfigScope)
		})

		Context("when the build is manually triggered", func() {
			BeforeEach(func() {
				fakeBuild.IsManuallyTriggeredReturns(true)
			})

			It("returns true", func() {
				Expect(run).To(BeTrue())
			})
		})

		Context("with an interval configured", func() {
			var interval time.Duration = time.Minute

			BeforeEach(func() {
				plan.Check.Interval = interval.String()
			})

			Context("when the interval has not elapsed since the last check", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LastCheckEndTimeReturns(now.Add(-(interval - 1)), nil)
				})

				It("returns false", func() {
					Expect(run).To(BeFalse())
				})
			})

			Context("when the interval has elapsed since the last check", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LastCheckEndTimeReturns(now.Add(-interval), nil)
				})

				It("returns true", func() {
					Expect(run).To(BeTrue())
				})
			})
		})

		Context("when running for a resource", func() {
			var fakeLock *lockfakes.FakeLock

			BeforeEach(func() {
				plan.Check.Resource = "some-resource"

				fakeLock = new(lockfakes.FakeLock)
				fakeResourceConfigScope.AcquireResourceCheckingLockReturns(fakeLock, true, nil)
			})

			It("returns a lock", func() {
				Expect(runLock).To(Equal(fakeLock))
			})

			Context("before acquiring the lock", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.AcquireResourceCheckingLockStub = func(lager.Logger) (lock.Lock, bool, error) {
						Expect(fakeRateLimiter.WaitCallCount()).To(Equal(1))
						return fakeLock, true, nil
					}
				})

				It("rate limits", func() {
					Expect(fakeRateLimiter.WaitCallCount()).To(Equal(1))
				})
			})

			Context("when the build is manually triggered", func() {
				BeforeEach(func() {
					fakeBuild.IsManuallyTriggeredReturns(true)
				})

				It("does not rate limit", func() {
					Expect(fakeRateLimiter.WaitCallCount()).To(Equal(0))
				})
			})

			Context("when getting the last check end time errors", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LastCheckEndTimeReturns(time.Time{}, errors.New("oh no"))
				})

				It("returns an error", func() {
					Expect(runErr).To(HaveOccurred())
				})

				It("releases the lock", func() {
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
			})

			Context("with an interval configured", func() {
				var interval time.Duration = time.Minute

				BeforeEach(func() {
					plan.Check.Interval = interval.String()
				})

				Context("when the interval has not elapsed since the last check", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.LastCheckEndTimeReturns(now.Add(-(interval - 1)), nil)
					})

					It("returns false", func() {
						Expect(run).To(BeFalse())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})
			})
		})

		Context("when not running for a resource", func() {
			BeforeEach(func() {
				plan.Check.Resource = ""
			})

			It("does not rate limit", func() {
				Expect(fakeRateLimiter.WaitCallCount()).To(Equal(0))
			})

			It("does not acquire a lock", func() {
				Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(0))
			})

			It("returns a no-op lock", func() {
				Expect(runLock).To(Equal(lock.NoopLock{}))
			})
		})
	})

	Describe("PointToCheckedConfig", func() {
		var pointErr error

		BeforeEach(func() {
			pointErr = nil
		})

		JustBeforeEach(func() {
			pointErr = delegate.PointToCheckedConfig(fakeResourceConfigScope)
		})

		Context("when not checking for a resource or resource type", func() {
			It("succeeds", func() {
				Expect(pointErr).ToNot(HaveOccurred())
			})
		})

		Context("when checking for a resource", func() {
			var (
				fakePipeline *dbfakes.FakePipeline
				fakeResource *dbfakes.FakeResource
			)

			BeforeEach(func() {
				plan.Check.Resource = "some-resource"

				fakePipeline = new(dbfakes.FakePipeline)
				fakeBuild.PipelineReturns(fakePipeline, true, nil)

				fakeResource = new(dbfakes.FakeResource)
				fakePipeline.ResourceReturns(fakeResource, true, nil)
			})

			It("succeeds", func() {
				Expect(pointErr).ToNot(HaveOccurred())
			})

			It("looks up the resource on the pipeline", func() {
				Expect(fakePipeline.ResourceCallCount()).To(Equal(1))
				resourceName := fakePipeline.ResourceArgsForCall(0)
				Expect(resourceName).To(Equal("some-resource"))
			})

			It("sets the resource config scope", func() {
				Expect(fakeResource.SetResourceConfigScopeCallCount()).To(Equal(1))
				scope := fakeResource.SetResourceConfigScopeArgsForCall(0)
				Expect(scope).To(Equal(fakeResourceConfigScope))
			})

			Context("when the pipeline is not found", func() {
				BeforeEach(func() {
					fakeBuild.PipelineReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(pointErr).To(HaveOccurred())
				})
			})

			Context("when the resource is not found", func() {
				BeforeEach(func() {
					fakePipeline.ResourceReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(pointErr).To(HaveOccurred())
				})
			})
		})

		Context("when checking for a resource type", func() {
			var (
				fakePipeline     *dbfakes.FakePipeline
				fakeResourceType *dbfakes.FakeResourceType
			)

			BeforeEach(func() {
				plan.Check.ResourceType = "some-resource-type"

				fakePipeline = new(dbfakes.FakePipeline)
				fakeBuild.PipelineReturns(fakePipeline, true, nil)

				fakeResourceType = new(dbfakes.FakeResourceType)
				fakePipeline.ResourceTypeReturns(fakeResourceType, true, nil)
			})

			It("succeeds", func() {
				Expect(pointErr).ToNot(HaveOccurred())
			})

			It("looks up the resource type on the pipeline", func() {
				Expect(fakePipeline.ResourceTypeCallCount()).To(Equal(1))
				resourceName := fakePipeline.ResourceTypeArgsForCall(0)
				Expect(resourceName).To(Equal("some-resource-type"))
			})

			It("assigns the scope to the resource type", func() {
				Expect(fakeResourceType.SetResourceConfigScopeCallCount()).To(Equal(1))

				scope := fakeResourceType.SetResourceConfigScopeArgsForCall(0)
				Expect(scope).To(Equal(fakeResourceConfigScope))
			})

			Context("when the pipeline is not found", func() {
				BeforeEach(func() {
					fakeBuild.PipelineReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(pointErr).To(HaveOccurred())
				})
			})

			Context("when the resource is not found", func() {
				BeforeEach(func() {
					fakePipeline.ResourceTypeReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(pointErr).To(HaveOccurred())
				})
			})
		})
	})
})
