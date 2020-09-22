package pipelineserver_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rejected Archived Handler", func() {
	var (
		response *http.Response
		server   *httptest.Server
		delegate *delegateHandler

		fakePipeline *dbfakes.FakePipeline

		handler http.Handler
	)

	BeforeEach(func() {
		delegate = &delegateHandler{}

		fakePipeline = new(dbfakes.FakePipeline)

		handlerFactory := pipelineserver.RejectArchivedHandlerFactory{}
		handler = wrapHandler{
			delegate:        handlerFactory.RejectArchived(delegate.GetHandler(fakePipeline)),
			contextPipeline: fakePipeline,
		}
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
