package db_test

import (
	"time"

	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckLifecycle", func() {
	var (
		checkLifecycle db.CheckLifecycle
		removedChecks  int
		err            error
	)

	BeforeEach(func() {
		checkLifecycle = db.NewCheckLifecycle(dbConn)
	})

	Describe("RemoveExpiredChecks", func() {
		JustBeforeEach(func() {
			removedChecks, err = checkLifecycle.RemoveExpiredChecks(time.Hour * 24)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("removes checks created more than 24 hours ago", func() {

			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO checks(schema, status, create_time) VALUES('some-schema', 'succeeded', NOW() - '25 hours'::interval)")
				Expect(err).ToNot(HaveOccurred())
			})

			It("removes the record", func() {
				var count int
				err := dbConn.QueryRow("SELECT count(*) from checks").Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(0))
				Expect(removedChecks).To(Equal(1))
			})
		})

		Context("doesn't remove check is not finished", func() {

			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO checks(schema, status, create_time) VALUES('some-schema', 'started', NOW() - '25 hours'::interval)")
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not remove the record", func() {
				var count int
				err := dbConn.QueryRow("SELECT count(*) from checks").Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(1))
				Expect(removedChecks).To(Equal(0))
			})
		})

		Context("keeps checks for 24 hours", func() {

			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO checks(schema, status, create_time) VALUES('some-schema', 'succeeded', NOW() - '25 hours'::interval)")
				Expect(err).ToNot(HaveOccurred())

				_, err = dbConn.Exec("INSERT INTO checks(schema, status, create_time) VALUES('some-schema', 'succeeded', NOW() - '23 hours'::interval)")
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not remove the record", func() {
				var count int
				err := dbConn.QueryRow("SELECT count(*) from checks").Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(1))
				Expect(removedChecks).To(Equal(1))
			})
		})
	})
})
