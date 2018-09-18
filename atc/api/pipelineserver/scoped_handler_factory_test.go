package pipelineserver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/pipelineserver"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var (
		response *http.Response
		server   *httptest.Server
		delegate *delegateHandler

		dbTeamFactory *dbfakes.FakeTeamFactory
		fakeTeam      *dbfakes.FakeTeam
		fakePipeline  *dbfakes.FakePipeline

		handler http.Handler
	)

	BeforeEach(func() {
		delegate = &delegateHandler{}

		dbTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeam = new(dbfakes.FakeTeam)
		fakePipeline = new(dbfakes.FakePipeline)

		handlerFactory := pipelineserver.NewScopedHandlerFactory(dbTeamFactory)
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

	Context("when pipeline is in request context", func() {
		var contextPipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			contextPipeline = new(dbfakes.FakePipeline)
			handler = &wrapHandler{handler, contextPipeline}
		})

		It("calls scoped handler with pipeline from context", func() {
			Expect(delegate.IsCalled).To(BeTrue())
			Expect(delegate.Pipeline).To(BeIdenticalTo(contextPipeline))
		})
	})

	Context("when pipeline is not in request context", func() {
		Context("when the team does not exist", func() {
			BeforeEach(func() {
				dbTeamFactory.FindTeamReturns(nil, false, nil)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})

			It("does not call the scoped handler", func() {
				Expect(delegate.IsCalled).To(BeFalse())
			})
		})

		Context("when finding the team fails", func() {
			BeforeEach(func() {
				dbTeamFactory.FindTeamReturns(nil, false, errors.New("error"))
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("does not call the scoped handler", func() {
				Expect(delegate.IsCalled).To(BeFalse())
			})
		})

		Context("when pipeline exists", func() {
			BeforeEach(func() {
				fakeTeam.NameReturns("some-team")
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			It("looks up the team by the right name", func() {
				Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
				Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("some-team"))
			})

			Context("when the pipeline exists", func() {
				BeforeEach(func() {
					fakePipeline.NameReturns("some-pipeline")
					fakeTeam.PipelineReturns(fakePipeline, true, nil)
				})

				It("looks up the pipeline by the right name", func() {
					Expect(fakeTeam.PipelineCallCount()).To(Equal(1))
					Expect(fakeTeam.PipelineArgsForCall(0)).To(Equal("some-pipeline"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("calls the scoped handler", func() {
					Expect(delegate.IsCalled).To(BeTrue())
				})
			})

			Context("when the pipeline does not exist", func() {
				BeforeEach(func() {
					fakeTeam.PipelineReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})

				It("does not call the scoped handler", func() {
					Expect(delegate.IsCalled).To(BeFalse())
				})
			})

			Context("when finding the pipeline fails", func() {
				BeforeEach(func() {
					fakeTeam.PipelineReturns(nil, false, errors.New("error"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("does not call the scoped handler", func() {
					Expect(delegate.IsCalled).To(BeFalse())
				})
			})
		})
	})
})

type delegateHandler struct {
	IsCalled bool
	Pipeline db.Pipeline
}

func (handler *delegateHandler) GetHandler(dbPipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.IsCalled = true
		handler.Pipeline = dbPipeline
	})
}

type wrapHandler struct {
	delegate        http.Handler
	contextPipeline db.Pipeline
}

func (h *wrapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), auth.PipelineContextKey, h.contextPipeline)
	h.delegate.ServeHTTP(w, r.WithContext(ctx))
}
