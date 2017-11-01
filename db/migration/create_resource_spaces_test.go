package migration_test

import (
	"database/sql"

	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/db/migrations"
	_ "github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CreateResourceSpaces", func() {
	var dbConn *sql.DB
	var migrator migration.Migrator

	// explicit type here is important for reflect.ValueOf
	migrator = migrations.CreateResourceSpaces

	BeforeEach(func() {
		var err error
		dbConn, err = openDBConnPreMigration(migrator)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("before resource spaces are introduced", func() {
		var teamID, pipelineID, resourceID, versionedResourceID int

		BeforeEach(func() {
			err := dbConn.QueryRow(`
				INSERT INTO teams(name) VALUES('some-team') RETURNING id
			`).Scan(&teamID)
			Expect(err).NotTo(HaveOccurred())

			err = dbConn.QueryRow(`
				INSERT INTO pipelines(name, team_id) VALUES('some-pipeline', $1) RETURNING id
			`, teamID).Scan(&pipelineID)
			Expect(err).NotTo(HaveOccurred())

			err = dbConn.QueryRow(`
				INSERT INTO resources(name, config, pipeline_id) VALUES('some-resource', $1, $2) RETURNING id
			`, "{}", pipelineID).Scan(&resourceID)
			Expect(err).NotTo(HaveOccurred())

			err = dbConn.QueryRow(`
				INSERT INTO versioned_resources(resource_id, version, metadata, type) VALUES($1, '{}', '[]', 'some-resource') RETURNING id
			`, resourceID).Scan(&versionedResourceID)
			Expect(err).NotTo(HaveOccurred())

			err = dbConn.Close()
			Expect(err).NotTo(HaveOccurred())

			dbConn, err = openDBConnPostMigration(migrator)
			Expect(err).NotTo(HaveOccurred())
		})

		It("migrates the resources to resource spaces", func() {
			var resourceSpaceID int
			err := dbConn.QueryRow(`
				SELECT id FROM resource_spaces WHERE resource_id=$1
			`, resourceID).Scan(&resourceSpaceID)
			Expect(err).NotTo(HaveOccurred())

			Expect(resourceSpaceID).To(Equal(resourceID))
		})
	})
})
