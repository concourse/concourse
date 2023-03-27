package engine_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("SetPipelineStepDelegate", func() {
	var (
		logger                *lagertest.TestLogger
		fakeBuild             *dbfakes.FakeBuild
		fakeClock             *fakeclock.FakeClock
		fakePolicyChecker     *policyfakes.FakeChecker
		fakePolicyCheckResult *policyfakes.FakePolicyCheckResult

		state exec.RunState

		now      = time.Date(1991, 6, 3, 5, 30, 0, 0, time.UTC)
		delegate exec.SetPipelineStepDelegate
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.TeamNameReturns("some-team")
		fakeBuild.PipelineNameReturns("some-pipeline")
		fakeClock = fakeclock.NewFakeClock(now)
		credVars := vars.StaticVariables{
			"source-param": "super-secret-source",
			"git-key":      "{\n123\n456\n789\n}\n",
		}
		state = exec.NewRunState(noopStepper, credVars, true)

		fakePolicyCheckResult = new(policyfakes.FakePolicyCheckResult)
		fakePolicyChecker = new(policyfakes.FakeChecker)
		fakePolicyChecker.CheckReturns(fakePolicyCheckResult, nil)

		delegate = engine.NewSetPipelineStepDelegate(fakeBuild, "some-plan-id", state, fakeClock, fakePolicyChecker)
	})

	Describe("SetPipelineChanged", func() {
		JustBeforeEach(func() {
			delegate.SetPipelineChanged(logger, true)
		})

		It("saves an event", func() {
			Expect(fakeBuild.SaveEventCallCount()).To(Equal(1))
			Expect(fakeBuild.SaveEventArgsForCall(0)).To(Equal(event.SetPipelineChanged{
				Origin:  event.Origin{ID: event.OriginID("some-plan-id")},
				Changed: true,
			}))
		})
	})

	Describe("CheckRunSetPipelinePolicy", func() {
		var checkErr error
		var pipelineConfig atc.Config
		JustBeforeEach(func() {
			pipelineConfig = atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name: "g1",
					},
					{
						Name: "g2",
					},
				},
			}

			checkErr = delegate.CheckRunSetPipelinePolicy(&pipelineConfig)
		})

		Context("when the action does not need to be checked", func() {
			BeforeEach(func() {
				fakePolicyChecker.ShouldCheckActionReturns(false)
			})

			It("should succeed", func() {
				Expect(checkErr).ToNot(HaveOccurred())
			})

			It("should not check policy", func() {
				Expect(fakePolicyChecker.CheckCallCount()).To(Equal(0))
			})
		})

		Context("when the action needs to be checked", func() {
			BeforeEach(func() {
				fakePolicyChecker.ShouldCheckActionReturns(true)
			})

			It("should check policy", func() {
				Expect(fakePolicyChecker.CheckCallCount()).To(Equal(1))

				input := fakePolicyChecker.CheckArgsForCall(0)
				Expect(input).To(Equal(policy.PolicyCheckInput{
					Action:   policy.ActionRunSetPipeline,
					Team:     "some-team",
					Pipeline: "some-pipeline",
					Data:     &pipelineConfig,
				}))
			})

			Context("when policy check fails", func() {
				BeforeEach(func() {
					fakePolicyChecker.CheckReturns(nil, errors.New("some-error"))
				})

				It("should fail", func() {
					Expect(checkErr).To(HaveOccurred())
					Expect(checkErr.Error()).To(Equal("policy check: some-error"))
				})
			})

			Context("when policy check not pass", func() {
				BeforeEach(func() {
					fakePolicyCheckResult.AllowedReturns(false)
					fakePolicyCheckResult.MessagesReturns([]string{"reasonA", "reasonB"})
				})

				Context("when should block", func() {
					BeforeEach(func() {
						fakePolicyCheckResult.ShouldBlockReturns(true)
					})

					It("should fail", func() {
						Expect(checkErr).To(HaveOccurred())
						Expect(checkErr.Error()).To(ContainSubstring("policy check failed"))
						Expect(checkErr.Error()).To(ContainSubstring("reasonA"))
						Expect(checkErr.Error()).To(ContainSubstring("reasonB"))
					})
				})

				Context("when should not block", func() {
					BeforeEach(func() {
						fakePolicyCheckResult.ShouldBlockReturns(false)
					})

					It("should succeed", func() {
						Expect(checkErr).ToNot(HaveOccurred())
					})

					It("should log warning", func() {
						e := fakeBuild.SaveEventArgsForCall(0)
						Expect(e.EventType()).To(Equal(event.EventTypeLog))
						Expect(e.(event.Log).Origin).To(Equal(event.Origin{
							ID:     "some-plan-id",
							Source: event.OriginSourceStderr,
						}))
						Expect(e.(event.Log).Payload).To(ContainSubstring("policy check failed"))
						Expect(e.(event.Log).Payload).To(ContainSubstring("reasonA"))
						Expect(e.(event.Log).Payload).To(ContainSubstring("reasonB"))

						e = fakeBuild.SaveEventArgsForCall(1)
						Expect(e.EventType()).To(Equal(event.EventTypeLog))
						Expect(e.(event.Log).Origin).To(Equal(event.Origin{
							ID:     "some-plan-id",
							Source: event.OriginSourceStderr,
						}))
						Expect(e.(event.Log).Payload).To(ContainSubstring("WARNING: unblocking from the policy check failure for soft enforcement"))
					})
				})
			})

			Context("policy check passes", func() {
				BeforeEach(func() {
					fakePolicyCheckResult.AllowedReturns(true)
				})

				It("should succeed", func() {
					Expect(checkErr).ToNot(HaveOccurred())
				})

				It("should not log warning", func() {
					Expect(fakeBuild.SaveEventCallCount()).To(Equal(0))
				})
			})
		})
	})
})
