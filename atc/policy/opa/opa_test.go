package opa_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/opa"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OPA Policy Checker", func() {

	var (
		logger  = lagertest.NewTestLogger("opa-test")
		fakeOpa *httptest.Server
		agent   policy.Agent
		err     error
	)

	AfterEach(func() {
		if fakeOpa != nil {
			fakeOpa.Close()
		}
	})

	JustBeforeEach(func() {
		fakeOpa.Start()
		agent, err = (&opa.OpaConfig{
			URL:                  fakeOpa.URL,
			Timeout:              time.Second * 2,
			ResultAllowedKey:     "result.allowed",
			ResultShouldBlockKey: "result.block",
			ResultMessagesKey:    "result.reasons",
		}).NewAgent(logger)
		Expect(err).ToNot(HaveOccurred())
		Expect(agent).ToNot(BeNil())
	})

	Context("when OPA returns no result", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "{}")
			}))
		})

		It("should return an error", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(MatchError(ContainSubstring("allowed: key 'result.allowed' not found")))
			Expect(result).To(BeNil())
		})
	})

	Context("when OPA returns an empty result", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{ "result": {} }`)
			}))
		})

		It("should return an error", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(MatchError(ContainSubstring("allowed: missing field 'allowed' in var: result.allowed")))
			Expect(result).To(BeNil())
		})
	})

	Context("when OPA returns no allowed field", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{ "result": { "reasons": [ "a policy says you can't do that" ] }}`)
			}))
		})

		It("should return an error", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(MatchError(ContainSubstring("allowed: missing field 'allowed' in var: result.allowed")))
			Expect(result).To(BeNil())
		})
	})

	Context("when OPA returns only the field of allowed with true", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": {"allowed": true }}`)
			}))
		})

		It("should be allowed", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Allowed()).To(BeTrue())
			Expect(result.ShouldBlock()).To(BeFalse())
			Expect(result.Messages()).To(BeEmpty())
		})
	})

	Context("when OPA returns only the field of allowed with false", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": {"allowed": false }}`)
			}))
		})

		It("should not be allowed", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Allowed()).To(BeFalse())
			Expect(result.ShouldBlock()).To(BeTrue())
			Expect(result.Messages()).To(BeEmpty())
		})
	})

	Context("when OPA returns allowed false and block true", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": {"allowed": false, "block": true }}`)
			}))
		})

		It("should not be allowed", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Allowed()).To(BeFalse())
			Expect(result.ShouldBlock()).To(BeTrue())
			Expect(result.Messages()).To(BeEmpty())
		})
	})

	Context("when OPA returns allowed false and block false", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": {"allowed": false, "block": false }}`)
			}))
		})

		It("should not be allowed", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Allowed()).To(BeFalse())
			Expect(result.ShouldBlock()).To(BeFalse())
			Expect(result.Messages()).To(BeEmpty())
		})
	})

	Context("when OPA returns not-allowed with reasons", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": {"allowed": false, "reasons": ["a policy says you can't do that"]}}`)
			}))
		})

		It("should not be allowed and return reasons", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Allowed()).To(BeFalse())
			Expect(result.ShouldBlock()).To(BeTrue())
			Expect(result.Messages()).To(ConsistOf("a policy says you can't do that"))
		})
	})

	Context("when OPA is unreachable", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.NotFoundHandler())
		})

		JustBeforeEach(func() {
			fakeOpa.Close()
			fakeOpa = nil
		})

		It("should return error", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(MatchRegexp("connection refused"))
			Expect(result).To(BeNil())
		})
	})

	Context("when OPA returns http error", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.NotFoundHandler())
		})

		It("should return error", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("opa returned status: 404"))
			Expect(result).To(BeNil())
		})
	})

	Context("when OPA returns bad response", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `hello`)
			}))
		})

		It("should return error", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid character 'h' looking for beginning of value"))
			Expect(result).To(BeNil())
		})
	})
})
