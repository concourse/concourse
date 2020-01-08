package accessor_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	// dummy handler
	var (
		logger             lager.Logger
		innerHandlerCalled bool
		accessorFactory    *accessorfakes.FakeAccessFactory
		dummyHandler       http.HandlerFunc
		access             accessor.Access
		fakeAccess         *accessorfakes.FakeAccess
		accessorHandler    http.Handler

		req *http.Request
		w   *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		logger = lager.NewLogger("test")
		accessorFactory = new(accessorfakes.FakeAccessFactory)

		dummyHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			innerHandlerCalled = true

			access = r.Context().Value("accessor").(accessor.Access)
		})

		w = httptest.NewRecorder()

		var err error
		req, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		accessorHandler.ServeHTTP(w, req)
	})

	Describe("Accessor Handler", func() {
		BeforeEach(func() {
			accessorHandler = accessor.NewHandler(logger, dummyHandler, accessorFactory, "some-action", new(auditorfakes.FakeAuditor))
		})

		Context("when access factory returns an error", func() {
			BeforeEach(func() {
				accessorFactory.CreateReturns(nil, errors.New("nope"))
			})

			It("returns a server error", func() {
				Expect(w.Result().StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when access factory return valid access object", func() {
			BeforeEach(func() {
				fakeAccess = new(accessorfakes.FakeAccess)
				accessorFactory.CreateReturns(fakeAccess, nil)
			})
			It("calls the inner handler", func() {
				Expect(innerHandlerCalled).To(BeTrue())
				Expect(access).To(Equal(fakeAccess))
			})
		})
	})
})
