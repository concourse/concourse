package pipelineserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/api/pipelineserver"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var (
		response      *http.Response
		server        *httptest.Server
		delegate      *delegateHandler
		teamDBFactory *dbfakes.FakeTeamDBFactory
		teamDB        *dbfakes.FakeTeamDB
		pipelineDB    *dbfakes.FakePipelineDB

		dbTeamFactory *dbngfakes.FakeTeamFactory
		fakeTeam      *dbngfakes.FakeTeam
		fakePipeline  *dbngfakes.FakePipeline

		handler http.Handler
	)

	BeforeEach(func() {
		teamDBFactory = new(dbfakes.FakeTeamDBFactory)
		teamDB = new(dbfakes.FakeTeamDB)
		teamDBFactory.GetTeamDBReturns(teamDB)

		pipelineDB = new(dbfakes.FakePipelineDB)
		delegate = &delegateHandler{}

		pipelineDBFactory := new(dbfakes.FakePipelineDBFactory)
		pipelineDBFactory.BuildReturns(pipelineDB)

		dbTeamFactory = new(dbngfakes.FakeTeamFactory)

		fakeTeam = new(dbngfakes.FakeTeam)
		dbTeamFactory.GetByIDReturns(fakeTeam)

		fakePipeline = new(dbngfakes.FakePipeline)
		fakeTeam.PipelineReturns(fakePipeline, true, nil)

		handlerFactory := pipelineserver.NewScopedHandlerFactory(pipelineDBFactory, teamDBFactory, dbTeamFactory)
		handler = handlerFactory.HandlerFor(delegate.GetHandler)
	})

	JustBeforeEach(func() {
		server = httptest.NewServer(handler)

		request, err := http.NewRequest("POST", server.URL+"?:team_name=some-team&:pipeline_name=some-pipeline", nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	Context("when pipelineDB is in request context", func() {
		var contextPipelineDB db.PipelineDB

		BeforeEach(func() {
			contextPipelineDB = new(dbfakes.FakePipelineDB)
			handler = &wrapHandler{handler, contextPipelineDB}
		})

		It("calls scoped handler with pipelineDB from context", func() {
			Expect(delegate.IsCalled).To(BeTrue())
			Expect(delegate.PipelineDB).To(BeIdenticalTo(contextPipelineDB))
		})
	})

	Context("when pipelineDB is not in request context", func() {
		Context("when pipeline does not exist", func() {
			BeforeEach(func() {
				teamDB.GetPipelineByNameReturns(db.SavedPipeline{}, false, nil)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})

			It("does not call the scoped handler", func() {
				Expect(delegate.IsCalled).To(BeFalse())
			})
		})

		Context("when pipeline exists", func() {
			BeforeEach(func() {
				teamDB.GetPipelineByNameReturns(db.SavedPipeline{Pipeline: db.Pipeline{Name: "some-pipeline"}}, true, nil)
			})

			It("looks up the team by the right name", func() {
				Expect(teamDBFactory.GetTeamDBCallCount()).To(Equal(1))
				Expect(teamDBFactory.GetTeamDBArgsForCall(0)).To(Equal("some-team"))
			})

			It("looks up the pipeline by the right name", func() {
				Expect(teamDB.GetPipelineByNameCallCount()).To(Equal(1))
				Expect(teamDB.GetPipelineByNameArgsForCall(0)).To(Equal("some-pipeline"))
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("calls the scoped handler", func() {
				Expect(delegate.IsCalled).To(BeTrue())
			})
		})
	})
})

type delegateHandler struct {
	IsCalled   bool
	PipelineDB db.PipelineDB
}

func (handler *delegateHandler) GetHandler(pipelineDB db.PipelineDB, dbPipeline dbng.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.IsCalled = true
		handler.PipelineDB = pipelineDB
	})
}

type wrapHandler struct {
	delegate          http.Handler
	contextPipelineDB db.PipelineDB
}

func (h *wrapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), auth.PipelineDBKey, h.contextPipelineDB)
	h.delegate.ServeHTTP(w, r.WithContext(ctx))
}
