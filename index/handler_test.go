package index_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	enginefakes "github.com/concourse/atc/engine/fakes"
	pipelinefakes "github.com/concourse/atc/pipelines/fakes"
	webfakes "github.com/concourse/atc/web/fakes"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/web"
)

var _ = Describe("Handler", func() {
	var pipelineDBFactory *dbfakes.FakePipelineDBFactory
	var handler http.Handler

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("web")
		radarSchedulerFactory := new(pipelinefakes.FakeRadarSchedulerFactory)
		db := new(webfakes.FakeWebDB)
		pipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
		configDB := new(dbfakes.FakeConfigDB)
		engine := new(enginefakes.FakeEngine)

		var err error
		handler, err = web.NewHandler(
			logger,
			auth.NoopValidator{},
			radarSchedulerFactory,
			db,
			pipelineDBFactory,
			configDB,
			"templatefixtures",
			"../public",
			engine,
		)
		Ω(err).ShouldNot(HaveOccurred())
	})

	var recorder *httptest.ResponseRecorder

	JustBeforeEach(func() {
		recorder = httptest.NewRecorder()
		req, err := http.NewRequest("GET", "http://concourse.example.com/", nil)
		Ω(err).ShouldNot(HaveOccurred())

		handler.ServeHTTP(recorder, req)
	})

	Context("when the pipeline lookup fails", func() {
		Context("when there is an unexpected error", func() {
			BeforeEach(func() {
				pipelineDBFactory.BuildDefaultReturns(nil, errors.New("nope"))
			})

			It("returns an internal server error", func() {
				Ω(recorder.Code).Should(Equal(http.StatusInternalServerError))
			})
		})

		Context("because there are no pipelines", func() {
			BeforeEach(func() {
				pipelineDBFactory.BuildDefaultReturns(nil, db.ErrNoPipelines)
			})

			It("is successful", func() {
				Ω(recorder.Code).Should(Equal(http.StatusOK))
			})

			It("renders the index template", func() {
				Ω(recorder.Body).Should(ContainSubstring("index"))
			})
		})
	})

	Context("when there is a pipeline", func() {
		BeforeEach(func() {
			pipelineDB := new(dbfakes.FakePipelineDB)
			pipelineDBFactory.BuildDefaultReturns(pipelineDB, nil)
		})

		It("is successful", func() {
			Ω(recorder.Code).Should(Equal(http.StatusOK))
		})

		It("renders the pipeline template", func() {
			Ω(recorder.Body).Should(ContainSubstring("pipeline"))
		})
	})
})
