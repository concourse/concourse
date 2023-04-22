package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
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

		fakeBuild.NameReturns(db.CheckBuildName)
		fakeBuild.ResourceIDReturns(88)

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

				fakePipeline := new(dbfakes.FakePipeline)
				fakeBuild.PipelineReturns(fakePipeline, true, nil)
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
			)

			BeforeEach(func() {
				plan.Check.Resource = "some-resource"

				fakePipeline = new(dbfakes.FakePipeline)
				fakeBuild.PipelineReturns(fakePipeline, true, nil)

				fakePipeline.ResourceIDReturns(123, true, nil)
			})

			It("succeeds", func() {
				Expect(saveErr).ToNot(HaveOccurred())
			})

			It("finds or creates a scope for the resource", func() {
				Expect(fakeResourceConfig.FindOrCreateScopeCallCount()).To(Equal(1))
				resourceID := fakeResourceConfig.FindOrCreateScopeArgsForCall(0)
				Expect(*resourceID).To(Equal(123))
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
					fakePipeline.ResourceIDReturns(0, false, nil)
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

			Context("when the check plan is configured to skip interval", func() {
				BeforeEach(func() {
					plan.Check.SkipInterval = true
				})

				It("does not rate limit", func() {
					Expect(fakeRateLimiter.WaitCallCount()).To(Equal(0))
				})

				Context("when fail to get scope last start time", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.LastCheckReturns(db.LastCheck{}, errors.New("some-error"))
					})

					It("return the error", func() {
						Expect(runErr).To(HaveOccurred())
						Expect(runErr).To(Equal(errors.New("some-error")))
					})
				})

				Context("when from version is given", func() {
					BeforeEach(func() {
						plan.Check.FromVersion = atc.Version{"some": "version"}
					})

					It("returns true", func() {
						Expect(run).To(BeTrue())
					})
				})

				Context("when the interval is set to never on the plan", func() {
					It("does not exit early and attempts to fetch the last check value", func() {
						Expect(fakeResourceConfigScope.LastCheckCallCount()).To(Equal(2))
					})
				})

				Context("when the build create time earlier than last check start time", func() {
					BeforeEach(func() {
						fakeBuild.CreateTimeReturns(time.Now().Add(-5 * time.Second))
						fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
							StartTime: time.Now(),
							Succeeded: true,
						}, nil)
					})

					It("returns false", func() {
						Expect(run).To(BeFalse())
					})
				})

				Context("when the build create time after last check start time", func() {
					BeforeEach(func() {
						fakeBuild.CreateTimeReturns(time.Now().Add(5 * time.Second))
						fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
							StartTime: time.Now(),
							Succeeded: true,
						}, nil)
					})

					It("returns true", func() {
						Expect(run).To(BeTrue())
					})
				})

				Context("when the build fails with the create time earlier the last check start time", func() {
					BeforeEach(func() {
						fakeBuild.CreateTimeReturns(time.Now().Add(-5 * time.Second))
						fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
							StartTime: time.Now(),
							Succeeded: false,
						}, nil)
					})

					It("returns true", func() {
						Expect(run).To(BeTrue())
					})
				})
			})

			Context("when getting the last check end time errors", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{}, errors.New("oh no"))
				})

				It("returns an error", func() {
					Expect(runErr).To(HaveOccurred())
				})
			})

			Context("with an interval configured", func() {
				var interval time.Duration = time.Minute

				BeforeEach(func() {
					plan.Check.Interval = atc.CheckEvery{
						Interval: interval,
					}
				})

				Context("when the interval has not elapsed since the last check", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
							StartTime: now.Add(-(interval + 10)),
							EndTime:   now.Add(-(interval - 1)),
							Succeeded: true,
						}, nil)
					})

					It("returns false", func() {
						Expect(run).To(BeFalse())
					})

					It("never attempts to acquire the lock", func() {
						Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(0))
					})
				})

				Context("when the interval has elapsed since the last check", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
							StartTime: now.Add(-(interval + 10)),
							EndTime:   now.Add(-(interval + 1)),
							Succeeded: true,
						}, nil)
					})

					It("returns true", func() {
						Expect(run).To(BeTrue())
					})
				})

				Context("when the last check value gets updated after the first loop of attempting to acquire lock", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.LastCheckReturnsOnCall(0, db.LastCheck{
							StartTime: now.Add(-(interval + 10)),
							EndTime:   now.Add(-(interval + 1)),
							Succeeded: true,
						}, nil)
						fakeResourceConfigScope.LastCheckReturnsOnCall(1, db.LastCheck{
							StartTime: now.Add(time.Second).Add(-(interval + 10)),
							EndTime:   now.Add(time.Second).Add(-(interval - 1)),
							Succeeded: true,
						}, nil)
						fakeResourceConfigScope.AcquireResourceCheckingLockReturns(nil, false, nil)
						go fakeClock.WaitForWatcherAndIncrement(time.Second)
					})

					It("exits out of loop after last check gets updated in second loop and does not run", func() {
						Expect(run).To(BeFalse())
					})

					It("only attempts to acquire lock in first loop", func() {
						Expect(fakeResourceConfigScope.AcquireResourceCheckingLockCallCount()).To(Equal(1))
					})

					It("rechecks the last check value in both loops", func() {
						Expect(fakeResourceConfigScope.LastCheckCallCount()).To(Equal(2))
					})
				})
			})

			Context("when the interval is never", func() {
				BeforeEach(func() {
					plan.Check.Interval = atc.CheckEvery{
						Never: true,
					}
				})

				It("returns false", func() {
					Expect(run).To(BeFalse())
				})

				It("does not attempt to fetch the last check", func() {
					Expect(fakeResourceConfigScope.LastCheckCallCount()).To(Equal(0))
				})
			})
		})

		Context("when not running for a resource", func() {
			BeforeEach(func() {
				plan.Check.Resource = ""
				plan.Check.ResourceType = "some-resource-type"
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

			Context("when last check failed", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
						StartTime: now.Add(-10),
						EndTime:   now.Add(-1),
						Succeeded: false,
					}, nil)
				})

				It("returns true", func() {
					Expect(run).To(BeTrue())
				})
			})

			Context("when last check ended before build start time", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
						StartTime: now.Add(-10 * time.Second),
						EndTime:   now.Add(-time.Second),
						Succeeded: true,
					}, nil)
					fakeBuild.StartTimeReturns(now.Add(time.Second))
				})

				It("returns true", func() {
					Expect(run).To(BeTrue())
				})
			})

			Context("when last check succeeds after build starts", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
						StartTime: now.Add(-10 * time.Second),
						EndTime:   now.Add(-time.Second),
						Succeeded: true,
					}, nil)
					fakeBuild.StartTimeReturns(now.Add(-5 * time.Second))
				})

				It("returns false", func() {
					Expect(run).To(BeFalse())
				})
			})

			Context("when the checking interval has elapsed since the last check end time", func() {
				BeforeEach(func() {
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
						StartTime: now.Add(-6 * time.Minute),
						EndTime:   now.Add(-5 * time.Minute),
						Succeeded: true,
					}, nil)
					fakeBuild.StartTimeReturns(now)
				})

				It("returns true", func() {
					Expect(run).To(BeTrue())
				})
			})

			Context("when the checking interval has elapsed since the last check end time and it is a manual check", func() {
				BeforeEach(func() {
					plan.Check.SkipInterval = true
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
						StartTime: now.Add(-6 * time.Minute),
						EndTime:   now.Add(-5 * time.Minute),
						Succeeded: true,
					}, nil)
					fakeBuild.StartTimeReturns(now)
				})

				It("returns true", func() {
					Expect(run).To(BeTrue())
				})
			})

			Context("when the checking interval has not elapsed since the last check end time", func() {
				BeforeEach(func() {
					plan.Check.Interval.Interval = time.Hour
					plan.Check.SkipInterval = false
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
						StartTime: now,
						EndTime:   now,
						Succeeded: true,
					}, nil)
					fakeBuild.StartTimeReturns(now.Add(1 * time.Minute))
				})

				It("returns false", func() {
					Expect(run).To(BeFalse())
				})
			})

			Context("when the checking interval has not elapsed since the last check end time but it is a manual check", func() {
				BeforeEach(func() {
					plan.Check.Interval.Interval = time.Hour
					plan.Check.SkipInterval = true
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
						StartTime: now,
						EndTime:   now,
						Succeeded: true,
					}, nil)
					fakeBuild.StartTimeReturns(now.Add(1 * time.Minute))
				})

				It("returns true", func() {
					Expect(run).To(BeTrue())
				})
			})

			Context("when the last check is not successful and checking interval has not elapsed since the last check end time", func() {
				BeforeEach(func() {
					plan.Check.Interval.Interval = time.Hour
					fakeResourceConfigScope.LastCheckReturns(db.LastCheck{
						StartTime: now,
						EndTime:   now,
						Succeeded: false,
					}, nil)
					fakeBuild.StartTimeReturns(now.Add(1 * time.Minute))
				})

				It("returns true", func() {
					Expect(run).To(BeTrue())
				})
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
			)

			BeforeEach(func() {
				plan.Check.Resource = "some-resource"

				fakePipeline = new(dbfakes.FakePipeline)
				fakeBuild.PipelineReturns(fakePipeline, true, nil)
			})

			It("succeeds", func() {
				Expect(pointErr).ToNot(HaveOccurred())
			})

			It("sets the resource config scope", func() {
				Expect(fakePipeline.SetResourceConfigScopeForResourceCallCount()).To(Equal(1))
				name, scope := fakePipeline.SetResourceConfigScopeForResourceArgsForCall(0)
				Expect(name).To(Equal("some-resource"))
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
		})

		Context("when checking for a resource type", func() {
			var (
				fakePipeline *dbfakes.FakePipeline
			)

			BeforeEach(func() {
				plan.Check.ResourceType = "some-resource-type"

				fakePipeline = new(dbfakes.FakePipeline)
				fakeBuild.PipelineReturns(fakePipeline, true, nil)
			})

			It("succeeds", func() {
				Expect(pointErr).ToNot(HaveOccurred())
			})

			It("assigns the scope to the resource type", func() {
				Expect(fakePipeline.SetResourceConfigScopeForResourceTypeCallCount()).To(Equal(1))
				name, scope := fakePipeline.SetResourceConfigScopeForResourceTypeArgsForCall(0)
				Expect(name).To(Equal("some-resource-type"))
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
		})

		Context("when checking for a prototype", func() {
			var (
				fakePipeline *dbfakes.FakePipeline
			)

			BeforeEach(func() {
				plan.Check.Prototype = "some-prototype"

				fakePipeline = new(dbfakes.FakePipeline)
				fakeBuild.PipelineReturns(fakePipeline, true, nil)
			})

			It("succeeds", func() {
				Expect(pointErr).ToNot(HaveOccurred())
			})

			It("assigns the scope to the prototype", func() {
				Expect(fakePipeline.SetResourceConfigScopeForPrototypeCallCount()).To(Equal(1))
				name, scope := fakePipeline.SetResourceConfigScopeForPrototypeArgsForCall(0)
				Expect(name).To(Equal("some-prototype"))
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
		})
	})

	Describe("UpdateScopeLastCheckStartTime", func() {
		var (
			found        bool
			buildId      int
			nestedCheck  bool
			err          error
			expectedPlan json.RawMessage
		)

		BeforeEach(func() {
			fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(true, nil)
			expectedPlan = json.RawMessage(`{"id": 99}`)
			fakeBuild.IDReturns(9999)
			fakeBuild.PublicPlanReturns(&expectedPlan)
		})

		JustBeforeEach(func() {
			found, buildId, err = delegate.UpdateScopeLastCheckStartTime(fakeResourceConfigScope, nestedCheck)
		})

		Context("Resource check", func() {
			BeforeEach(func() {
				nestedCheck = false
			})

			Context("OnCheckBuildStart", func() {
				It("should call build.OnCheckBuildStart", func() {
					Expect(fakeBuild.OnCheckBuildStartCallCount()).To(Equal(1))
				})

				Context("when fails", func() {
					BeforeEach(func() {
						fakeBuild.OnCheckBuildStartReturns(fmt.Errorf("some-error"))
					})

					It("should fail", func() {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("some-error"))
						Expect(found).To(BeFalse())
					})
				})
			})

			Context("delegate to scope", func() {
				It("should succeeded", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildId).To(Equal(9999))
				})

				It("should delegate to scope", func() {
					Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))
					b, p := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
					Expect(b).To(Equal(9999))
					Expect(p).To(Equal(&expectedPlan))
				})

				It("Build id and public plan should not be nil", func() {
					buildId, publicPlan := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
					Expect(buildId).To(Equal(9999))
					Expect(publicPlan).To(Equal(&expectedPlan))
				})

				Context("when update fails", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(false, fmt.Errorf("some-error"))
					})

					It("should fail", func() {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("some-error"))
						Expect(found).To(BeFalse())
						Expect(buildId).To(Equal(9999))
					})
				})
			})
		})

		Context("Step nested check", func() {
			BeforeEach(func() {
				nestedCheck = true
			})

			It("should not call build.OnCheckBuildStart", func() {
				Expect(fakeBuild.OnCheckBuildStartCallCount()).To(Equal(0))
			})

			Context("delegate to scope", func() {
				It("should succeeded", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(buildId).To(Equal(0))
				})

				It("should delegate to scope", func() {
					Expect(fakeResourceConfigScope.UpdateLastCheckStartTimeCallCount()).To(Equal(1))
				})

				It("Build id and public plan should be nil", func() {
					buildId, publicPlan := fakeResourceConfigScope.UpdateLastCheckStartTimeArgsForCall(0)
					Expect(buildId).To(Equal(0))
					Expect(publicPlan).To(BeNil())
				})

				Context("when update fails", func() {
					BeforeEach(func() {
						fakeResourceConfigScope.UpdateLastCheckStartTimeReturns(false, fmt.Errorf("some-error"))
					})

					It("should fail", func() {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("some-error"))
						Expect(found).To(BeFalse())
						Expect(buildId).To(Equal(0))
					})
				})
			})
		})
	})

	Describe("UpdateScopeLastCheckEndTime", func() {
		var (
			found bool
			err   error
		)

		BeforeEach(func() {
			fakeResourceConfigScope.UpdateLastCheckEndTimeReturns(true, nil)
		})

		JustBeforeEach(func() {
			found, err = delegate.UpdateScopeLastCheckEndTime(fakeResourceConfigScope, true)
		})

		It("should succeeded", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

		})

		It("should delegate to scope", func() {
			Expect(fakeResourceConfigScope.UpdateLastCheckEndTimeCallCount()).To(Equal(1))
			s := fakeResourceConfigScope.UpdateLastCheckEndTimeArgsForCall(0)
			Expect(s).To(BeTrue())
		})

		Context("when update fails", func() {
			BeforeEach(func() {
				fakeResourceConfigScope.UpdateLastCheckEndTimeReturns(false, fmt.Errorf("some-error"))
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("some-error"))
				Expect(found).To(BeFalse())
			})
		})

	})
})
