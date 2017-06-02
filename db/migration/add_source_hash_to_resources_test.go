package migration_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/db/migrations"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AddSourceHashToResources", func() {
	var dbConn *sql.DB
	var err error

	createResourcesTableMigration := func(tx migration.LimitedTx) error {
		_, err := tx.Exec(`
      CREATE TABLE resources (
        id serial PRIMARY KEY,
        config text
      )
    `)
		return err
	}

	Context("when there no existing resources", func() {
		BeforeEach(func() {
			dbConn, err = migration.Open(
				"postgres",
				postgresRunner.DataSourceName(),
				[]migration.Migrator{
					createResourcesTableMigration,
					migrations.AddSourceHashToResources,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := dbConn.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds source_hash to resources that can't  be null", func() {
			_, err := dbConn.Exec(`
			INSERT INTO resources (
				source_hash
			)
			VALUES (NULL)
		`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`null value in column "source_hash" violates not-null constraint`))

			_, err = dbConn.Exec(`
      INSERT INTO resources (
        source_hash
      )
      VALUES ('some-value')
    `)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when there are existing resources", func() {
		var config atc.ResourceConfig

		BeforeEach(func() {
			dbConn, err = migration.Open(
				"postgres",
				postgresRunner.DataSourceName(),
				[]migration.Migrator{
					createResourcesTableMigration,
				},
			)
			Expect(err).NotTo(HaveOccurred())

			config = atc.ResourceConfig{
				Source: atc.Source{
					"some": "source",
				},
			}
			marshalledConfig, err := json.Marshal(config)
			Expect(err).NotTo(HaveOccurred())

			_, err = dbConn.Exec(`
  			INSERT INTO resources (
  				config
  			)
  			VALUES ('` + string(marshalledConfig) + `')
  		`)
			Expect(err).NotTo(HaveOccurred())

			Expect(dbConn.Close()).To(Succeed())

			dbConn, err = migration.Open(
				"postgres",
				postgresRunner.DataSourceName(),
				[]migration.Migrator{
					createResourcesTableMigration,
					migrations.AddSourceHashToResources,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(dbConn.Close()).To(Succeed())
		})

		It("calculates source hash for them", func() {
			var sourceHash string
			err := dbConn.QueryRow(`
				SELECT source_hash FROM resources LIMIT 1
			`).Scan(&sourceHash)
			Expect(err).NotTo(HaveOccurred())

			marshalledSourceConfig, err := json.Marshal(config.Source)
			Expect(err).NotTo(HaveOccurred())

			Expect(sourceHash).To(Equal(fmt.Sprintf("%x", sha256.Sum256(marshalledSourceConfig))))
		})
	})
})
