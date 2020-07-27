package accessor_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {

	var (
		logger              lager.Logger
		fakeHandler         *accessorfakes.FakeHandler
		fakeAccess          *accessorfakes.FakeAccess
		fakeAccessorFactory *accessorfakes.FakeAccessFactory
		fakeAuditor         *auditorfakes.FakeAuditor

		createAccessError   error

		action      string
		customRoles map[string]string

		r *http.Request
		w *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		logger = lager.NewLogger("test")

		fakeHandler = new(accessorfakes.FakeHandler)
		fakeAccess = new(accessorfakes.FakeAccess)
		fakeAccessorFactory = new(accessorfakes.FakeAccessFactory)
		fakeAuditor = new(auditorfakes.FakeAuditor)

		action = "some-action"
		customRoles = map[string]string{"some-action": "some-role"}

		var err error
		r, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())

		w = httptest.NewRecorder()
	})

	JustBeforeEach(func() {
		fakeAccessorFactory.CreateReturns(fakeAccess, createAccessError)
		handler := accessor.NewHandler(
			logger,
			action,
			fakeHandler,
			fakeAccessorFactory,
			fakeAuditor,
			customRoles,
		)

		handler.ServeHTTP(w, r)
	})

	Describe("Accessor Handler", func() {
		Context("when there's a default role for the given action", func() {
			BeforeEach(func() {
				action = atc.SaveConfig
			})

			Context("when the role has not been customized", func() {
				BeforeEach(func() {
					customRoles = map[string]string{}
				})

				It("finds the role", func() {
					Expect(fakeAccessorFactory.CreateCallCount()).To(Equal(1))
					_, role := fakeAccessorFactory.CreateArgsForCall(0)
					Expect(role).To(Equal(accessor.MemberRole))
				})
			})

			Context("when the role has been customized", func() {
				BeforeEach(func() {
					customRoles = map[string]string{
						atc.SaveConfig: accessor.ViewerRole,
					}
				})

				It("finds the role", func() {
					Expect(fakeAccessorFactory.CreateCallCount()).To(Equal(1))
					_, role := fakeAccessorFactory.CreateArgsForCall(0)
					Expect(role).To(Equal(accessor.ViewerRole))
				})
			})
		})

		Context("when there's no default role for the given action", func() {
			BeforeEach(func() {
				action = "some-admin-role"
			})

			Context("when the role has not been customized", func() {
				BeforeEach(func() {
					customRoles = map[string]string{}
				})

				It("sends a blank role (admin roles don't have defaults)", func() {
					Expect(fakeAccessorFactory.CreateCallCount()).To(Equal(1))
					_, role := fakeAccessorFactory.CreateArgsForCall(0)
					Expect(role).To(BeEmpty())
				})
			})
		})

		Context("when the request is authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.ClaimsReturns(accessor.Claims{
					UserName:  "some-user",
					Connector: "some-connector",
					Sub:       "some-sub",
				})
			})

			It("audits the event", func() {
				Expect(fakeAuditor.AuditCallCount()).To(Equal(1))
				action, userName, req := fakeAuditor.AuditArgsForCall(0)
				Expect(action).To(Equal("some-action"))
				Expect(userName).To(Equal("some-user"))
				Expect(req).To(Equal(r))
			})

			It("invokes the handler", func() {
				Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
				_, r := fakeHandler.ServeHTTPArgsForCall(0)
				Expect(accessor.GetAccessor(r)).To(Equal(fakeAccess))
			})
		})

		Context("when the request is not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
				fakeAccess.ClaimsReturns(accessor.Claims{})
			})

			It("audits the anonymous request", func() {
				Expect(fakeAuditor.AuditCallCount()).To(Equal(1))
				action, userName, req := fakeAuditor.AuditArgsForCall(0)
				Expect(action).To(Equal("some-action"))
				Expect(userName).To(Equal(""))
				Expect(req).To(Equal(r))
			})

			It("invokes the handler", func() {
				Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
				_, r := fakeHandler.ServeHTTPArgsForCall(0)
				Expect(accessor.GetAccessor(r)).To(Equal(fakeAccess))
			})
		})

		Context("when the accessor factory errors", func() {
			BeforeEach(func() {
				createAccessError = errors.New("<<something bad here>>")
			})

			It("returns a server error", func() {
				Expect(w.Result().StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
