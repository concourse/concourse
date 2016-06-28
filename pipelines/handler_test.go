package pipelines_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/pipelines"

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

		authValidator     *authfakes.FakeValidator
		userContextReader *authfakes.FakeUserContextReader
		allowsPublic      bool
	)

	BeforeEach(func() {
		teamDBFactory = new(dbfakes.FakeTeamDBFactory)
		teamDB = new(dbfakes.FakeTeamDB)
		teamDBFactory.GetTeamDBReturns(teamDB)

		pipelineDB = new(dbfakes.FakePipelineDB)
		delegate = &delegateHandler{}
		authValidator = new(authfakes.FakeValidator)
		userContextReader = new(authfakes.FakeUserContextReader)
	})

	JustBeforeEach(func() {
		pipelineDBFactory := new(dbfakes.FakePipelineDBFactory)
		pipelineDBFactory.BuildReturns(pipelineDB)

		handlerFactory := NewHandlerFactory(pipelineDBFactory, teamDBFactory)
		handler := handlerFactory.HandlerFor(delegate.GetHandler, allowsPublic)

		authHandler := auth.WrapHandler(handler, authValidator, userContextReader)
		server = httptest.NewServer(authHandler)

		var err error
		response, err = http.PostForm(server.URL+"?:team_name=some-team",
			url.Values{
				":pipeline_name": {"some-pipeline"},
				":team_name":     {"some-team"},
			},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	Context("when authenticated", func() {
		Context("and authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", 42, true, true)
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

		Context("but not authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("", 42, true, true)
			})

			Context("and allows public", func() {
				BeforeEach(func() {
					allowsPublic = true
				})

				It("looks up the team by the right name", func() {
					Expect(teamDBFactory.GetTeamDBCallCount()).To(Equal(1))
					Expect(teamDBFactory.GetTeamDBArgsForCall(0)).To(Equal("some-team"))
				})

				It("looks up the pipeline by the right name", func() {
					Expect(teamDB.GetPipelineByNameCallCount()).To(Equal(1))
					Expect(teamDB.GetPipelineByNameArgsForCall(0)).To(Equal("some-pipeline"))
				})

				Context("and pipeline is public", func() {
					BeforeEach(func() {
						pipelineDB.IsPublicReturns(true)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("calls the scoped handler", func() {
						Expect(delegate.IsCalled).To(BeTrue())
					})
				})

				Context("and pipeline is not public", func() {
					BeforeEach(func() {
						pipelineDB.IsPublicReturns(false)
					})

					It("returns 403 forbidden", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})

					It("does not call the scoped handler", func() {
						Expect(delegate.IsCalled).To(BeFalse())
					})
				})
			})

			Context("and does not allow public", func() {
				BeforeEach(func() {
					allowsPublic = false
				})

				It("doesn't bother looking up the team", func() {
					Expect(teamDBFactory.GetTeamDBCallCount()).To(BeZero())
				})

				It("returns 403 forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})

				It("does not call the scoped handler", func() {
					Expect(delegate.IsCalled).To(BeFalse())
				})
			})
		})
	})

	Context("when not authenticated", func() {
		BeforeEach(func() {
			authValidator.IsAuthenticatedReturns(false)
			userContextReader.GetTeamReturns("", 0, false, false)
		})

		Context("and allows public", func() {
			BeforeEach(func() {
				allowsPublic = true
			})

			It("looks up the team by the right name", func() {
				Expect(teamDBFactory.GetTeamDBCallCount()).To(Equal(1))
				Expect(teamDBFactory.GetTeamDBArgsForCall(0)).To(Equal("some-team"))
			})

			It("looks up the pipeline by the right name", func() {
				Expect(teamDB.GetPipelineByNameCallCount()).To(Equal(1))
				Expect(teamDB.GetPipelineByNameArgsForCall(0)).To(Equal("some-pipeline"))
			})

			Context("and pipeline is public", func() {
				BeforeEach(func() {
					pipelineDB.IsPublicReturns(true)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("calls the scoped handler", func() {
					Expect(delegate.IsCalled).To(BeTrue())
				})
			})

			Context("and pipeline is not public", func() {
				BeforeEach(func() {
					pipelineDB.IsPublicReturns(false)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})

				It("does not call the scoped handler", func() {
					Expect(delegate.IsCalled).To(BeFalse())
				})
			})
		})

		Context("and does not allow public", func() {
			BeforeEach(func() {
				allowsPublic = false
			})

			It("doesn't bother looking up the team", func() {
				Expect(teamDBFactory.GetTeamDBCallCount()).To(BeZero())
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not call the scoped handler", func() {
				Expect(delegate.IsCalled).To(BeFalse())
			})
		})
	})
})

type delegateHandler struct {
	IsCalled bool
}

func (handler *delegateHandler) GetHandler(db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.IsCalled = true
	})
}
