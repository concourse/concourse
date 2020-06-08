package exec_test

import (
	"errors"
	"github.com/concourse/concourse/atc/policy/policyfakes"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImagePolicyChecker", func() {
	var (
		policyFilter policy.Filter
		pass         bool
		checkErr     error
	)

	BeforeEach(func() {
		policyFilter = policy.Filter{
			ActionsToSkip: []string{},
			Actions:       []string{},
			HttpMethods:   []string{},
		}

		fakePolicyAgent = new(policyfakes.FakeAgent)
		fakePolicyAgentFactory.NewAgentReturns(fakePolicyAgent, nil)
	})

	JustBeforeEach(func() {
		policyCheck, err := policy.Initialize(testLogger, "some-cluster", "some-version", policyFilter)
		Expect(err).ToNot(HaveOccurred())
		pass, checkErr = exec.NewImagePolicyChecker(policyCheck).Check(
			"some-team", "some-pipeline", "get", "registry-image", atc.Source{"repository": "some-image", "tag": "some-tag"})
	})

	Context("when the action should be skipped", func() {
		BeforeEach(func() {
			policyFilter.ActionsToSkip = []string{policy.ActionUsingImage}
		})

		It("should pass", func() {
			Expect(checkErr).ToNot(HaveOccurred())
			Expect(pass).To(BeTrue())
		})
		It("Agent should not be called", func() {
			Expect(fakePolicyAgent.CheckCallCount()).To(Equal(0))
		})
	})

	Context("when the action is not skipped", func() {
		BeforeEach(func() {
			policyFilter.ActionsToSkip = []string{}
		})

		It("should not error", func() {
			Expect(checkErr).ToNot(HaveOccurred())
		})
		It("agent check should be called", func() {
			Expect(fakePolicyAgent.CheckCallCount()).To(Equal(1))
		})
		It("agent should take correct input", func() {
			Expect(fakePolicyAgent.CheckArgsForCall(0)).To(Equal(policy.PolicyCheckInput{
				Service:        "concourse",
				ClusterName:    "some-cluster",
				ClusterVersion: "some-version",
				Action:         policy.ActionUsingImage,
				Team:           "some-team",
				Pipeline:       "some-pipeline",
				Data: map[string]interface{}{
					"image_source": atc.Source{
						"tag":        "some-tag",
						"repository": "some-image",
					},
					"step":              "get",
					"image_source_type": "registry-image",
				},
			}))
		})

		Context("when agent says pass", func() {
			BeforeEach(func() {
				fakePolicyAgent.CheckReturns(true, nil)
			})

			It("it should pass", func() {
				Expect(checkErr).ToNot(HaveOccurred())
				Expect(pass).To(BeTrue())
			})
		})

		Context("when agent says not-pass", func() {
			BeforeEach(func() {
				fakePolicyAgent.CheckReturns(false, nil)
			})

			It("should not pass", func() {
				Expect(checkErr).ToNot(HaveOccurred())
				Expect(pass).To(BeFalse())
			})
		})

		Context("when agent says error", func() {
			BeforeEach(func() {
				fakePolicyAgent.CheckReturns(false, errors.New("some-error"))
			})

			It("should not pass", func() {
				Expect(checkErr).To(HaveOccurred())
				Expect(checkErr.Error()).To(Equal("some-error"))
				Expect(pass).To(BeFalse())
			})
		})
	})
})
