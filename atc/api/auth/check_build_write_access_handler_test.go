package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/api/accessor"
	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckBuildWriteAccessHandler", func() {
	var (
		response       *http.Response
		server         *httptest.Server
		delegate       *buildDelegateHandler
		buildFactory   *dbfakes.FakeBuildFactory
		handlerFactory auth.CheckBuildWriteAccessHandlerFactory
		handler        http.Handler
		fakeAccessor   *accessorfakes.FakeAccessFactory
		fakeaccess     *accessorfakes.FakeAccess
		build          *dbfakes.FakeBuild
		pipeline       *dbfakes.FakePipeline
	)

	BeforeEach(func() {
		buildFactory = new(dbfakes.FakeBuildFactory)
		handlerFactory = auth.NewCheckBuildWriteAccessHandlerFactory(buildFactory)
		fakeAccessor = new(accessorfakes.FakeAccessFactory)
		fakeaccess = new(accessorfakes.FakeAccess)

		delegate = &buildDelegateHandler{}

		build = new(dbfakes.FakeBuild)
		pipeline = new(dbfakes.FakePipeline)
		build.PipelineReturns(pipeline, true, nil)
		build.TeamNameReturns("some-team")
		build.JobNameReturns("some-job")

		checkBuildWriteAccessHandler := handlerFactory.HandlerFor(delegate, auth.UnauthorizedRejector{})
		handler = accessor.NewHandler(checkBuildWriteAccessHandler, fakeAccessor)
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
		server = httptest.NewServer(handler)

		request, err := http.NewRequest("POST", server.URL+"?:team_name=some-team&:build_id=55", nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	Context("when authenticated and accessing same team's build", func() {
		BeforeEach(func() {
			fakeaccess.IsAuthenticatedReturns(true)
			fakeaccess.IsAuthorizedReturns(true)
		})

		Context("when build exists", func() {
			BeforeEach(func() {
				buildFactory.BuildReturns(build, true, nil)
			})

			It("returns 200 ok", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("calls delegate with the build context", func() {
				Expect(delegate.IsCalled).To(BeTrue())
				Expect(delegate.ContextBuild).To(BeIdenticalTo(build))
			})
		})

		Context("when build is not found", func() {
			BeforeEach(func() {
				buildFactory.BuildReturns(nil, false, nil)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when getting build fails", func() {
			BeforeEach(func() {
				buildFactory.BuildReturns(nil, false, errors.New("disaster"))
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Context("when authenticated but accessing different team's build", func() {
		BeforeEach(func() {
			fakeaccess.IsAuthenticatedReturns(true)
			fakeaccess.IsAuthorizedReturns(false)
			buildFactory.BuildReturns(build, true, nil)
		})

		It("returns 403", func() {
			Expect(response.StatusCode).To(Equal(http.StatusForbidden))
		})
	})

	Context("when not authenticated", func() {
		BeforeEach(func() {
			fakeaccess.IsAuthenticatedReturns(false)
		})

		It("returns 401", func() {
			Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})
})
