package accessor_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {

	var (
		logger lager.Logger

		fakeAccessorFactory *accessorfakes.FakeAccessFactory
		fakeHandler         *accessorfakes.FakeHandler
		fakeAuditor         *auditorfakes.FakeAuditor
		fakeUserFactory     *dbfakes.FakeUserFactory
		fakeAccess          *accessorfakes.FakeAccess
		accessorHandler     http.Handler

		req *http.Request
		w   *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		logger = lager.NewLogger("test")

		fakeAccessorFactory = new(accessorfakes.FakeAccessFactory)
		fakeHandler = new(accessorfakes.FakeHandler)
		fakeAuditor = new(auditorfakes.FakeAuditor)
		fakeUserFactory = new(dbfakes.FakeUserFactory)
		fakeAccess = new(accessorfakes.FakeAccess)

		var err error
		req, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())

		w = httptest.NewRecorder()
	})

	JustBeforeEach(func() {
		accessorHandler.ServeHTTP(w, req)
	})

	Describe("Accessor Handler", func() {
		BeforeEach(func() {
			accessorHandler = accessor.NewHandler(
				logger,
				fakeHandler,
				fakeAccessorFactory,
				"some-action",
				fakeAuditor,
				fakeUserFactory,
			)
		})

		Context("when access factory returns an error", func() {
			BeforeEach(func() {
				fakeAccessorFactory.CreateReturns(nil, errors.New("nope"))
			})

			It("returns a server error", func() {
				Expect(w.Result().StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when access factory return valid access object", func() {
			BeforeEach(func() {
				fakeAccessorFactory.CreateReturns(fakeAccess, nil)
			})

			Context("when the request is anonymous", func() {
				BeforeEach(func() {
					fakeAccess.ClaimsReturns(accessor.Claims{})
				})

				It("doesn't track the request", func() {
					Expect(fakeUserFactory.CreateOrUpdateUserCallCount()).To(Equal(0))
				})

				It("audits the anonymous request", func() {
					Expect(fakeAuditor.AuditCallCount()).To(Equal(1))
					action, userName, r := fakeAuditor.AuditArgsForCall(0)
					Expect(action).To(Equal("some-action"))
					Expect(userName).To(Equal(""))
					Expect(r).To(Equal(req))
				})

				It("invokes the handler", func() {
					Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
					_, r := fakeHandler.ServeHTTPArgsForCall(0)
					Expect(accessor.GetAccessor(r)).To(Equal(fakeAccess))
				})
			})

			Context("when the request is authenticated", func() {
				BeforeEach(func() {
					fakeAccess.ClaimsReturns(accessor.Claims{
						UserName:  "some-user",
						Connector: "some-connector",
						Sub:       "some-sub",
					})
				})

				Context("when the user factory fails", func() {
					BeforeEach(func() {
						fakeUserFactory.CreateOrUpdateUserReturns(nil, errors.New("nope"))
					})

					It("returns a server error", func() {
						Expect(w.Result().StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the user factory succeeds", func() {
					BeforeEach(func() {
						fakeUser := new(dbfakes.FakeUser)
						fakeUserFactory.CreateOrUpdateUserReturns(fakeUser, nil)
					})

					It("updates the requesting user's activity", func() {
						Expect(fakeUserFactory.CreateOrUpdateUserCallCount()).To(Equal(1))
						username, connector, sub := fakeUserFactory.CreateOrUpdateUserArgsForCall(0)
						Expect(username).To(Equal("some-user"))
						Expect(connector).To(Equal("some-connector"))
						Expect(sub).To(Equal("some-sub"))
					})

					It("audits the event", func() {
						Expect(fakeAuditor.AuditCallCount()).To(Equal(1))
						action, userName, r := fakeAuditor.AuditArgsForCall(0)
						Expect(action).To(Equal("some-action"))
						Expect(userName).To(Equal("some-user"))
						Expect(r).To(Equal(req))
					})

					It("invokes the handler", func() {
						Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
						_, r := fakeHandler.ServeHTTPArgsForCall(0)
						Expect(accessor.GetAccessor(r)).To(Equal(fakeAccess))
					})
				})
			})
		})
	})
})
