package db_test

import (
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerArtifactLifecycle", func() {
	var workerArtifactLifecycle db.WorkerArtifactLifecycle

	BeforeEach(func() {
		workerArtifactLifecycle = db.NewArtifactLifecycle(dbConn)
	})

	Describe("RemoveExpiredArtifacts", func() {
		JustBeforeEach(func() {
			err := workerArtifactLifecycle.RemoveExpiredArtifacts()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("removes artifacts created more than 12 hours ago", func() {

			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO worker_artifacts(name, created_at) VALUES('some-name', NOW() - '13 hours'::interval)")
				Expect(err).ToNot(HaveOccurred())
			})

			It("removes the record", func() {
				var count int
				err := dbConn.QueryRow("SELECT count(*) from worker_artifacts").Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(0))
			})
		})

		Context("keeps artifacts for 12 hours", func() {

			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO worker_artifacts(name, created_at) VALUES('some-name', NOW() - '13 hours'::interval)")
				Expect(err).ToNot(HaveOccurred())

				_, err = dbConn.Exec("INSERT INTO worker_artifacts(name, created_at) VALUES('some-other-name', NOW())")
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not remove the record", func() {
				var count int
				err := dbConn.QueryRow("SELECT count(*) from worker_artifacts").Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(1))
			})
		})
	})
})
