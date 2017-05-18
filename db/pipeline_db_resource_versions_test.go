package db_test

import (
	"crypto/aes"
	"crypto/cipher"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource History", func() {
	var dbConn db.Conn
	var dbngConn dbng.Conn
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB
	var pipelineDB db.PipelineDB
	var savedPipeline db.SavedPipeline
	var dbngBuildFactory dbng.BuildFactory

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.OpenDB())

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		lockFactory := lock.NewLockFactory(postgresRunner.OpenSingleton())
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)
		dbngConn = postgresRunner.OpenConn()

		_, err := sqlDB.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		config := atc.Config{
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
					Type: "some-type",
				},
			},
		}

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB := teamDBFactory.GetTeamDB("some-team")
		savedPipeline, _, err = teamDB.SaveConfigToBeDeprecated("a-pipeline-name", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
		eBlock, err := aes.NewCipher([]byte("AES256Key-32Characters1234567890"))
		Expect(err).ToNot(HaveOccurred())
		aesgcm, err := cipher.NewGCM(eBlock)
		Expect(err).ToNot(HaveOccurred())

		key := dbng.NewEncryptionKey(aesgcm)

		dbngBuildFactory = dbng.NewBuildFactory(dbngConn, lockFactory, key)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = dbngConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("GetBuildsWithVersionAsInput", func() {
		var savedVersionedResourceID int
		var expectedBuilds []db.Build

		BeforeEach(func() {
			build, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			_, err = pipelineDB.CreateJobBuild("some-other-job")
			Expect(err).NotTo(HaveOccurred())

			dbngBuild, found, err := dbngBuildFactory.Build(build.ID())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbngBuild.SaveInput(dbng.BuildInput{
				Name: "some-input",
				VersionedResource: dbng.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: dbng.ResourceVersion{
						"version": "v1",
					},
					Metadata: []dbng.ResourceMetadataField{
						{
							Name:  "some",
							Value: "value",
						},
					},
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())
			versionedResources, err := dbngBuild.GetVersionedResources()
			Expect(err).NotTo(HaveOccurred())
			Expect(versionedResources).To(HaveLen(1))

			dbngSecondBuild, found, err := dbngBuildFactory.Build(secondBuild.ID())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			err = dbngSecondBuild.SaveInput(dbng.BuildInput{
				Name: "some-input",
				VersionedResource: dbng.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: dbng.ResourceVersion{
						"version": "v1",
					},
					Metadata: []dbng.ResourceMetadataField{
						{
							Name:  "some",
							Value: "value",
						},
					},
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())
			secondVersionedResources, err := dbngBuild.GetVersionedResources()
			Expect(err).NotTo(HaveOccurred())
			Expect(secondVersionedResources).To(HaveLen(1))
			Expect(secondVersionedResources[0].ID).To(Equal(versionedResources[0].ID))

			savedVersionedResourceID = versionedResources[0].ID
		})

		It("returns the builds for which the provided version id was an input", func() {
			builds, err := pipelineDB.GetBuildsWithVersionAsInput(savedVersionedResourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipelineDB.GetBuildsWithVersionAsInput(savedVersionedResourceID + 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})
	})

	Context("GetBuildsWithVersionAsOutput", func() {
		var savedVersionedResourceID int
		var expectedBuilds []db.Build

		BeforeEach(func() {
			build, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			_, err = pipelineDB.CreateJobBuild("some-other-job")
			Expect(err).NotTo(HaveOccurred())

			dbngBuild, found, err := dbngBuildFactory.Build(build.ID())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbngBuild.SaveOutput(dbng.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: dbng.ResourceVersion{
					"version": "v1",
				},
				Metadata: []dbng.ResourceMetadataField{
					{
						Name:  "some",
						Value: "value",
					},
				},
			}, true)
			Expect(err).NotTo(HaveOccurred())
			versionedResources, err := dbngBuild.GetVersionedResources()
			Expect(err).NotTo(HaveOccurred())
			Expect(versionedResources).To(HaveLen(1))

			dbngSecondBuild, found, err := dbngBuildFactory.Build(secondBuild.ID())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbngSecondBuild.SaveOutput(dbng.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: dbng.ResourceVersion{
					"version": "v1",
				},
				Metadata: []dbng.ResourceMetadataField{
					{
						Name:  "some",
						Value: "value",
					},
				},
			}, true)
			Expect(err).NotTo(HaveOccurred())

			secondVersionedResources, err := dbngBuild.GetVersionedResources()
			Expect(err).NotTo(HaveOccurred())
			Expect(secondVersionedResources).To(HaveLen(1))
			Expect(secondVersionedResources[0].ID).To(Equal(versionedResources[0].ID))

			savedVersionedResourceID = versionedResources[0].ID
		})

		It("returns the builds for which the provided version id was an output", func() {
			builds, err := pipelineDB.GetBuildsWithVersionAsOutput(savedVersionedResourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipelineDB.GetBuildsWithVersionAsOutput(savedVersionedResourceID + 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})
	})
})
