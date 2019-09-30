package db_test

import (
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SuccessfulBuildOutputsMigrator", func() {
	Describe("Migrate", func() {
		var (
			migrator   *db.SuccessfulBuildOutputsMigrator
			migrateErr error
		)

		BeforeEach(func() {
			postgresRunner.MigrateToVersion(1567800389)

			migrator = db.NewSuccessfulBuildOutputsMigrator(dbConn, lockFactory, 2)
		})

		JustBeforeEach(func() {
			postgresRunner.MigrateToVersion(1568753120)
			migrateErr = migrator.Migrate(logger)
		})

		Context("when there are no builds to migrate", func() {
			It("does not migrate any data", func() {
				Expect(migrateErr).ToNot(HaveOccurred())

				var count int
				err := psql.Select("COUNT(*)").
					From("successful_build_outputs").
					RunWith(dbConn).
					QueryRow().
					Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(0))
			})

			It("removes the migrator table", func() {
				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS ( SELECT 1 FROM information_schema.tables WHERE table_name=$1)", "successful_build_outputs_migrator").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
			})
		})

		Context("when there are builds to migrate", func() {
			var build1, build2, build3 db.Build
			var resource, resource2 db.Resource

			BeforeEach(func() {
				pipeline, _, err := defaultTeam.SavePipeline("pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
						{
							Name: "some-other-job",
						},
					},
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-base-resource-type",
							Source: atc.Source{
								"some": "source",
							},
						},
						{
							Name: "some-other-resource",
							Type: "some-base-resource-type",
							Source: atc.Source{
								"some": "other-source",
							},
						},
					},
				}, db.ConfigVersion(0), false)
				Expect(err).NotTo(HaveOccurred())

				job, found, err := pipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				job2, found, err := pipeline.Job("some-other-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build1, err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				build2, err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				build3, err = job2.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resource2, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				rcs1, err := resource.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				rcs2, err := resource2.SetResourceConfig(atc.Source{"some": "other-source"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = rcs1.SaveVersions([]atc.Version{
					{"ver": "1"},
					{"ver": "2"},
				})
				Expect(err).ToNot(HaveOccurred())

				err = rcs2.SaveVersions([]atc.Version{
					{"ver2": "1"},
				})
				Expect(err).ToNot(HaveOccurred())

				inputVersions := db.InputMapping{
					"some-input-1": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
					"some-input-2": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "2"})),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
					"some-input-3": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver2": "1"})),
								ResourceID: resource2.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
				}

				err = job.SaveNextInputMapping(inputVersions, true)
				Expect(err).ToNot(HaveOccurred())

				_, resolved, err := build1.AdoptInputsAndPipes()
				Expect(err).ToNot(HaveOccurred())
				Expect(resolved).To(BeTrue())

				err = build1.SaveOutput("some-base-resource-type", atc.Source{"some": "source"}, atc.VersionedResourceTypes{}, atc.Version{"ver": "3"}, nil, "some-output", "some-resource")
				Expect(err).ToNot(HaveOccurred())

				_, err = psql.Update("builds").
					Set("status", db.BuildStatusSucceeded).
					Where(sq.Eq{
						"id": build1.ID(),
					}).
					RunWith(dbConn).
					Exec()
				Expect(err).ToNot(HaveOccurred())

				inputVersions = db.InputMapping{
					"some-input-1": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
					"some-input-2": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "2"})),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
				}

				err = job.SaveNextInputMapping(inputVersions, true)
				Expect(err).ToNot(HaveOccurred())

				_, resolved, err = build2.AdoptInputsAndPipes()
				Expect(err).ToNot(HaveOccurred())
				Expect(resolved).To(BeTrue())

				_, err = psql.Update("builds").
					Set("status", db.BuildStatusFailed).
					Where(sq.Eq{
						"id": build2.ID(),
					}).
					RunWith(dbConn).
					Exec()
				Expect(err).ToNot(HaveOccurred())

				inputVersions = db.InputMapping{
					"some-input-1": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
					"some-input-2": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version{"ver": "1"})),
								ResourceID: resource.ID(),
							},
							FirstOccurrence: false,
						},
						PassedBuildIDs: []int{},
					},
				}

				err = job2.SaveNextInputMapping(inputVersions, true)
				Expect(err).ToNot(HaveOccurred())

				_, resolved, err = build3.AdoptInputsAndPipes()
				Expect(err).ToNot(HaveOccurred())
				Expect(resolved).To(BeTrue())

				_, err = psql.Update("builds").
					Set("status", db.BuildStatusSucceeded).
					Where(sq.Eq{
						"id": build3.ID(),
					}).
					RunWith(dbConn).
					Exec()
				Expect(err).ToNot(HaveOccurred())
			})

			It("should migrate all the successful build inputs and outputs to the new outputs table", func() {
				Expect(migrateErr).ToNot(HaveOccurred())

				rows, err := psql.Select("build_id", "job_id", "outputs").
					From("successful_build_outputs").
					RunWith(dbConn).
					Query()
				Expect(err).ToNot(HaveOccurred())

				actualOutputs := map[int]map[int][]string{}
				actualJobBuilds := map[int]int{}

				for rows.Next() {
					var buildID, jobID int
					var outputs string
					err = rows.Scan(&buildID, &jobID, &outputs)
					Expect(err).ToNot(HaveOccurred())

					actualJobBuilds[buildID] = jobID

					outputsMap := map[int][]string{}
					err = json.Unmarshal([]byte(outputs), &outputsMap)
					Expect(err).ToNot(HaveOccurred())

					actualOutputs[buildID] = outputsMap
				}

				Expect(actualJobBuilds).To(Equal(map[int]int{
					build1.ID(): build1.JobID(),
					build3.ID(): build3.JobID(),
				}))

				Expect(actualOutputs).To(HaveLen(2))
				Expect(actualOutputs[build1.ID()]).To(HaveLen(2))
				Expect(actualOutputs[build1.ID()][resource.ID()]).To(ConsistOf(
					convertToMD5(atc.Version{"ver": "3"}),
					convertToMD5(atc.Version{"ver": "1"}),
					convertToMD5(atc.Version{"ver": "2"}),
				))
				Expect(actualOutputs[build1.ID()][resource2.ID()]).To(ConsistOf(
					convertToMD5(atc.Version{"ver2": "1"}),
				))
				Expect(actualOutputs[build3.ID()]).To(HaveLen(1))
				Expect(actualOutputs[build3.ID()][resource.ID()]).To(ConsistOf(
					convertToMD5(atc.Version{"ver": "1"}),
					convertToMD5(atc.Version{"ver": "1"}),
				))
			})

			It("removes the migrator table", func() {
				var exists bool
				err := dbConn.QueryRow("SELECT EXISTS ( SELECT 1 FROM information_schema.tables WHERE table_name=$1)", "successful_build_outputs_migrator").Scan(&exists)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeFalse())
			})
		})
	})
})
