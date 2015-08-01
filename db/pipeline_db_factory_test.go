package db_test

import (
	"database/sql"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/fakes"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineDBFactory", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory

	var pipelinesDB *fakes.FakePipelinesDB

	BeforeEach(func() {
		postgresRunner.CreateTestDB()

		dbConn = postgresRunner.Open()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener)

		pipelinesDB = new(fakes.FakePipelinesDB)

		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, pipelinesDB)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		err = listener.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
	})

	Describe("default pipeline", func() {
		It("is the first one returned from the DB", func() {
			savedPipelineOne := db.SavedPipeline{
				ID: 1,
				Pipeline: db.Pipeline{
					Name: "a-pipeline",
				},
			}

			savedPipelineTwo := db.SavedPipeline{
				ID: 2,
				Pipeline: db.Pipeline{
					Name: "another-pipeline",
				},
			}

			pipelinesDB.GetAllActivePipelinesReturns([]db.SavedPipeline{
				savedPipelineOne,
				savedPipelineTwo,
			}, nil)

			defaultPipelineDB, err := pipelineDBFactory.BuildDefault()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(defaultPipelineDB.GetPipelineName()).Should(Equal("a-pipeline"))
		})

		Context("when there are no pipelines", func() {
			BeforeEach(func() {
				pipelinesDB.GetAllActivePipelinesReturns([]db.SavedPipeline{}, nil)
			})

			It("returns a useful error if there are no pipelines", func() {
				_, err := pipelineDBFactory.BuildDefault()
				Ω(err).Should(MatchError(db.ErrNoPipelines))
			})
		})
	})
})
