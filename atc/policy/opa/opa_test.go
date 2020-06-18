package opa_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"fmt"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/opa"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Policy checker", func() {

	var (
		logger = lagertest.NewTestLogger("opa-test")
		fakeOpa *httptest.Server
		agent policy.Agent
		err error
	)

	AfterEach(func() {
		if fakeOpa != nil {
			fakeOpa.Close()
		}
	})

	JustBeforeEach(func() {
		fakeOpa.Start()
		agent, err = (&opa.OpaConfig{fakeOpa.URL, time.Second*2}).NewAgent(logger)
		Expect(err).ToNot(HaveOccurred())
		Expect(agent).ToNot(BeNil())
	})

	Context("when OPA returns no result", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "{}")
			}))
		})

		It("should pass", func() {
			pass, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(pass).To(BeTrue())
		})
	})

	Context("when OPA returns pass", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": true}`)
			}))
		})

		It("should pass", func() {
			pass, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(pass).To(BeTrue())
		})
	})

	Context("when OPA returns not-pass", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result": false}`)
			}))
		})

		It("should not pass", func() {
			pass, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).ToNot(HaveOccurred())
			Expect(pass).To(BeFalse())
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
			pass, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(MatchRegexp("connection refused"))
			Expect(pass).To(BeFalse())
		})
	})

	Context("when OPA returns http error", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.NotFoundHandler())
		})

		It("should return error", func() {
			pass, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("opa returned status: 404"))
			Expect(pass).To(BeFalse())
		})
	})

	Context("when OPA returns bad response", func() {
		BeforeEach(func() {
			fakeOpa = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `hello`)
			}))
		})

		It("should return error", func() {
			pass, err := agent.Check(policy.PolicyCheckInput{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("opa returned bad response: invalid character 'h' looking for beginning of value"))
			Expect(pass).To(BeFalse())
		})
	})
})
