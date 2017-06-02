package migration_test

import (
	"database/sql"

	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/db/migrations"

	"encoding/json"

	"crypto/sha256"
	"fmt"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AddMetadataToResourceCache", func() {
	var dbConn *sql.DB

	var migrator migration.Migrator

	BeforeEach(func() {
		migrator = migrations.AddMetadataToResourceCache
	})

	Context("when there no existing resources", func() {
		BeforeEach(func() {
			var err error
			dbConn, err = openDBConnPreMigration(migrator)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := dbConn.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are volumes for resource cache", func() {
			var (
				resourceTypeResourceCacheID int
				resourceResourceCacheID     int
				baseResourceResourceCacheID int
				recursiveResourceCacheID    int
			)

			BeforeEach(func() {
				/*
					---
					resource_types:
					- name: some-resource-type
					  type: docker
					  source:
					    some: source
					- name: some-recursive-resource-type
					  type: some-resource-type
					  source:
					    some: source

					resources:
					- name: some-resource
					  type: some-resource-type

					- name: some-base-resource
					  type: docker

					- name: some-recursive-resource
					  type: some-recursive-resource-type

				*/

				// pipeline info
				var teamID int
				err := dbConn.QueryRow(`
					INSERT INTO teams (name) VALUES ($1) RETURNING id
				`, "some-team").Scan(&teamID)
				Expect(err).NotTo(HaveOccurred())

				var pipelineID int
				err = dbConn.QueryRow(`
					INSERT INTO pipelines (name, team_id, config) VALUES ($1, $2, $3) RETURNING id
				`, "some-pipeline", teamID, "{}").Scan(&pipelineID)
				Expect(err).NotTo(HaveOccurred())

				resourceTypeConfig := atc.ResourceConfig{
					Name: "some-resource-type",
					Type: "docker",
					Source: atc.Source{
						"some": "source",
					},
				}
				configJSON, err := json.Marshal(resourceTypeConfig)
				Expect(err).NotTo(HaveOccurred())
				sourceJSON, err := json.Marshal(resourceTypeConfig.Source)
				Expect(err).NotTo(HaveOccurred())
				resourceTypeSourceHash := fmt.Sprintf("%x", sha256.Sum256(sourceJSON))

				var resourceTypeID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_types (name, version, type, pipeline_id, config) VALUES ($1, $2, $3, $4, $5) RETURNING id
				`, resourceTypeConfig.Name, "some-resource-type-version", resourceTypeConfig.Type, pipelineID, configJSON).Scan(&resourceTypeID)
				Expect(err).NotTo(HaveOccurred())

				recursiveResourceTypeConfig := atc.ResourceConfig{
					Name: "some-recursive-resource-type",
					Type: "some-resource-type",
					Source: atc.Source{
						"some": "source",
					},
				}
				recursiveConfigJSON, err := json.Marshal(recursiveResourceTypeConfig)
				Expect(err).NotTo(HaveOccurred())
				recursiveSourceJSON, err := json.Marshal(recursiveResourceTypeConfig.Source)
				Expect(err).NotTo(HaveOccurred())
				recursiveResourceTypeSourceHash := fmt.Sprintf("%x", sha256.Sum256(recursiveSourceJSON))

				var recursiveResourceTypeID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_types (name, version, type, pipeline_id, config) VALUES ($1, $2, $3, $4, $5) RETURNING id
				`, recursiveResourceTypeConfig.Name, "some-recursive-resource-type-version", recursiveResourceTypeConfig.Type, pipelineID, recursiveConfigJSON).Scan(&recursiveResourceTypeID)
				Expect(err).NotTo(HaveOccurred())

				// some-base-resource
				var resourceID int
				err = dbConn.QueryRow(`
					INSERT INTO resources (name, pipeline_id, config, source_hash) VALUES ($1, $2, $3, $4) RETURNING id
				`, "some-base-resource", pipelineID, `{"type":"docker"}`, resourceTypeSourceHash).Scan(&resourceID)
				Expect(err).NotTo(HaveOccurred())

				_, err = dbConn.Exec(`
					INSERT INTO versioned_resources (version, metadata, type, resource_id) VALUES ($1, $2, $3, $4) RETURNING id
				`, "some-base-resource-version", "some-base-resource-metadata", "docker", resourceID)
				Expect(err).NotTo(HaveOccurred())

				// some-resource
				err = dbConn.QueryRow(`
					INSERT INTO resources (name, pipeline_id, config, source_hash) VALUES ($1, $2, $3, $4) RETURNING id
				`, "some-resource", pipelineID, `{"type":"some-resource-type"}`, "some-resource-source-hash").Scan(&resourceID)
				Expect(err).NotTo(HaveOccurred())

				_, err = dbConn.Exec(`
					INSERT INTO versioned_resources (version, metadata, type, resource_id) VALUES ($1, $2, $3, $4) RETURNING id
				`, "some-resource-version", "some-resource-metadata", "some-resource-type", resourceID)
				Expect(err).NotTo(HaveOccurred())

				// some-recursive-resource
				err = dbConn.QueryRow(`
					INSERT INTO resources (name, pipeline_id, config, source_hash) VALUES ($1, $2, $3, $4) RETURNING id
				`, "some-recursive-resource", pipelineID, `{"type":"some-recursive-resource-type"}`, "some-recursive-resource-source-hash").Scan(&resourceID)
				Expect(err).NotTo(HaveOccurred())

				_, err = dbConn.Exec(`
					INSERT INTO versioned_resources (version, metadata, type, resource_id) VALUES ($1, $2, $3, $4) RETURNING id
				`, "some-recursive-resource-version", "some-recursive-resource-metadata", recursiveResourceTypeConfig.Name, resourceID)
				Expect(err).NotTo(HaveOccurred())

				// base resource type
				var baseResourceTypeID int
				err = dbConn.QueryRow(`
					INSERT INTO base_resource_types (name) VALUES ($1) RETURNING id
				`, "docker").Scan(&baseResourceTypeID)
				Expect(err).NotTo(HaveOccurred())

				// base-resource cache
				var baseResourceConfigID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_configs (base_resource_type_id, source_hash) VALUES ($1, $2) RETURNING id
				`, baseResourceTypeID, resourceTypeSourceHash).Scan(&baseResourceConfigID)
				Expect(err).NotTo(HaveOccurred())

				err = dbConn.QueryRow(`
					INSERT INTO resource_caches (resource_config_id, version, params_hash) VALUES ($1, $2, $3) RETURNING id
				`, baseResourceConfigID, "some-base-resource-version", "some-base-resource-params-hash").Scan(&baseResourceResourceCacheID)
				Expect(err).NotTo(HaveOccurred())

				// some-resource-type cache
				err = dbConn.QueryRow(`
					INSERT INTO resource_caches (resource_config_id, version, params_hash) VALUES ($1, $2, $3) RETURNING id
				`, baseResourceConfigID, "some-resource-type-version", "some-resource-type-params-hash").Scan(&resourceTypeResourceCacheID)
				Expect(err).NotTo(HaveOccurred())

				// some-resource cache
				var resourceConfigID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_configs (resource_cache_id, source_hash) VALUES ($1, $2) RETURNING id
				`, resourceTypeResourceCacheID, "some-resource-source-hash").Scan(&resourceConfigID)
				Expect(err).NotTo(HaveOccurred())

				err = dbConn.QueryRow(`
					INSERT INTO resource_caches (resource_config_id, version, params_hash) VALUES ($1, $2, $3) RETURNING id
				`, resourceConfigID, "some-resource-version", "some-resource-params-hash").Scan(&resourceResourceCacheID)
				Expect(err).NotTo(HaveOccurred())

				// some-recursive-resource-type cache
				var recursiveResourceTypeConfigID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_configs (resource_cache_id, source_hash) VALUES ($1, $2) RETURNING id
				`, resourceTypeResourceCacheID, recursiveResourceTypeSourceHash).Scan(&recursiveResourceTypeConfigID)
				Expect(err).NotTo(HaveOccurred())

				var recursiveResourceTypeCacheID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_caches (resource_config_id, version, params_hash) VALUES ($1, $2, $3) RETURNING id
				`, recursiveResourceTypeConfigID, "some-recursive-resource-type-version", "some-recursive-resource-type-params-hash").Scan(&recursiveResourceTypeCacheID)
				Expect(err).NotTo(HaveOccurred())

				// some-recursive-resource cache
				var recursiveResourceConfigID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_configs (resource_cache_id, source_hash) VALUES ($1, $2) RETURNING id
				`, recursiveResourceTypeCacheID, "some-recursive-resource-source-hash").Scan(&recursiveResourceConfigID)
				Expect(err).NotTo(HaveOccurred())

				err = dbConn.QueryRow(`
					INSERT INTO resource_caches (resource_config_id, version, params_hash) VALUES ($1, $2, $3) RETURNING id
				`, recursiveResourceConfigID, "some-recursive-resource-version", "some-recursive-resource-params-hash").Scan(&recursiveResourceCacheID)
				Expect(err).NotTo(HaveOccurred())

				err = dbConn.Close()
				Expect(err).NotTo(HaveOccurred())

				dbConn, err = openDBConnPostMigration(migrator)
				Expect(err).NotTo(HaveOccurred())
			})

			It("migrates metadata to resource cache for base resources", func() {
				var migratedMetadata []byte
				err := dbConn.QueryRow(`
					SELECT metadata FROM resource_caches WHERE id=$1
				`, baseResourceResourceCacheID).Scan(&migratedMetadata)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(migratedMetadata)).To(Equal("some-base-resource-metadata"))
			})

			It("migrates metadata to resource cache for custom resources", func() {
				var migratedMetadata []byte
				err := dbConn.QueryRow(`
					SELECT metadata FROM resource_caches WHERE id=$1
				`, resourceTypeResourceCacheID).Scan(&migratedMetadata)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(migratedMetadata)).To(Equal(""))

				err = dbConn.QueryRow(`
					SELECT metadata FROM resource_caches WHERE id=$1
				`, resourceResourceCacheID).Scan(&migratedMetadata)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(migratedMetadata)).To(Equal("some-resource-metadata"))
			})

			It("migrates metadata to resource cache for custom resource that depends on other resource", func() {
				var migratedMetadata []byte
				err := dbConn.QueryRow(`
					SELECT metadata FROM resource_caches WHERE id=$1
				`, recursiveResourceCacheID).Scan(&migratedMetadata)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(migratedMetadata)).To(Equal("some-recursive-resource-metadata"))
			})
		})
	})
})
