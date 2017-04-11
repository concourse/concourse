package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/lock"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("next build inputs for job", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB
	var pipelineDB db.PipelineDB
	var pipelineDB2 db.PipelineDB
	var versions db.SavedVersionedResources

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		lockFactory := lock.NewLockFactory(postgresRunner.OpenSingleton())
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)

		_, err := sqlDB.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		resourceConfig := atc.ResourceConfig{
			Name: "some-resource",
			Type: "some-type",
		}

		config := atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
				{
					Name: "some-other-job",
				},
			},
			Resources: atc.ResourceConfigs{resourceConfig},
		}

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB := teamDBFactory.GetTeamDB("some-team")
		savedPipeline, _, err := teamDB.SaveConfigToBeDeprecated("some-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)

		err = pipelineDB.SaveResourceVersions(
			resourceConfig,
			[]atc.Version{
				{"version": "v1"},
				{"version": "v2"},
				{"version": "v3"},
			},
		)
		Expect(err).NotTo(HaveOccurred())

		// save metadata for v1
		build, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).ToNot(HaveOccurred())
		_, err = build.SaveInput(db.BuildInput{
			Name: "some-input",
			VersionedResource: db.VersionedResource{
				Resource:   "some-resource",
				Type:       "some-type",
				Version:    db.Version{"version": "v1"},
				Metadata:   []db.MetadataField{{Name: "name1", Value: "value1"}},
				PipelineID: pipelineDB.GetPipelineID(),
			},
			FirstOccurrence: true,
		})
		Expect(err).NotTo(HaveOccurred())

		reversions, _, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 3})
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		versions = []db.SavedVersionedResource{reversions[2], reversions[1], reversions[0]}

		savedPipeline2, _, err := teamDB.SaveConfigToBeDeprecated("some-pipeline-2", config, 1, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB2 = pipelineDBFactory.Build(savedPipeline2)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("independent build inputs", func() {
		It("gets independent build inputs for the given job name", func() {
			inputVersions := algorithm.InputMapping{
				"some-input-1": algorithm.InputVersion{
					VersionID:       versions[0].ID,
					FirstOccurrence: false,
				},
				"some-input-2": algorithm.InputVersion{
					VersionID:       versions[1].ID,
					FirstOccurrence: true,
				},
			}
			err := pipelineDB.SaveIndependentInputMapping(inputVersions, "some-job")
			Expect(err).NotTo(HaveOccurred())

			pipeline2InputVersions := algorithm.InputMapping{
				"some-input-3": algorithm.InputVersion{
					VersionID:       versions[2].ID,
					FirstOccurrence: false,
				},
			}
			err = pipelineDB2.SaveIndependentInputMapping(pipeline2InputVersions, "some-job")
			Expect(err).NotTo(HaveOccurred())

			buildInputs := []db.BuildInput{
				{
					Name:              "some-input-1",
					VersionedResource: versions[0].VersionedResource,
					FirstOccurrence:   false,
				},
				{
					Name:              "some-input-2",
					VersionedResource: versions[1].VersionedResource,
					FirstOccurrence:   true,
				},
			}

			actualBuildInputs, err := pipelineDB.GetIndependentBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(actualBuildInputs).To(ConsistOf(buildInputs))

			By("updating the set of independent build inputs")
			inputVersions2 := algorithm.InputMapping{
				"some-input-2": algorithm.InputVersion{
					VersionID:       versions[2].ID,
					FirstOccurrence: false,
				},
				"some-input-3": algorithm.InputVersion{
					VersionID:       versions[2].ID,
					FirstOccurrence: true,
				},
			}
			err = pipelineDB.SaveIndependentInputMapping(inputVersions2, "some-job")
			Expect(err).NotTo(HaveOccurred())

			buildInputs2 := []db.BuildInput{
				{
					Name:              "some-input-2",
					VersionedResource: versions[2].VersionedResource,
					FirstOccurrence:   false,
				},
				{
					Name:              "some-input-3",
					VersionedResource: versions[2].VersionedResource,
					FirstOccurrence:   true,
				},
			}

			actualBuildInputs2, err := pipelineDB.GetIndependentBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

			By("updating independent build inputs to an empty set when the mapping is nil")
			err = pipelineDB.SaveIndependentInputMapping(nil, "some-job")
			Expect(err).NotTo(HaveOccurred())

			actualBuildInputs3, err := pipelineDB.GetIndependentBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualBuildInputs3).To(BeEmpty())
		})
	})

	Describe("next build inputs", func() {
		It("gets next build inputs for the given job name", func() {
			inputVersions := algorithm.InputMapping{
				"some-input-1": algorithm.InputVersion{
					VersionID:       versions[0].ID,
					FirstOccurrence: false,
				},
				"some-input-2": algorithm.InputVersion{
					VersionID:       versions[1].ID,
					FirstOccurrence: true,
				},
			}
			err := pipelineDB.SaveNextInputMapping(inputVersions, "some-job")
			Expect(err).NotTo(HaveOccurred())

			pipeline2InputVersions := algorithm.InputMapping{
				"some-input-3": algorithm.InputVersion{
					VersionID:       versions[2].ID,
					FirstOccurrence: false,
				},
			}
			err = pipelineDB2.SaveNextInputMapping(pipeline2InputVersions, "some-job")
			Expect(err).NotTo(HaveOccurred())

			buildInputs := []db.BuildInput{
				{
					Name:              "some-input-1",
					VersionedResource: versions[0].VersionedResource,
					FirstOccurrence:   false,
				},
				{
					Name:              "some-input-2",
					VersionedResource: versions[1].VersionedResource,
					FirstOccurrence:   true,
				},
			}

			actualBuildInputs, found, err := pipelineDB.GetNextBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(actualBuildInputs).To(ConsistOf(buildInputs))

			By("updating the set of next build inputs")
			inputVersions2 := algorithm.InputMapping{
				"some-input-2": algorithm.InputVersion{
					VersionID:       versions[2].ID,
					FirstOccurrence: false,
				},
				"some-input-3": algorithm.InputVersion{
					VersionID:       versions[2].ID,
					FirstOccurrence: true,
				},
			}
			err = pipelineDB.SaveNextInputMapping(inputVersions2, "some-job")
			Expect(err).NotTo(HaveOccurred())

			buildInputs2 := []db.BuildInput{
				{
					Name:              "some-input-2",
					VersionedResource: versions[2].VersionedResource,
					FirstOccurrence:   false,
				},
				{
					Name:              "some-input-3",
					VersionedResource: versions[2].VersionedResource,
					FirstOccurrence:   true,
				},
			}

			actualBuildInputs2, found, err := pipelineDB.GetNextBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(actualBuildInputs2).To(ConsistOf(buildInputs2))

			By("updating next build inputs to an empty set when the mapping is nil")
			err = pipelineDB.SaveNextInputMapping(nil, "some-job")
			Expect(err).NotTo(HaveOccurred())

			actualBuildInputs3, found, err := pipelineDB.GetNextBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualBuildInputs3).To(BeEmpty())
		})

		It("distinguishes between a job with no inputs and a job with missing inputs", func() {
			By("initially returning not found")
			_, found, err := pipelineDB.GetNextBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("returning found when an empty input mapping is saved")
			err = pipelineDB.SaveNextInputMapping(algorithm.InputMapping{}, "some-job")
			Expect(err).NotTo(HaveOccurred())

			_, found, err = pipelineDB.GetNextBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			By("returning not found when the input mapping is deleted")
			err = pipelineDB.DeleteNextInputMapping("some-job")
			Expect(err).NotTo(HaveOccurred())

			_, found, err = pipelineDB.GetNextBuildInputs("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})
})
