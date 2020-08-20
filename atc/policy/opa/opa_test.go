package opa_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/opa"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Policy checker", func() {

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
		agent, err = (&opa.OpaConfig{fakeOpa.URL, time.Second * 2}).NewAgent(logger)
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
			Expect(err).To(MatchError(ContainSubstring("opa returned invalid response")))
			Expect(result.Allowed).To(BeFalse())
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
			Expect(err).To(MatchError(ContainSubstring("opa returned invalid response")))
			Expect(result.Allowed).To(BeFalse())
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
			Expect(err).To(MatchError(ContainSubstring("opa returned invalid response")))
			Expect(result.Allowed).To(BeFalse())
		})
	})

	Context("when OPA returns allowed", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": {"allowed": true }}`)
			}))
		})

		It("should be allowed", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Allowed).To(BeTrue())
		})
	})

	Context("when OPA returns not-allowed", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": {"allowed": false }}`)
			}))
		})

		It("should not be allowed and return reasons", func() {
			result, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Allowed).To(BeFalse())
			Expect(result.Reasons).To(BeEmpty())
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
			Expect(result.Allowed).To(BeFalse())
			Expect(result.Reasons).To(ConsistOf("a policy says you can't do that"))
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
			Expect(result.Allowed).To(BeFalse())
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
			Expect(result.Allowed).To(BeFalse())
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
			Expect(err.Error()).To(Equal("opa returned bad response: invalid character 'h' looking for beginning of value"))
			Expect(result.Allowed).To(BeFalse())
		})
	})
})
