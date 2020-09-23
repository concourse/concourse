package wrappa_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetchPipelineWrappa", func() {
	var (
		fpWrappa            wrappa.FetchPipelineWrappa
		fakeTeamFactory     *dbfakes.FakeTeamFactory
		fakePipelineFactory *dbfakes.FakePipelineFactory
		fakeTeam            *dbfakes.FakeTeam
		fakePipeline        *dbfakes.FakePipeline

		server   *httptest.Server
		response *http.Response

		params   string
		endpoint string
		handler  *spyHandler
	)

	BeforeEach(func() {
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakePipelineFactory = new(dbfakes.FakePipelineFactory)
		fakeTeam = new(dbfakes.FakeTeam)
		fakePipeline = new(dbfakes.FakePipeline)
		fpWrappa = wrappa.FetchPipelineWrappa{
			TeamFactory:     fakeTeamFactory,
			PipelineFactory: fakePipelineFactory,
		}

		handler = &spyHandler{}
	})

	JustBeforeEach(func() {
		handlers := fpWrappa.Wrap(rata.Handlers{endpoint: handler})
		server = httptest.NewServer(handlers[endpoint])

		request, err := http.NewRequest("POST", server.URL+"?"+params, nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("fetching by pipeline id", func() {
		BeforeEach(func() {
			endpoint = atc.GetPipelineByPipelineID
			params = ":pipeline_id=100"
		})

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				fakePipelineFactory.GetPipelineReturns(fakePipeline, true, nil)
				fakePipeline.IDReturns(100)
			})

			It("looks up the pipeline by the right id", func() {
				Expect(fakePipelineFactory.GetPipelineCallCount()).To(Equal(1))
				Expect(fakePipelineFactory.GetPipelineArgsForCall(0)).To(Equal(100))
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("stores the pipeline on the request context", func() {
				Expect(handler.called).To(BeTrue())
				Expect(handler.pipeline).To(BeIdenticalTo(fakePipeline))
			})
		})

		Context("when the pipeline does not exist", func() {
			BeforeEach(func() {
				fakePipelineFactory.GetPipelineReturns(nil, false, nil)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})

			It("does not call the scoped handler", func() {
				Expect(handler.called).To(BeFalse())
			})
		})

		Context("when finding the pipeline fails", func() {
			BeforeEach(func() {
				fakePipelineFactory.GetPipelineReturns(nil, false, errors.New("error"))
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("does not call the scoped handler", func() {
				Expect(handler.called).To(BeFalse())
			})
		})
	})

	Describe("fetching by team name+pipeline ref", func() {
		BeforeEach(func() {
			endpoint = atc.GetPipeline
			params = ":team_name=some-team&:pipeline_name=some-pipeline"
		})

		Context("when the team exists", func() {
			BeforeEach(func() {
				fakeTeam.NameReturns("some-team")
				fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			It("looks up the team by the right name", func() {
				Expect(fakeTeamFactory.FindTeamCallCount()).To(Equal(1))
				Expect(fakeTeamFactory.FindTeamArgsForCall(0)).To(Equal("some-team"))
			})

			Context("when the pipeline exists", func() {
				BeforeEach(func() {
					fakePipeline.NameReturns("some-pipeline")
					fakeTeam.PipelineReturns(fakePipeline, true, nil)
				})

				It("looks up the pipeline by the right name", func() {
					Expect(fakeTeam.PipelineCallCount()).To(Equal(1))
					Expect(fakeTeam.PipelineArgsForCall(0)).To(Equal(atc.PipelineRef{Name: "some-pipeline"}))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("stores the pipeline on the request context", func() {
					Expect(handler.called).To(BeTrue())
					Expect(handler.pipeline).To(BeIdenticalTo(fakePipeline))
				})

				Context("when querying using instance vars", func() {
					Context("when the instance vars are valid JSON", func() {
						BeforeEach(func() {
							params += "&instance_vars=%7B%22some%22%3A%20%22var%22%7D"
						})

						It("succeeds", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("looks up the pipeline using the instance vars", func() {
							Expect(fakeTeam.PipelineArgsForCall(0)).To(Equal(atc.PipelineRef{
								Name: "some-pipeline",
								InstanceVars: atc.InstanceVars{
									"some": "var",
								},
							}))
						})
					})
					Context("when the instance vars are invalid JSON", func() {
						BeforeEach(func() {
							params += "&instance_vars=blah"
						})

						It("returns a bad request error", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})
					})
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
					Expect(handler.called).To(BeFalse())
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
					Expect(handler.called).To(BeFalse())
				})
			})
		})

		Context("when the team does not exist", func() {
			BeforeEach(func() {
				fakeTeamFactory.FindTeamReturns(nil, false, nil)
			})

			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})

			It("does not call the scoped handler", func() {
				Expect(handler.called).To(BeFalse())
			})
		})

		Context("when finding the team fails", func() {
			BeforeEach(func() {
				fakeTeamFactory.FindTeamReturns(nil, false, errors.New("error"))
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("does not call the scoped handler", func() {
				Expect(handler.called).To(BeFalse())
			})
		})
	})
})

type spyHandler struct {
	pipeline db.Pipeline
	called   bool
}

func (s *spyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.pipeline = r.Context().Value(auth.PipelineContextKey).(db.Pipeline)
	s.called = true
}
