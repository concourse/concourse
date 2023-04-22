package auth_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo/v2"
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
		build          *dbfakes.FakeBuildForAPI
		pipeline       *dbfakes.FakePipeline
	)

	BeforeEach(func() {
		buildFactory = new(dbfakes.FakeBuildFactory)
		handlerFactory = auth.NewCheckBuildReadAccessHandlerFactory(buildFactory)
		fakeAccessor = new(accessorfakes.FakeAccessFactory)
		fakeaccess = new(accessorfakes.FakeAccess)

		delegate = &buildDelegateHandler{}

		build = new(dbfakes.FakeBuildForAPI)
		pipeline = new(dbfakes.FakePipeline)
		build.PipelineIDReturns(41)
		build.PipelineReturns(pipeline, true, nil)
		build.TeamIDReturns(42)
		build.TeamNameReturns("some-team")
		build.AllAssociatedTeamNamesReturns([]string{"some-team"})
		build.JobIDReturns(43)
		build.JobNameReturns("some-job")
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess, nil)
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
				buildFactory.BuildForAPIReturns(build, true, nil)
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
				buildFactory.BuildForAPIReturns(nil, false, errors.New("disaster"))
			})

			It("returns 503", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	}

	Context("AnyJobHandler", func() {
		BeforeEach(func() {
			innerHandler := handlerFactory.AnyJobHandler(delegate, auth.UnauthorizedRejector{})

			handler = accessor.NewHandler(
				logger,
				"some-action",
				innerHandler,
				fakeAccessor,
				new(auditorfakes.FakeAuditor),
				map[string]string{},
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
				Context("when the build is not for a pipeline", func() {
					BeforeEach(func() {
						build.PipelineIDReturns(0)
					})
					It("return 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})
				Context("when pipeline is not found", func() {
					BeforeEach(func() {
						build.PipelineReturns(nil, false, nil)
					})
					It("return 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
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
				Context("when the build is not for a pipeline", func() {
					BeforeEach(func() {
						build.PipelineIDReturns(0)
					})
					It("return 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
				Context("when pipeline is not found", func() {
					BeforeEach(func() {
						build.PipelineReturns(nil, false, nil)
					})
					It("return 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})
		})
	})

	Context("CheckIfPrivateJobHandler", func() {
		var fakeJob *dbfakes.FakeJob

		BeforeEach(func() {
			fakeJob = new(dbfakes.FakeJob)
			innerHandler := handlerFactory.CheckIfPrivateJobHandler(delegate, auth.UnauthorizedRejector{})

			handler = accessor.NewHandler(
				logger,
				"some-action",
				innerHandler,
				fakeAccessor,
				new(auditorfakes.FakeAuditor),
				map[string]string{},
			)
		})

		ItChecksIfJobIsPrivate := func(status int) {
			Context("when pipeline is public", func() {
				BeforeEach(func() {
					pipeline.PublicReturns(true)
					build.PipelineReturns(pipeline, true, nil)
				})

				Context("when the build is not for a job", func() {
					BeforeEach(func() {
						build.JobIDReturns(0)
						build.JobNameReturns("")
					})

					It("returns "+fmt.Sprint(status), func() {
						Expect(response.StatusCode).To(Equal(status))
					})
				})

				Context("and job is public", func() {
					BeforeEach(func() {
						fakeJob.NameReturns("some-job")
						fakeJob.PublicReturns(true)

						pipeline.JobReturns(fakeJob, true, nil)
					})

					ItReturnsTheBuild()
				})

				Context("and job is private", func() {
					BeforeEach(func() {
						fakeJob.NameReturns("some-job")
						fakeJob.PublicReturns(false)

						pipeline.JobReturns(fakeJob, true, nil)
					})

					It("returns "+fmt.Sprint(status), func() {
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

					It("return 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})

			Context("when pipeline is private", func() {
				BeforeEach(func() {
					pipeline.PublicReturns(false)
					build.PipelineReturns(pipeline, true, nil)
				})

				It("returns "+fmt.Sprint(status), func() {
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
	ContextBuild db.BuildForAPI
}

func (handler *buildDelegateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.IsCalled = true
	handler.ContextBuild = r.Context().Value(auth.BuildContextKey).(db.BuildForAPI)
}
