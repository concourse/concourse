package accessor_test

import (
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"

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
		accessorHandler.ServeHTTP(nil, req)
	})

	Describe("Accessor Handler", func() {
		BeforeEach(func() {
			accessorHandler = accessor.NewHandler(dummyHandler, accessorFactory, "some-action", new(auditorfakes.FakeAuditor))
		})

		Context("when access factory return valid access object", func() {
			BeforeEach(func() {
				fakeAccess = new(accessorfakes.FakeAccess)
				accessorFactory.CreateReturns(fakeAccess)
			})
			It("calls the innder handler", func() {
				Expect(innerHandlerCalled).To(BeTrue())
				Expect(access).To(Equal(fakeAccess))
			})
		})
	})
})
