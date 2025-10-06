package jobserver_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/atc/api/jobserver"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
)

var _ = Describe("JobBadge Handler", func() {
	var (
		fakeLogger        *lagertest.TestLogger
		server            *jobserver.Server
		dbPipeline        *dbfakes.FakePipeline
		dbJob             *dbfakes.FakeJob
		dbBuild           *dbfakes.FakeBuild
		fakeSecretManager *credsfakes.FakeSecrets
		fakeJobFactory    *dbfakes.FakeJobFactory
		fakeCheckFactory  *dbfakes.FakeCheckFactory
		handler           http.Handler
		recorder          *httptest.ResponseRecorder
		request           *http.Request
	)

	BeforeEach(func() {
		fakeLogger = lagertest.NewTestLogger("test")
		fakeSecretManager = new(credsfakes.FakeSecrets)
		fakeJobFactory = new(dbfakes.FakeJobFactory)
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)
		server = jobserver.NewServer(
			fakeLogger,
			"",
			fakeSecretManager,
			fakeJobFactory,
			fakeCheckFactory,
		)
		dbPipeline = new(dbfakes.FakePipeline)
		dbJob = new(dbfakes.FakeJob)
		dbBuild = new(dbfakes.FakeBuild)
		handler = server.JobBadge(dbPipeline)
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest("GET", "http://example.com?job_name=test-job", nil)
	})

	It("returns passing badge for succeeded build", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusSucceeded)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Header().Get("Content-type")).To(Equal("image/svg+xml"))
		Expect(recorder.Body.String()).To(ContainSubstring("passing"))
		Expect(recorder.Body.String()).To(ContainSubstring("#44cc11"))
	})

	It("returns failing badge for failed build", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusFailed)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(ContainSubstring("failing"))
		Expect(recorder.Body.String()).To(ContainSubstring("#e05d44"))
	})

	It("returns aborted badge for aborted build", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusAborted)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(ContainSubstring("aborted"))
		Expect(recorder.Body.String()).To(ContainSubstring("#8f4b2d"))
	})

	It("returns errored badge for errored build", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusErrored)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(ContainSubstring("errored"))
		Expect(recorder.Body.String()).To(ContainSubstring("#fe7d37"))
	})

	It("returns unknown badge when no build exists", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbJob.FinishedAndNextBuildReturns(nil, nil, nil)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(ContainSubstring("unknown"))
		Expect(recorder.Body.String()).To(ContainSubstring("#9f9f9f"))
	})

	It("uses custom title from query parameter", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusSucceeded)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)
		request = httptest.NewRequest("GET", "http://example.com?job_name=test-job&title=custom", nil)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(ContainSubstring("custom"))
	})

	It("sets no-cache headers", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Header().Get("Cache-Control")).To(Equal("no-cache, no-store, must-revalidate"))
		Expect(recorder.Header().Get("Expires")).To(Equal("0"))
	})

	It("returns 404 when job not found", func() {
		dbPipeline.JobReturns(nil, false, nil)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusNotFound))
	})

	It("logs error and returns 500 when finding job fails", func() {
		expectedError := errors.New("db error finding job")
		dbPipeline.JobReturns(nil, false, expectedError)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
		Expect(fakeLogger.Logs()).To(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Message":  Equal("test.job-badge.error-finding-job"),
			"LogLevel": Equal(lager.ERROR),
		})))
	})

	It("logs error and returns 500 when getting build fails", func() {
		expectedError := errors.New("db error getting build")
		dbPipeline.JobReturns(dbJob, true, nil)
		dbJob.FinishedAndNextBuildReturns(nil, nil, expectedError)

		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
		Expect(fakeLogger.Logs()).To(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Message":  Equal("test.job-badge.could-not-get-job-finished-and-next-build"),
			"LogLevel": Equal(lager.ERROR),
		})))
	})

	It("scales badge correctly for custom titles", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusSucceeded)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		request = httptest.NewRequest("GET", "http://example.com?job_name=test-job&title=production-deployment", nil)
		handler.ServeHTTP(recorder, request)

		svg := recorder.Body.String()

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(svg).To(ContainSubstring("production-deployment"))
		Expect(svg).To(ContainSubstring("passing"))
		Expect(svg).To(MatchRegexp(`width="\d{3}"`))
	})

	It("keeps default badge dimensions when no custom title", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusSucceeded)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		handler.ServeHTTP(recorder, request)

		svg := recorder.Body.String()

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(svg).To(ContainSubstring(`width="88"`))
		Expect(svg).To(ContainSubstring(`d="M0 0h37v20H0z"`))
		Expect(svg).To(ContainSubstring("build"))
	})

	It("scales short custom titles with padding", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusSucceeded)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		request = httptest.NewRequest("GET", "http://example.com?job_name=test-job&title=test", nil)
		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(ContainSubstring("test"))
		Expect(recorder.Body.String()).To(ContainSubstring(`width="87"`))
	})

	It("scales medium custom titles with reduced padding", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusSucceeded)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		request = httptest.NewRequest("GET", "http://example.com?job_name=test-job&title=integration", nil)
		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(ContainSubstring("integration"))
		Expect(recorder.Body.String()).To(ContainSubstring(`width="123"`))
	})

	It("scales long custom titles with no padding", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusSucceeded)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		request = httptest.NewRequest("GET", "http://example.com?job_name=test-job&title=very-long-deployment-name", nil)
		handler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusOK))
		Expect(recorder.Body.String()).To(ContainSubstring("very-long-deployment-name"))
		Expect(recorder.Body.String()).To(ContainSubstring(`width="201"`))
	})

	It("preserves original status width for custom titles", func() {
		dbPipeline.JobReturns(dbJob, true, nil)
		dbBuild.StatusReturns(db.BuildStatusSucceeded)
		dbJob.FinishedAndNextBuildReturns(dbBuild, nil, nil)

		request = httptest.NewRequest("GET", "http://example.com?job_name=test-job&title=custom", nil)
		handler.ServeHTTP(recorder, request)

		Expect(recorder.Body.String()).To(MatchRegexp(`d="M48 0h51v20H48z"`))
	})
})
