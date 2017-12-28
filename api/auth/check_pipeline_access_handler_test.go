package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/auth/authfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckPipelineAccessHandler", func() {
	var (
		response    *http.Response
		server      *httptest.Server
		delegate    *pipelineDelegateHandler
		teamFactory *dbfakes.FakeTeamFactory
		team        *dbfakes.FakeTeam
		pipeline    *dbfakes.FakePipeline
		handler     http.Handler

		authValidator     *authfakes.FakeValidator
		userContextReader *authfakes.FakeUserContextReader
	)

	BeforeEach(func() {
		teamFactory = new(dbfakes.FakeTeamFactory)
		team = new(dbfakes.FakeTeam)
		teamFactory.FindTeamReturns(team, true, nil)

		pipeline = new(dbfakes.FakePipeline)

		handlerFactory := auth.NewCheckPipelineAccessHandlerFactory(teamFactory)

		authValidator = new(authfakes.FakeValidator)
		userContextReader = new(authfakes.FakeUserContextReader)

		delegate = &pipelineDelegateHandler{}
		checkPipelineAccessHandler := handlerFactory.HandlerFor(delegate, auth.UnauthorizedRejector{})
		handler = auth.WrapHandler(checkPipelineAccessHandler, authValidator, userContextReader)
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

	Context("when pipeline exists", func() {
		BeforeEach(func() {
			pipeline.NameReturns("some-pipeline")
			team.PipelineReturns(pipeline, true, nil)
		})

		Context("when pipeline is public", func() {
			BeforeEach(func() {
				pipeline.PublicReturns(true)
			})

			It("calls pipelineScopedHandler with pipelineDB in context", func() {
				Expect(delegate.IsCalled).To(BeTrue())
				Expect(delegate.ContextPipelineDB).To(BeIdenticalTo(pipeline))
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when pipeline is private", func() {
			Context("and authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(true)
					userContextReader.GetTeamReturns("some-team", true, true)
				})

				It("calls pipelineScopedHandler with pipelineDB in context", func() {
					Expect(delegate.IsCalled).To(BeTrue())
					Expect(delegate.ContextPipelineDB).To(BeIdenticalTo(pipeline))
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("and unauthorized", func() {
				BeforeEach(func() {
					userContextReader.GetTeamReturns("some-other-team", true, true)
				})

				Context("and is authenticated", func() {
					BeforeEach(func() {
						authValidator.IsAuthenticatedReturns(true)
					})

					It("returns 403 forbidden", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})

				Context("and not authenticated", func() {
					BeforeEach(func() {
						authValidator.IsAuthenticatedReturns(false)
					})

					It("returns 401 unauthorized", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
			})
		})
	})

	Context("when pipeline does not exist", func() {
		BeforeEach(func() {
			team.PipelineReturns(nil, false, nil)
		})

		It("returns 404", func() {
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("does not call the scoped handler", func() {
			Expect(delegate.IsCalled).To(BeFalse())
		})
	})

	Context("when getting pipeline fails", func() {
		BeforeEach(func() {
			team.PipelineReturns(nil, false, errors.New("disaster"))
		})

		It("returns 500", func() {
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("does not call the scoped handler", func() {
			Expect(delegate.IsCalled).To(BeFalse())
		})
	})
})

type pipelineDelegateHandler struct {
	IsCalled          bool
	ContextPipelineDB db.Pipeline
}

func (handler *pipelineDelegateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.IsCalled = true
	handler.ContextPipelineDB = r.Context().Value(auth.PipelineContextKey).(db.Pipeline)
}
