package migration_test

import (
	"database/sql"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrate config pinned resources", func() {
	const preMigrationVersion = 1588860260
	const postMigrationVersion = 1589991895

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates all the resources that are pinned through it's resource config into the resource pins table", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			_, err := db.Exec(`
			INSERT INTO teams(id, name) VALUES
			(1, 'some-team')
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
			INSERT INTO pipelines(id, team_id, name) VALUES
			(1, 1, 'pipeline1'),
			(2, 1, 'pipeline2')
			`)
			Expect(err).NotTo(HaveOccurred())

			var resource1ID, resource2ID, resource3ID, resource4ID int
			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-1', 1, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource1ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-2', 1, '{"type": "some-type", "version": {"ref": "v1"}}', true, 'some-type') RETURNING id`).Scan(&resource2ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-3', 2, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource3ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-1', 2, '{"type": "some-type", "version": {"ref": "v2"}}', true, 'some-type') RETURNING id`).Scan(&resource4ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO resource_pins(resource_id, version, comment_text) VALUES($1, '{"ref": "v0"}', 'api-pinned')`, resource1ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO resource_pins(resource_id, version, comment_text) VALUES($1, '{"ref": "api"}', 'api-pinned')`, resource2ID)
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			type pinnedResource struct {
				resourceID int
				version    map[string]string
				config     bool
				comment    string
			}

			rows, err := db.Query(`SELECT resource_id, version, config, comment_text FROM resource_pins`)
			Expect(err).NotTo(HaveOccurred())

			var pinnedResources []pinnedResource
			for rows.Next() {
				var p pinnedResource
				var version []byte
				var pinComment sql.NullString

				err := rows.Scan(&p.resourceID, &version, &p.config, &pinComment)
				Expect(err).NotTo(HaveOccurred())

				err = json.Unmarshal(version, &p.version)
				Expect(err).NotTo(HaveOccurred())

				if pinComment.Valid {
					p.comment = pinComment.String
				}

				pinnedResources = append(pinnedResources, p)
			}

			expectedPinnedResources := []pinnedResource{
				{
					resourceID: resource1ID,
					version:    map[string]string{"ref": "v0"},
					config:     false,
					comment:    "api-pinned",
				},
				{
					resourceID: resource2ID,
					version:    map[string]string{"ref": "v1"},
					config:     true,
				},
				{
					resourceID: resource4ID,
					version:    map[string]string{"ref": "v2"},
					config:     true,
				},
			}

			_ = db.Close()

			Expect(pinnedResources).To(HaveLen(3))
			Expect(pinnedResources).To(ConsistOf(expectedPinnedResources))
		})
	})

	Context("Down", func() {
		It("removes the config pinned resources", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			_, err := db.Exec(`
			INSERT INTO teams(id, name) VALUES
			(1, 'some-team')
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
			INSERT INTO pipelines(id, team_id, name) VALUES
			(1, 1, 'pipeline1'),
			(2, 1, 'pipeline2')
			`)
			Expect(err).NotTo(HaveOccurred())

			var resource1ID, resource2ID, resource3ID, resource4ID int
			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-1', 1, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource1ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-2', 1, '{"type": "some-type", "version": {"ref": "v1"}}', true, 'some-type') RETURNING id`).Scan(&resource2ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-3', 2, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource3ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-1', 2, '{"type": "some-type", "version": {"ref": "v2"}}', true, 'some-type') RETURNING id`).Scan(&resource4ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO resource_pins(resource_id, version, config, comment_text) VALUES($1, '{"ref": "v0"}', false, '')`, resource1ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO resource_pins(resource_id, version, config, comment_text) VALUES($1, '{"ref": "v1"}', true, '')`, resource2ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO resource_pins(resource_id, version, config, comment_text) VALUES($1, '{"ref": "v2"}', true, '')`, resource4ID)
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			rows, err := db.Query(`SELECT resource_id, version FROM resource_pins`)
			Expect(err).NotTo(HaveOccurred())

			type pinnedResource struct {
				resourceID int
				version    map[string]string
			}

			var pinnedResources []pinnedResource
			for rows.Next() {
				var p pinnedResource
				var version []byte

				err := rows.Scan(&p.resourceID, &version)
				Expect(err).NotTo(HaveOccurred())

				err = json.Unmarshal(version, &p.version)
				Expect(err).NotTo(HaveOccurred())

				pinnedResources = append(pinnedResources, p)
			}

			expectedPinnedResources := []pinnedResource{
				{
					resourceID: resource1ID,
					version:    map[string]string{"ref": "v0"},
				},
			}

			_ = db.Close()

			Expect(pinnedResources).To(HaveLen(1))
			Expect(pinnedResources).To(ConsistOf(expectedPinnedResources))
		})
	})
})
