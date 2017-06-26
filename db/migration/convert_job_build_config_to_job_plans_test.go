package migration_test

import (
	"database/sql"

	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/db/migrations"
	_ "github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConvertJobBuildConfigToJobPlans", func() {
	var dbConn *sql.DB
	var migrator migration.Migrator

	// explicit type here is important for reflect.ValueOf
	migrator = migrations.ConvertJobBuildConfigToJobPlans

	BeforeEach(func() {
		var err error
		dbConn, err = openDBConnPreMigration(migrator)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when a config is present with old-style job configurations", func() {
		var initialConfigID int

		BeforeEach(func() {
			err := dbConn.QueryRow(`
				INSERT INTO config (config, id)
				VALUES ($1, nextval('config_id_seq'))
				RETURNING id
			`, `{
				"jobs": [
					{
						"name": "some-job",
						"inputs": [
							{
								"name": "some-input-name",
								"resource": "some-resource",
								"passed": ["job-a", "job-b"],
								"trigger": false,
								"params": {	"some": "param"	}
							},
							{
								"name": "some-other-input-name",
								"resource": "some-triggering-resource",
								"passed": ["job-a", "job-b"],
								"trigger": true,
								"params": {	"some": "param" }
							},
							{
								"resource": "some-simple-resource"
							}
						],
						"build": "some-input-name/build.yml",
						"config": {
							"params": {"A": "B"}
						},
						"privileged": true,
						"outputs": [
							{
								"resource": "some-resource",
								"params": {	"some": "param" },
								"perform_on": []
							},
							{
								"resource": "some-triggering-resource",
								"perform_on": ["success", "failure"]
							},
							{
								"resource": "some-simple-resource"
							}
						]
					},
					{
						"name": "some-job-with-no-inputs",
						"build": "some-input-name/build.yml",
						"config": {
							"params": {"A": "B"}
						},
						"privileged": false,
						"outputs": [
							{
								"resource": "some-resource",
								"params": {	"some": "param" },
								"perform_on": []
							},
							{
								"resource": "some-triggering-resource",
								"perform_on": ["success", "failure"]
							},
							{
								"resource": "some-simple-resource"
							}
						]
					},
					{
						"name": "some-job-with-no-outputs",
						"inputs": [
							{
								"name": "some-input-name",
								"resource": "some-resource",
								"passed": ["job-a", "job-b"],
								"trigger": false,
								"params": {	"some": "param"	}
							},
							{
								"name": "some-other-input-name",
								"resource": "some-triggering-resource",
								"passed": ["job-a", "job-b"],
								"trigger": true,
								"params": {	"some": "param" }
							},
							{
								"resource": "some-simple-resource"
							}
						],
						"build": "some-input-name/build.yml",
						"config": {
							"params": {"A": "B"}
						}
					},
					{
						"name": "some-job-with-no-task",
						"inputs": [
							{
								"name": "some-input-name",
								"resource": "some-resource",
								"passed": ["job-a", "job-b"],
								"trigger": false,
								"params": {	"some": "param"	}
							},
							{
								"name": "some-other-input-name",
								"resource": "some-triggering-resource",
								"passed": ["job-a", "job-b"],
								"trigger": true,
								"params": {	"some": "param" }
							},
							{
								"resource": "some-simple-resource"
							}
						],
						"outputs": [
							{
								"resource": "some-resource",
								"params": {	"some": "param" },
								"perform_on": []
							},
							{
								"resource": "some-triggering-resource",
								"perform_on": ["success", "failure"]
							},
							{
								"resource": "some-simple-resource"
							}
						]
					}
				]
			}`).Scan(&initialConfigID)
			Expect(err).NotTo(HaveOccurred())

			err = dbConn.Close()
			Expect(err).NotTo(HaveOccurred())

			dbConn, err = openDBConnPostMigration(migrator)
			Expect(err).NotTo(HaveOccurred())
		})

		It("migrates them to the new plan-based configuration", func() {
			var configBlob []byte
			var id int

			err := dbConn.QueryRow(`
				SELECT config, id
				FROM config
			`).Scan(&configBlob, &id)
			Expect(err).NotTo(HaveOccurred())

			//Expect(id).To(Equal(initialConfigID + 1))
			Expect(configBlob).To(MatchJSON(`{
				"jobs": [
					{
						"name": "some-job",
						"plan": [
							{
								"aggregate": [
									{
										"get": "some-input-name",
										"resource": "some-resource",
										"passed": ["job-a", "job-b"],
										"trigger": false,
										"params": { "some": "param" }
									},
									{
										"get": "some-other-input-name",
										"resource": "some-triggering-resource",
										"passed": ["job-a", "job-b"],
										"trigger": true,
										"params": {	"some": "param"	}
									},
									{
										"get": "some-simple-resource"
									}
								]
							},
							{
								"task": "build",
								"privileged": true,
								"file": "some-input-name/build.yml",
								"config": {
									"params": {"A": "B"}
								}
							},
							{
								"aggregate": [
									{
										"put": "some-resource",
										"params": {	"some": "param" },
										"conditions": []
									},
									{
										"put": "some-triggering-resource",
										"conditions": ["success", "failure"]
									},
									{
										"put": "some-simple-resource"
									}
								]
							}
						]
					},
					{
						"name": "some-job-with-no-inputs",
						"plan": [
							{
								"task": "build",
								"file": "some-input-name/build.yml",
								"config": {
									"params": {"A": "B"}
								}
							},
							{
								"aggregate": [
									{
										"put": "some-resource",
										"params": {	"some": "param" },
										"conditions": []
									},
									{
										"put": "some-triggering-resource",
										"conditions": ["success", "failure"]
									},
									{
										"put": "some-simple-resource"
									}
								]
							}
						]
					},
					{
						"name": "some-job-with-no-outputs",
						"plan": [
							{
								"aggregate": [
									{
										"get": "some-input-name",
										"resource": "some-resource",
										"passed": ["job-a", "job-b"],
										"trigger": false,
										"params": { "some": "param" }
									},
									{
										"get": "some-other-input-name",
										"resource": "some-triggering-resource",
										"passed": ["job-a", "job-b"],
										"trigger": true,
										"params": {	"some": "param"	}
									},
									{
										"get": "some-simple-resource"
									}
								]
							},
							{
								"task": "build",
								"file": "some-input-name/build.yml",
								"config": {
									"params": {"A": "B"}
								}
							}
						]
					},
					{
						"name": "some-job-with-no-task",
						"plan": [
							{
								"aggregate": [
									{
										"get": "some-input-name",
										"resource": "some-resource",
										"passed": ["job-a", "job-b"],
										"trigger": false,
										"params": { "some": "param" }
									},
									{
										"get": "some-other-input-name",
										"resource": "some-triggering-resource",
										"passed": ["job-a", "job-b"],
										"trigger": true,
										"params": {	"some": "param"	}
									},
									{
										"get": "some-simple-resource"
									}
								]
							},
							{
								"aggregate": [
									{
										"put": "some-resource",
										"params": {	"some": "param" },
										"conditions": []
									},
									{
										"put": "some-triggering-resource",
										"conditions": ["success", "failure"]
									},
									{
										"put": "some-simple-resource"
									}
								]
							}
						]
					}
				]
			}`))

		})
	})

	Context("when no config is present", func() {
		It("succeeds", func() {})
	})
})
