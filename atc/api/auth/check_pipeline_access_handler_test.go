package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"

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

		fakeAccessor *accessorfakes.FakeAccessFactory
		fakeAccess   *accessorfakes.FakeAccess
	)

	BeforeEach(func() {
		teamFactory = new(dbfakes.FakeTeamFactory)
		team = new(dbfakes.FakeTeam)
		teamFactory.FindTeamReturns(team, true, nil)

		pipeline = new(dbfakes.FakePipeline)

		handlerFactory := auth.CheckPipelineAccessHandlerFactory{}
		fakeAccessor = new(accessorfakes.FakeAccessFactory)
		fakeAccess = new(accessorfakes.FakeAccess)

		delegate = &pipelineDelegateHandler{}
		innerHandler := handlerFactory.HandlerFor(delegate, auth.UnauthorizedRejector{})

		handler = wrapContext(pipeline, accessor.NewHandler(
			logger,
			"some-action",
			innerHandler,
			fakeAccessor,
			new(auditorfakes.FakeAuditor),
			map[string]string{},
		))
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeAccess, nil)
		server = httptest.NewServer(handler)

		request, err := http.NewRequest("POST", server.URL, nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
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
		BeforeEach(func() {
			pipeline.PublicReturns(false)
		})

		Context("and authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
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
				fakeAccess.IsAuthorizedReturns(false)
			})

			Context("and is authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
				})

				It("returns 403 Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("and not authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(false)
				})

				It("returns 401 Unauthorized", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
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

func wrapContext(pipeline db.Pipeline, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newCtx := context.WithValue(r.Context(), auth.PipelineContextKey, pipeline)
		handler.ServeHTTP(w, r.WithContext(newCtx))
	})
}
