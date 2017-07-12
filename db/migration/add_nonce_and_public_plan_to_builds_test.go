package migration_test

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/db/migrations"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("AddNonceAndPublicPlanToBuilds", func() {
	var (
		dbConn   *sql.DB
		migrator migration.Migrator

		buildID      int
		otherBuildID int
	)

	BeforeEach(func() {
		migrator = migrations.AddNonceAndPublicPlanToBuilds
	})

	Context("when there no existing resources", func() {
		var engineMetadataJSON []byte

		BeforeEach(func() {
			var err error
			dbConn, err = openDBConnPreMigration(migrator)
			Expect(err).NotTo(HaveOccurred())

			// pipeline build
			var teamID int
			err = dbConn.QueryRow(`
				INSERT INTO teams (name) VALUES ($1) RETURNING id
			`, "some-team").Scan(&teamID)
			Expect(err).NotTo(HaveOccurred())

			planID := atc.PlanID("42")
			engineMetadata := execV2Metadata{
				Plan: atc.Plan{
					ID: atc.PlanID("56"),
					Get: &atc.GetPlan{
						Type:        "some-type",
						Name:        "some-name",
						Resource:    "some-resource",
						Source:      atc.Source{"some": "source"},
						Params:      atc.Params{"some": "params"},
						Version:     &atc.Version{"some": "version"},
						VersionFrom: &planID,
						Tags:        atc.Tags{"some-tags"},
						VersionedResourceTypes: atc.VersionedResourceTypes{
							{
								ResourceType: atc.ResourceType{
									Name:       "some-name",
									Source:     atc.Source{"some": "source"},
									Type:       "some-type",
									Privileged: true,
									Tags:       atc.Tags{"some-tags"},
								},
								Version: atc.Version{"some-resource-type": "version"},
							},
						},
					},
				},
			}
			engineMetadataJSON, err = json.Marshal(engineMetadata)
			Expect(err).NotTo(HaveOccurred())

			//put 1000 entries in the table to make sure batching works
			for i := 0; i < 1000; i++ {
				err = dbConn.
					QueryRow(
						`INSERT INTO builds (name, status, team_id, engine, engine_metadata) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
						fmt.Sprintf("%d", i),
						db.BuildStatusSucceeded,
						teamID,
						"exec.v2",
						engineMetadataJSON,
					).Scan(&buildID)
				Expect(err).NotTo(HaveOccurred())
			}

			statuses := []db.BuildStatus{
				db.BuildStatusStarted,
				db.BuildStatusSucceeded,
				db.BuildStatusAborted,
				db.BuildStatusFailed,
				db.BuildStatusErrored,
				db.BuildStatusPending,
			}
			for i, status := range statuses {
				err = dbConn.
					QueryRow(
						`INSERT INTO builds (name, status, team_id, engine, engine_metadata) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
						fmt.Sprintf("%d", i),
						status,
						teamID,
						"exec.v2",
						engineMetadataJSON,
					).Scan(&buildID)
				Expect(err).NotTo(HaveOccurred())
			}

			err = dbConn.
				QueryRow(
					`INSERT INTO builds (name, status, team_id, engine, engine_metadata) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
					"1",
					db.BuildStatusStarted,
					teamID,
					"exec.v3",
					engineMetadataJSON,
				).Scan(&otherBuildID)
			Expect(err).NotTo(HaveOccurred())

			err = dbConn.Close()
			Expect(err).NotTo(HaveOccurred())

			dbConn, err = openDBConnPostMigration(migrator)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := dbConn.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds a nonce field", func() {
			result, err := dbConn.Exec(
				`UPDATE builds SET nonce='some-nonce' WHERE id=$1`, buildID)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RowsAffected()).To(BeNumerically("==", 1))
		})

		Context("when build was created with exec.v2 engine", func() {
			It("creates a public plan from existing engine metadata", func() {
				var publicPlanJSON []byte
				err := dbConn.QueryRow(
					`SELECT public_plan FROM builds WHERE id=$1`,
					buildID,
				).Scan(&publicPlanJSON)
				Expect(err).NotTo(HaveOccurred())

				var publicPlan atc.Plan
				err = json.Unmarshal(publicPlanJSON, &publicPlan)
				Expect(err).NotTo(HaveOccurred())

				Expect(publicPlan).To(Equal(atc.Plan{
					ID: atc.PlanID("56"),
					Get: &atc.GetPlan{
						Type:     "some-type",
						Name:     "some-name",
						Resource: "some-resource",
						Version:  &atc.Version{"some": "version"},
					},
				}))
			})

			DescribeTable("keeps engine metadata",
				func(status db.BuildStatus) {
					var returnedEngineMetadataJSON []byte
					err := dbConn.QueryRow(
						`SELECT engine_metadata FROM builds WHERE status=$1 AND engine='exec.v2' LIMIT 1`,
						status,
					).Scan(&returnedEngineMetadataJSON)
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedEngineMetadataJSON).To(MatchJSON(engineMetadataJSON))
				},
				Entry("for pending builds", db.BuildStatusPending),
				Entry("for started builds", db.BuildStatusStarted),
			)

			DescribeTable("nullifies engine metadata",
				func(status db.BuildStatus) {
					var returnedEngineMetadataJSON []byte
					err := dbConn.QueryRow(
						`SELECT engine_metadata FROM builds WHERE status=$1 AND engine='exec.v2' LIMIT 1`,
						status,
					).Scan(&returnedEngineMetadataJSON)
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedEngineMetadataJSON).To(BeNil())
				},
				Entry("for succeeded builds", db.BuildStatusSucceeded),
				Entry("for aborted builds", db.BuildStatusAborted),
				Entry("for errored builds", db.BuildStatusErrored),
				Entry("for failed builds", db.BuildStatusFailed),
			)
		})

		Context("when build was created with other engine", func() {
			It("does not create public plan from engine metadata", func() {
				var publicPlanJSON []byte
				err := dbConn.QueryRow(
					`SELECT public_plan FROM builds WHERE id=$1`,
					otherBuildID,
				).Scan(&publicPlanJSON)
				Expect(err).NotTo(HaveOccurred())

				Expect(publicPlanJSON).To(Equal([]byte("{}")))
			})
		})

		It("Creates a public plan for all builds", func() {
			var count int
			err := dbConn.QueryRow(`
				SELECT COUNT(*)
				FROM builds
				WHERE public_plan IS NOT NULL
				AND status='succeeded'
				AND engine='exec.v2'
				`).
				Scan(&count)
			Expect(err).NotTo(HaveOccurred())

			Expect(count).To(BeNumerically(">", 1000))
		})
	})
})

type execV2Metadata struct {
	Plan atc.Plan
}
