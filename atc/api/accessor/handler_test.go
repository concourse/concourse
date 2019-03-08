package accessor_test

import (
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	// dummy handler
	var (
		innerHandlerCalled bool
		accessorFactory    *accessorfakes.FakeAccessFactory
		dummyHandler       http.HandlerFunc
		access             accessor.Access
		fakeAccess         *accessorfakes.FakeAccess
		accessorHandler    http.Handler
		req                *http.Request
	)
	BeforeEach(func() {
		accessorFactory = new(accessorfakes.FakeAccessFactory)

		dummyHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			innerHandlerCalled = true
			access = r.Context().Value("accessor").(accessor.Access)
		})

		var err error
		req, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		accessorFactory.CreateReturns(fakeAccess)
		accessorHandler.ServeHTTP(nil, req)
	})

	Describe("Accessor Handler", func() {
		BeforeEach(func() {

			accessorHandler = accessor.NewHandler(dummyHandler, accessorFactory, "some-action", lager.NewLogger("accessor_handler_test"))
		})

		Context("when access factory return valid access object", func() {
			BeforeEach(func() {
				fakeAccess = new(accessorfakes.FakeAccess)
			})
			It("calls the inner handler", func() {
				Expect(innerHandlerCalled).To(BeTrue())
				Expect(access).To(Equal(fakeAccess))
			})
		})

		Context("when access factory does not return valid access object", func() {
			BeforeEach(func() {
				fakeAccess = nil
			})
			It("request context is set to Nil", func() {
				Expect(innerHandlerCalled).To(BeTrue())
				Expect(access).To(BeNil())
			})
		})
	})
})
