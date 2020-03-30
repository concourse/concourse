package pipelineserver_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/api/pipelineserver/pipelineserverfakes"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate counterfeiter code.cloudfoundry.org/lager.Logger

var _ = Describe("Archive Handler", func() {
	var (
		fakeLogger *pipelineserverfakes.FakeLogger
		server     *pipelineserver.Server
		dbPipeline *dbfakes.FakePipeline
		handler    http.Handler
		recorder   *httptest.ResponseRecorder
		request    *http.Request
	)

	BeforeEach(func() {
		fakeLogger = new(pipelineserverfakes.FakeLogger)
		server = pipelineserver.NewServer(
			fakeLogger,
			new(dbfakes.FakeTeamFactory),
			new(dbfakes.FakePipelineFactory),
			"",
			true, /* enableArchivePipeline */
		)
		dbPipeline = new(dbfakes.FakePipeline)
		handler = server.ArchivePipeline(dbPipeline)
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest("PUT", "http://example.com", nil)
	})

	It("logs database errors", func() {
		expectedError := errors.New("db error")
		dbPipeline.ArchiveReturns(expectedError)

		handler.ServeHTTP(recorder, request)

		Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
		action, actualError, _ := fakeLogger.ErrorArgsForCall(0)
		Expect(action).To(Equal("archive-pipeline"), "wrong action name")
		Expect(actualError).To(Equal(expectedError))
	})

	It("write a debug log on every request", func() {
		handler.ServeHTTP(recorder, request)

		Expect(fakeLogger.DebugCallCount()).To(Equal(1))
		action, _ := fakeLogger.DebugArgsForCall(0)
		Expect(action).To(Equal("archive-pipeline"), "wrong action name")
	})

	It("logs no errors if everything works", func() {
		dbPipeline.ArchiveReturns(nil)

		handler.ServeHTTP(recorder, request)

		Expect(fakeLogger.ErrorCallCount()).To(Equal(0))
	})

	Context("when the endpoint is not enabled", func() {
		BeforeEach(func() {
			server = pipelineserver.NewServer(
				fakeLogger,
				new(dbfakes.FakeTeamFactory),
				new(dbfakes.FakePipelineFactory),
				"",
				false, /* enableArchivePipeline */
			)
			handler = server.ArchivePipeline(dbPipeline)
		})

		It("responds with status Forbidden", func() {
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusForbidden))
			body, _ := ioutil.ReadAll(recorder.Body)
			Expect(body).To(Equal([]byte("endpoint is not enabled\n")))
		})
	})
})
