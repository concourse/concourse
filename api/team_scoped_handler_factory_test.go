package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/auth/authfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamScopedHandlerFactory", func() {
	var (
		response          *http.Response
		server            *httptest.Server
		delegate          *delegateHandler
		fakeTeamFactory   *dbfakes.FakeTeamFactory
		fakeTeam          *dbfakes.FakeTeam
		authValidator     *authfakes.FakeValidator
		userContextReader *authfakes.FakeUserContextReader
		handler           http.Handler
	)

	BeforeEach(func() {
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)

		delegate = &delegateHandler{}

		logger := lagertest.NewTestLogger("test")

		handlerFactory := api.NewTeamScopedHandlerFactory(logger, fakeTeamFactory)
		innerHandler := handlerFactory.HandlerFor(delegate.GetHandler)

		authValidator = new(authfakes.FakeValidator)
		userContextReader = new(authfakes.FakeUserContextReader)

		handler = auth.WrapHandler(innerHandler, authValidator, userContextReader)
	})

	JustBeforeEach(func() {
		server = httptest.NewServer(handler)

		request, err := http.NewRequest("POST", server.URL, nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	Context("when team is in auth context", func() {
		BeforeEach(func() {
			authValidator.IsAuthenticatedReturns(true)
			userContextReader.GetTeamReturns("some-team", false, true)
		})

		Context("when the team is not found", func() {
			BeforeEach(func() {
				fakeTeamFactory.FindTeamReturns(nil, false, nil)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when finding the team fails", func() {
			BeforeEach(func() {
				fakeTeamFactory.FindTeamReturns(nil, false, errors.New("what is a team?"))
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		It("creates team with team name from context", func() {
			Expect(fakeTeamFactory.FindTeamCallCount()).To(Equal(1))
			Expect(fakeTeamFactory.FindTeamArgsForCall(0)).To(Equal("some-team"))
		})

		It("calls scoped handler with team from context", func() {
			Expect(delegate.IsCalled).To(BeTrue())
			Expect(delegate.Team).To(BeIdenticalTo(fakeTeam))
		})
	})

	Context("when team is not in auth context", func() {
		BeforeEach(func() {
			authValidator.IsAuthenticatedReturns(true)
			userContextReader.GetTeamReturns("", false, false)
		})

		It("returns 500", func() {
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("does not call scoped handler", func() {
			Expect(delegate.IsCalled).To(BeFalse())
			Expect(delegate.Team).To(BeNil())
		})
	})
})

type delegateHandler struct {
	IsCalled bool
	Team     db.Team
}

func (handler *delegateHandler) GetHandler(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.IsCalled = true
		handler.Team = team
	})
}
