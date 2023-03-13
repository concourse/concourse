package pipelineserver_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rejected Archived Handler", func() {
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

		handlerFactory := pipelineserver.NewRejectArchivedHandlerFactory(dbTeamFactory)
		handler = handlerFactory.RejectArchived(delegate.GetHandler(fakePipeline))
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

	Context("when a team is found", func() {
		BeforeEach(func() {
			dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
		})
		Context("when a pipeline is found", func() {
			BeforeEach(func() {
				fakeTeam.PipelineReturns(fakePipeline, true, nil)
			})
			Context("when a pipeline is archived", func() {
				BeforeEach(func() {
					fakePipeline.ArchivedReturns(true)
				})
				It("returns 409", func() {
					Expect(response.StatusCode).To(Equal(http.StatusConflict))
				})
				It("returns an error in the body", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(body).To(ContainSubstring("action not allowed for an archived pipeline"))
				})
			})
			Context("when a pipeline is not archived", func() {
				BeforeEach(func() {
					fakePipeline.ArchivedReturns(false)
				})
				It("returns the delegate handler", func() {
					Expect(delegate.IsCalled).To(BeTrue())
				})
			})
		})
		Context("when a pipeline is not found", func() {
			BeforeEach(func() {
				fakeTeam.PipelineReturns(nil, false, nil)
			})
			It("returns 404", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
		Context("when getting a pipeline returns an error", func() {
			BeforeEach(func() {
				fakeTeam.PipelineReturns(nil, false, errors.New("some error"))
			})
			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
	Context("when a team is not found", func() {
		BeforeEach(func() {
			dbTeamFactory.FindTeamReturns(nil, false, nil)
		})
		It("returns 404", func() {
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})
	})
	Context("when finding a team returns an error", func() {
		BeforeEach(func() {
			dbTeamFactory.FindTeamReturns(nil, false, errors.New("some error"))
		})
		It("returns 500", func() {
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
		})
	})
})
