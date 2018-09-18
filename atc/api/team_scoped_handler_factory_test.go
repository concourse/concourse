package api_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/accessor"
	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamScopedHandlerFactory", func() {
	var (
		response        *http.Response
		server          *httptest.Server
		delegate        *delegateHandler
		fakeTeamFactory *dbfakes.FakeTeamFactory
		fakeTeam        *dbfakes.FakeTeam
		handler         http.Handler
		fakeaccess      *accessorfakes.FakeAccess
	//	accessFactory   *accessorfakes.FakeAccessFactory
	)

	BeforeEach(func() {
		fakeaccess = new(accessorfakes.FakeAccess)
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)

		delegate = &delegateHandler{}

		logger := lagertest.NewTestLogger("test")

		handlerFactory := api.NewTeamScopedHandlerFactory(logger, fakeTeamFactory)
		innerHandler := handlerFactory.HandlerFor(delegate.GetHandler)

		handler = accessor.NewHandler(innerHandler, fakeAccessor)
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
		server = httptest.NewServer(handler)

		fullUrl := fmt.Sprintf("%s?:team_name=some-team", server.URL)

		serverUrl, err := url.Parse(fullUrl)
		Expect(err).NotTo(HaveOccurred())

		request, err := http.NewRequest("POST", serverUrl.String(), nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	Context("when team is in auth context", func() {
		BeforeEach(func() {
			fakeaccess.IsAuthenticatedReturns(true)
			fakeaccess.IsAuthorizedReturns(true)
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
			fakeaccess.IsAuthorizedReturns(false)
		})

		It("returns 403", func() {
			Expect(response.StatusCode).To(Equal(http.StatusForbidden))
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
