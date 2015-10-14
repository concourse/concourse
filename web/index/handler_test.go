package index_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
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
			nil,
			true,
			auth.NoopValidator{},
			radarSchedulerFactory,
			db,
			pipelineDBFactory,
			configDB,
			"templatefixtures",
			"../public",
			engine,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	var recorder *httptest.ResponseRecorder

	JustBeforeEach(func() {
		recorder = httptest.NewRecorder()
		req, err := http.NewRequest("GET", "http://concourse.example.com/", nil)
		Expect(err).NotTo(HaveOccurred())

		handler.ServeHTTP(recorder, req)
	})

	Context("when the pipeline lookup fails", func() {
		Context("when there is an unexpected error", func() {
			BeforeEach(func() {
				pipelineDBFactory.BuildDefaultReturns(nil, false, errors.New("nope"))
			})

			It("returns an internal server error", func() {
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("because there are no pipelines", func() {
			BeforeEach(func() {
				pipelineDBFactory.BuildDefaultReturns(nil, false, nil)
			})

			It("is successful", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("renders the index template", func() {
				Expect(recorder.Body).To(ContainSubstring("index"))
			})
		})
	})

	Context("when there is a pipeline", func() {
		BeforeEach(func() {
			pipelineDB := new(dbfakes.FakePipelineDB)
			pipelineDB.GetConfigReturns(atc.Config{}, db.ConfigVersion(1), true, nil)
			pipelineDBFactory.BuildDefaultReturns(pipelineDB, true, nil)
		})

		It("is successful", func() {
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("renders the pipeline template", func() {
			Expect(recorder.Body).To(ContainSubstring("pipeline"))
		})
	})
})
