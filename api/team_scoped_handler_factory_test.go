package api_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamScopedHandlerFactory", func() {
	var (
		response          *http.Response
		server            *httptest.Server
		delegate          *delegateHandler
		teamDBFactory     *dbfakes.FakeTeamDBFactory
		teamDB            *dbfakes.FakeTeamDB
		authValidator     *authfakes.FakeValidator
		userContextReader *authfakes.FakeUserContextReader
		handler           http.Handler
	)

	BeforeEach(func() {
		teamDBFactory = new(dbfakes.FakeTeamDBFactory)
		teamDB = new(dbfakes.FakeTeamDB)
		teamDBFactory.GetTeamDBReturns(teamDB)

		delegate = &delegateHandler{}

		logger := lagertest.NewTestLogger("test")

		handlerFactory := api.NewTeamScopedHandlerFactory(logger, teamDBFactory, dbTeamFactory)
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

		It("creates teamDB with team name from context", func() {
			Expect(teamDBFactory.GetTeamDBCallCount()).To(Equal(1))
			Expect(teamDBFactory.GetTeamDBArgsForCall(0)).To(Equal("some-team"))
		})

		It("calls scoped handler with teamDB from context", func() {
			Expect(delegate.IsCalled).To(BeTrue())
			Expect(delegate.TeamDB).To(BeIdenticalTo(teamDB))
			Expect(delegate.Team).To(BeIdenticalTo(dbTeam))
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
			Expect(delegate.TeamDB).To(BeNil())
			Expect(delegate.Team).To(BeNil())
		})
	})
})

type delegateHandler struct {
	IsCalled bool
	TeamDB   db.TeamDB
	Team     dbng.Team
}

func (handler *delegateHandler) GetHandler(teamDB db.TeamDB, dbTeam dbng.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.IsCalled = true
		handler.TeamDB = teamDB
		handler.Team = dbTeam
	})
}
