package db_test

import (
	"database/sql"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Contracts", func() {
	var (
		dbConn   *sql.DB
		listener *pq.Listener

		pipelineDBFactory db.PipelineDBFactory
		sqlDB             *db.SQLDB

		pipelineDB db.PipelineDB
	)

	BeforeEach(func() {
		postgresRunner.CreateTestDB()

		dbConn = postgresRunner.Open()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener)

		sqlDB = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, sqlDB)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Ω(err).ShouldNot(HaveOccurred())

		err = listener.Close()
		Ω(err).ShouldNot(HaveOccurred())

		postgresRunner.DropTestDB()
	})

	pipelineConfig := atc.Config{
		Resources: atc.ResourceConfigs{
			{
				Name: "some-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
		},
	}

	BeforeEach(func() {
		_, err := sqlDB.SaveConfig("pipeline-name", pipelineConfig, 0, db.PipelineUnpaused)
		Ω(err).ShouldNot(HaveOccurred())

		savedPipeline, err := sqlDB.GetPipelineByName("pipeline-name")
		Ω(err).ShouldNot(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
	})

	Describe("taking out a lease on resource checking", func() {
		BeforeEach(func() {
			_, err := pipelineDB.GetResource("some-resource")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when there has been a check recently", func() {
			It("does not get the contract", func() {
				contract, leased, err := pipelineDB.LeaseCheck("some-resource", 1*time.Second)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(leased).Should(BeTrue())

				contract.Break()

				_, leased, err = pipelineDB.LeaseCheck("some-resource", 1*time.Second)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(leased).Should(BeFalse())
			})
		})

		Context("when there has not been a check recently", func() {
			It("gets and keeps the contract and stops others from getting it", func() {
				contract, leased, err := pipelineDB.LeaseCheck("some-resource", 1*time.Second)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(leased).Should(BeTrue())

				Consistently(func() bool {
					_, leased, err = pipelineDB.LeaseCheck("some-resource", 1*time.Second)
					Ω(err).ShouldNot(HaveOccurred())

					return leased
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				contract.Break()

				time.Sleep(600 * time.Millisecond)

				newContract, leased, err := pipelineDB.LeaseCheck("some-resource", 1*time.Second)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(leased).Should(BeTrue())

				newContract.Break()
			})
		})
	})
})
