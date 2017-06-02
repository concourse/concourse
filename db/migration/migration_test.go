package migration_test

import (
	"database/sql"

	"github.com/concourse/atc/db/migration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migration", func() {
	var dbConn *sql.DB
	var err error

	createTableMigration := func(tx migration.LimitedTx) error {
		_, err := tx.Exec(`
			CREATE TABLE test (
				field1 text
			)
		`)
		return err
	}
	createFieldMigration := func(tx migration.LimitedTx) error {
		_, err := tx.Exec(`
			ALTER TABLE test
			ADD COLUMN field2 text
		`)
		return err
	}

	BeforeEach(func() {
		dbConn, err = migration.Open(
			"postgres",
			postgresRunner.DataSourceName(),
			[]migration.Migrator{
				createTableMigration,
				createFieldMigration,
			},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("runs migrations", func() {
		_, err := dbConn.Exec(`
			INSERT INTO test (
				field1, field2
			)
			VALUES ('value-1', 'value-2')
		`)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("with new migrations", func() {
		var newDbConn *sql.DB

		BeforeEach(func() {
			newDbConn, err = migration.Open(
				"postgres",
				postgresRunner.DataSourceName(),
				[]migration.Migrator{
					createTableMigration,
					createFieldMigration,
					func(tx migration.LimitedTx) error {
						_, err := tx.Exec(`
							ALTER TABLE test
							ADD COLUMN field3 text
						`)
						return err
					},
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := newDbConn.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		It("runs new migrations after existing migrations", func() {
			_, err := newDbConn.Exec(`
				INSERT INTO test (
					field1, field2, field3
				)
				VALUES ('value-1', 'value-2', 'value-3')
			`)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
