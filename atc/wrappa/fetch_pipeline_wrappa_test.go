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
		fpWrappa        wrappa.FetchPipelineWrappa
		fakeTeamFactory *dbfakes.FakeTeamFactory
		fakeTeam        *dbfakes.FakeTeam
		fakePipeline    *dbfakes.FakePipeline

		server   *httptest.Server
		response *http.Response

		endpoint string
		handler  *spyHandler
	)

	BeforeEach(func() {
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeTeam = new(dbfakes.FakeTeam)
		fakePipeline = new(dbfakes.FakePipeline)
		fpWrappa = wrappa.FetchPipelineWrappa{TeamFactory: fakeTeamFactory}

		handler = &spyHandler{}
	})

	JustBeforeEach(func() {
		handlers := fpWrappa.Wrap(rata.Handlers{endpoint: handler})
		server = httptest.NewServer(handlers[endpoint])

		request, err := http.NewRequest("POST", server.URL+"?:team_name=some-team&:pipeline_name=some-pipeline", nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())

	})

	Describe("fetching by team+pipeline name", func() {
		BeforeEach(func() {
			endpoint = atc.GetPipeline
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
					Expect(fakeTeam.PipelineArgsForCall(0)).To(Equal("some-pipeline"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("calls the scoped handler", func() {
					Expect(handler.called).To(BeTrue())
					Expect(handler.pipeline).To(BeIdenticalTo(fakePipeline))
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
