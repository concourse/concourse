package policy_test

import (
	"errors"

	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Policy checker", func() {

	var (
		checker *policy.Checker
		filter  policy.Filter
		err     error
	)

	BeforeEach(func() {
		filter = policy.Filter{
			HttpMethods:   []string{"POST", "PUT"},
			Actions:       []string{"do_1", "do_2"},
			ActionsToSkip: []string{"skip_1", "skip_2"},
		}

		fakeAgent = new(policyfakes.FakeAgent)
		fakeAgentFactory.NewAgentReturns(fakeAgent, nil)
	})

	JustBeforeEach(func() {
		checker, err = policy.Initialize(testLogger, "some-cluster", "some-version", filter)
	})

	// fakeAgent is configured in BeforeSuite.
	Context("Initialize", func() {
		It("new agent should be returned", func() {
			Expect(fakeAgentFactory.NewAgentCallCount()).To(Equal(1))
		})

		It("should return a checker", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(checker).ToNot(BeNil())
		})

		Context("Checker", func() {
			Context("ShouldCheckHttpMethod", func() {
				It("should return correct result", func() {
					Expect(checker.ShouldCheckHttpMethod("GET")).To(BeFalse())
					Expect(checker.ShouldCheckHttpMethod("DELETE")).To(BeFalse())
					Expect(checker.ShouldCheckHttpMethod("PUT")).To(BeTrue())
					Expect(checker.ShouldCheckHttpMethod("POST")).To(BeTrue())
				})
			})

			Context("ShouldCheckAction", func() {
				It("should return correct result", func() {
					Expect(checker.ShouldCheckAction("did_1")).To(BeFalse())
					Expect(checker.ShouldCheckAction("did_2")).To(BeFalse())
					Expect(checker.ShouldCheckAction("do_1")).To(BeTrue())
					Expect(checker.ShouldCheckAction("do_2")).To(BeTrue())
				})
			})

			Context("ShouldSkipAction", func() {
				It("should return correct result", func() {
					Expect(checker.ShouldSkipAction("did_1")).To(BeFalse())
					Expect(checker.ShouldSkipAction("did_2")).To(BeFalse())
					Expect(checker.ShouldSkipAction("skip_1")).To(BeTrue())
					Expect(checker.ShouldSkipAction("skip_2")).To(BeTrue())
				})
			})

			Context("Check", func() {
				var (
					input    policy.PolicyCheckInput
					output   policy.PolicyCheckOutput
					checkErr error
				)
				BeforeEach(func() {
					input = policy.PolicyCheckInput{}
				})
				JustBeforeEach(func() {
					output, checkErr = checker.Check(input)
				})

				It("agent should be called", func() {
					Expect(fakeAgent.CheckCallCount()).To(Equal(1))
				})
				It("cluster name should be injected into input", func() {
					realInput := fakeAgent.CheckArgsForCall(0)
					Expect(realInput).To(Equal(policy.PolicyCheckInput{
						Service:        "concourse",
						ClusterName:    "some-cluster",
						ClusterVersion: "some-version",
					}))
				})

				Context("when agent says pass", func() {
					BeforeEach(func() {
						fakeAgent.CheckReturns(policy.PassedPolicyCheck(), nil)
					})

					It("it should pass", func() {
						Expect(checkErr).ToNot(HaveOccurred())
						Expect(output.Allowed).To(BeTrue())
					})
				})

				Context("when agent says not-pass", func() {
					BeforeEach(func() {
						fakeAgent.CheckReturns(policy.FailedPolicyCheck(), nil)
					})

					It("should not pass", func() {
						Expect(checkErr).ToNot(HaveOccurred())
						Expect(output.Allowed).To(BeFalse())
					})
				})

				Context("when agent includes reasons", func() {
					BeforeEach(func() {
						fakeAgent.CheckReturns(
							policy.PolicyCheckOutput{
								Allowed: false,
								Reasons: []string{"a policy says you can't do that"},
							},
							nil,
						)
					})

					It("should include reasons", func() {
						Expect(checkErr).ToNot(HaveOccurred())
						Expect(output.Reasons).To(ConsistOf("a policy says you can't do that"))
					})
				})

				Context("when agent says error", func() {
					BeforeEach(func() {
						fakeAgent.CheckReturns(policy.FailedPolicyCheck(), errors.New("some-error"))
					})

					It("should not pass", func() {
						Expect(checkErr).To(HaveOccurred())
						Expect(checkErr.Error()).To(Equal("some-error"))
						Expect(output.Allowed).To(BeFalse())
					})
				})
			})
		})
	})
})
