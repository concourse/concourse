package pipelineserver_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
)

//go:generate counterfeiter code.cloudfoundry.org/lager.Logger

var _ = Describe("Archive Handler", func() {
	var (
		fakeLogger *lagertest.TestLogger
		server     *pipelineserver.Server
		dbPipeline *dbfakes.FakePipeline
		handler    http.Handler
		recorder   *httptest.ResponseRecorder
		request    *http.Request
	)

	BeforeEach(func() {
		fakeLogger = lagertest.NewTestLogger("test")
		server = pipelineserver.NewServer(
			fakeLogger,
			new(dbfakes.FakeTeamFactory),
			new(dbfakes.FakePipelineFactory),
			"",
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

		Expect(fakeLogger.Logs()).To(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Message":  Equal("test.archive-pipeline"),
			"LogLevel": Equal(lager.ERROR),
		})))
	})
	It("write a debug log on every request", func() {
		handler.ServeHTTP(recorder, request)

		Expect(fakeLogger.Logs()).To(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Message":  Equal("test.archive-pipeline"),
			"LogLevel": Equal(lager.DEBUG),
		})))
	})

	It("logs no errors if everything works", func() {
		dbPipeline.ArchiveReturns(nil)

		handler.ServeHTTP(recorder, request)

		Expect(fakeLogger.Logs()).ToNot(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Message":  Equal("test.archive-pipeline"),
			"LogLevel": Equal(lager.ERROR),
		})))
	})
})
