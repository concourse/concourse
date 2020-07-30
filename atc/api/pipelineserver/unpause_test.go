package pipelineserver_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/api/pipelineserver/pipelineserverfakes"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unpause Handler", func() {
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
		fakeLogger.SessionReturns(fakeLogger)
		server = pipelineserver.NewServer(
			fakeLogger,
			new(dbfakes.FakeTeamFactory),
			new(dbfakes.FakePipelineFactory),
			"",
		)
		dbPipeline = new(dbfakes.FakePipeline)
		handler = server.UnpausePipeline(dbPipeline)
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest("PUT", "http://example.com", nil)
	})

	Context("when there is a database error", func() {
		var expectedError error

		BeforeEach(func() {
			expectedError = errors.New("db error")
			dbPipeline.UnpauseReturns(expectedError)
		})

		It("logs the error", func() {
			handler.ServeHTTP(recorder, request)

			Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
			action, actualError, _ := fakeLogger.ErrorArgsForCall(0)
			Expect(action).To(Equal("failed-to-unpause-pipeline"), "wrong action name")
			Expect(actualError).To(Equal(expectedError))
		})

		It("returns a 500 status code", func() {
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
		})
	})
})
