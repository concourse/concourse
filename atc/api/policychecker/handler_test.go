package policychecker_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/atc/api/policychecker"
	"github.com/concourse/concourse/atc/api/policychecker/policycheckerfakes"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var (
		innerHandlerCalled    bool
		dummyHandler          http.HandlerFunc
		policyCheckerHandler  http.Handler
		req                   *http.Request
		fakePolicyChecker     *policycheckerfakes.FakePolicyChecker
		fakePolicyCheckResult *policyfakes.FakePolicyCheckResult
		responseWriter        *httptest.ResponseRecorder

		logger = lagertest.NewTestLogger("test")
	)

	BeforeEach(func() {
		fakePolicyChecker = new(policycheckerfakes.FakePolicyChecker)
		fakePolicyCheckResult = new(policyfakes.FakePolicyCheckResult)

		innerHandlerCalled = false
		dummyHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			innerHandlerCalled = true
		})

		responseWriter = httptest.NewRecorder()

		var err error
		req, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		policyCheckerHandler.ServeHTTP(responseWriter, req)
	})

	BeforeEach(func() {
		policyCheckerHandler = policychecker.NewHandler(logger, dummyHandler, "some-action", fakePolicyChecker)
	})

	Context("policy check passes", func() {
		BeforeEach(func() {
			fakePolicyChecker.CheckReturns(policy.PassedPolicyCheck(), nil)
		})

		It("calls the inner handler", func() {
			Expect(innerHandlerCalled).To(BeTrue())
		})
	})

	Context("policy check doesn't pass", func() {
		BeforeEach(func() {
			fakePolicyCheckResult.AllowedReturns(false)
			fakePolicyCheckResult.MessagesReturns([]string{"a policy says you can't do that", "another policy also says you can't do that"})
			fakePolicyChecker.CheckReturns(fakePolicyCheckResult, nil)
		})

		Context("when should block", func() {
			BeforeEach(func() {
				fakePolicyCheckResult.ShouldBlockReturns(true)
			})

			It("return http forbidden", func() {
				Expect(responseWriter.Code).To(Equal(http.StatusForbidden))

				msg, err := ioutil.ReadAll(responseWriter.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(msg)).To(ContainSubstring("a policy says you can't do that"))
				Expect(string(msg)).To(ContainSubstring("another policy also says you can't do that"))
			})

			It("not call the inner handler", func() {
				Expect(innerHandlerCalled).To(BeFalse())
			})
		})

		Context("when should not block", func() {
			BeforeEach(func() {
				fakePolicyCheckResult.ShouldBlockReturns(false)
			})

			It("calls the inner handler", func() {
				Expect(innerHandlerCalled).To(BeTrue())
			})

			It("response should have a header about policy check warning", func() {
				value := responseWriter.Header().Get("X-Concourse-Policy-Check-Warning")
				Expect(value).To(ContainSubstring("a policy says you can't do that"))
				Expect(value).To(ContainSubstring("another policy also says you can't do that"))
			})
		})
	})

	Context("policy check errors", func() {
		BeforeEach(func() {
			fakePolicyChecker.CheckReturns(nil, errors.New("some-error"))
		})

		It("return http bad request", func() {
			Expect(responseWriter.Code).To(Equal(http.StatusBadRequest))

			msg, err := ioutil.ReadAll(responseWriter.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(msg)).To(Equal("policy check error: some-error"))
		})

		It("not call the inner handler", func() {
			Expect(innerHandlerCalled).To(BeFalse())
		})
	})
})
