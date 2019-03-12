package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckBuildReadAccessHandler", func() {
	var (
		response       *http.Response
		server         *httptest.Server
		delegate       *buildDelegateHandler
		buildFactory   *dbfakes.FakeBuildFactory
		handlerFactory auth.CheckBuildReadAccessHandlerFactory
		handler        http.Handler
		fakeAccessor   *accessorfakes.FakeAccessFactory
		fakeaccess     *accessorfakes.FakeAccess
		build          *dbfakes.FakeBuild
		pipeline       *dbfakes.FakePipeline
	)

	BeforeEach(func() {
		buildFactory = new(dbfakes.FakeBuildFactory)
		handlerFactory = auth.NewCheckBuildReadAccessHandlerFactory(buildFactory)
		fakeAccessor = new(accessorfakes.FakeAccessFactory)
		fakeaccess = new(accessorfakes.FakeAccess)

		delegate = &buildDelegateHandler{}

		build = new(dbfakes.FakeBuild)
		pipeline = new(dbfakes.FakePipeline)
		build.PipelineReturns(pipeline, true, nil)
		build.TeamNameReturns("some-team")
		build.JobNameReturns("some-job")
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
		server = httptest.NewServer(handler)

		request, err := http.NewRequest("POST", server.URL+"?:build_id=55", nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	ItReturnsTheBuild := func() {
		It("returns 200 ok", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("calls delegate with the build context", func() {
			Expect(delegate.IsCalled).To(BeTrue())
			Expect(delegate.ContextBuild).To(BeIdenticalTo(build))
		})
	}

	WithExistingBuild := func(buildExistsFunc func()) {
		Context("when build exists", func() {
			BeforeEach(func() {
				buildFactory.BuildReturns(build, true, nil)
			})

			buildExistsFunc()
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
	}

	Context("AnyJobHandler", func() {
		BeforeEach(func() {
			checkBuildReadAccessHandler := handlerFactory.AnyJobHandler(delegate, auth.UnauthorizedRejector{})
			handler = accessor.NewHandler(
				checkBuildReadAccessHandler,
				fakeAccessor,
				"some-action",
				new(auditorfakes.FakeAuditor),
			)
		})

		Context("when authenticated and accessing same team's build", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
			})

			WithExistingBuild(ItReturnsTheBuild)
		})

		Context("when authenticated but accessing different team's build", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(false)
			})

			WithExistingBuild(func() {
				Context("when pipeline is public", func() {
					BeforeEach(func() {
						pipeline.PublicReturns(true)
						build.PipelineReturns(pipeline, true, nil)
					})

					ItReturnsTheBuild()
				})

				Context("when pipeline is private", func() {
					BeforeEach(func() {
						pipeline.PublicReturns(false)
						build.PipelineReturns(pipeline, true, nil)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})
				Context("when fetching pipeline throws error", func() {
					BeforeEach(func() {
						build.PipelineReturns(pipeline, true, errors.New("some-error"))
					})
					It("return 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
				Context("when pipeline is not found", func() {
					BeforeEach(func() {
						build.PipelineReturns(nil, false, nil)
					})
					It("return 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			WithExistingBuild(func() {
				Context("when pipeline is public", func() {
					BeforeEach(func() {
						pipeline.PublicReturns(true)
						build.PipelineReturns(pipeline, true, nil)
					})

					ItReturnsTheBuild()
				})

				Context("when pipeline is private", func() {
					BeforeEach(func() {
						pipeline.PublicReturns(false)
						build.PipelineReturns(pipeline, true, nil)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
				Context("when fetching pipeline throws error", func() {
					BeforeEach(func() {
						build.PipelineReturns(pipeline, true, errors.New("some-error"))
					})
					It("return 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
				Context("when pipeline is not found", func() {
					BeforeEach(func() {
						build.PipelineReturns(nil, false, nil)
					})
					It("return 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
			})
		})
	})

	Context("CheckIfPrivateJobHandler", func() {
		var fakeJob *dbfakes.FakeJob

		BeforeEach(func() {
			fakeJob = new(dbfakes.FakeJob)
			checkBuildReadAccessHandler := handlerFactory.CheckIfPrivateJobHandler(delegate, auth.UnauthorizedRejector{})
			handler = accessor.NewHandler(
				checkBuildReadAccessHandler,
				fakeAccessor,
				"some-action",
				new(auditorfakes.FakeAuditor),
			)
		})

		ItChecksIfJobIsPrivate := func(status int) {
			Context("when pipeline is public", func() {
				BeforeEach(func() {
					pipeline.PublicReturns(true)
					build.PipelineReturns(pipeline, true, nil)
				})

				Context("and job is public", func() {
					BeforeEach(func() {
						fakeJob.NameReturns("some-job")
						fakeJob.ConfigReturns(atc.JobConfig{
							Name:   "some-job",
							Public: true,
						})

						pipeline.JobReturns(fakeJob, true, nil)
					})

					ItReturnsTheBuild()
				})

				Context("and job is private", func() {
					BeforeEach(func() {
						fakeJob.NameReturns("some-job")
						fakeJob.ConfigReturns(atc.JobConfig{
							Name:   "some-job",
							Public: false,
						})

						pipeline.JobReturns(fakeJob, true, nil)
					})

					It("returns "+string(status), func() {
						Expect(response.StatusCode).To(Equal(status))
					})
				})

				Context("getting the job fails", func() {
					BeforeEach(func() {
						pipeline.JobReturns(nil, false, errors.New("error"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the job is not found", func() {
					BeforeEach(func() {
						pipeline.JobReturns(nil, false, nil)
					})

					It("returns not found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})

			Context("when pipeline is private", func() {
				BeforeEach(func() {
					pipeline.PublicReturns(false)
					build.PipelineReturns(pipeline, true, nil)
				})

				It("returns "+string(status), func() {
					Expect(response.StatusCode).To(Equal(status))
				})
			})
		}

		Context("when authenticated and accessing same team's build", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
			})

			WithExistingBuild(ItReturnsTheBuild)
		})

		Context("when authenticated but accessing different team's build", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(false)
			})

			WithExistingBuild(func() {
				ItChecksIfJobIsPrivate(http.StatusForbidden)
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			WithExistingBuild(func() {
				ItChecksIfJobIsPrivate(http.StatusUnauthorized)
			})
		})
	})
})

type buildDelegateHandler struct {
	IsCalled     bool
	ContextBuild db.Build
}

func (handler *buildDelegateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.IsCalled = true
	handler.ContextBuild = r.Context().Value(auth.BuildContextKey).(db.Build)
}
