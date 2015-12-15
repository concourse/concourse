package db_test

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Keeping track of builds", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var database db.DB
	var pipelineDB db.PipelineDB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = postgresRunner.Open()
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		sqlDB := db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)

		pipelineDBFactory := db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, sqlDB)

		team, err := sqlDB.SaveTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())

		sqlDB.SaveConfig(team.Name, "some-pipeline", atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		pipelineDB, err = pipelineDBFactory.BuildWithTeamNameAndName(team.Name, "some-pipeline")
		Expect(err).NotTo(HaveOccurred())

		database = sqlDB
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("can get a build's inputs", func() {
		build, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).ToNot(HaveOccurred())

		expectedBuildInput, err := pipelineDB.SaveBuildInput(build.ID, db.BuildInput{
			Name: "some-input",
			VersionedResource: db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: db.Version{
					"some": "version",
				},
				Metadata: []db.MetadataField{
					{
						Name:  "meta1",
						Value: "data1",
					},
					{
						Name:  "meta2",
						Value: "data2",
					},
				},
				PipelineName: "some-pipeline",
			},
		})
		Expect(err).ToNot(HaveOccurred())

		actualBuildInput, err := database.GetBuildInputVersionedResouces(build.ID)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(actualBuildInput)).To(Equal(1))
		Expect(actualBuildInput[0]).To(Equal(expectedBuildInput))
	})

	It("can get a build's output", func() {
		build, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).ToNot(HaveOccurred())

		expectedBuildOutput, err := pipelineDB.SaveBuildOutput(build.ID, db.VersionedResource{
			Resource: "some-explicit-resource",
			Type:     "some-type",
			Version: db.Version{
				"some": "version",
			},
			Metadata: []db.MetadataField{
				{
					Name:  "meta1",
					Value: "data1",
				},
				{
					Name:  "meta2",
					Value: "data2",
				},
			},
			PipelineName: "some-pipeline",
		}, true)

		_, err = pipelineDB.SaveBuildOutput(build.ID, db.VersionedResource{
			Resource: "some-implicit-resource",
			Type:     "some-type",
			Version: db.Version{
				"some": "version",
			},
			Metadata: []db.MetadataField{
				{
					Name:  "meta1",
					Value: "data1",
				},
				{
					Name:  "meta2",
					Value: "data2",
				},
			},
			PipelineName: "some-pipeline",
		}, false)
		Expect(err).ToNot(HaveOccurred())

		actualBuildOutput, err := database.GetBuildOutputVersionedResouces(build.ID)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(actualBuildOutput)).To(Equal(1))
		Expect(actualBuildOutput[0]).To(Equal(expectedBuildOutput))
	})

	It("can get (no) resources from a one-off build", func() {
		oneOff, err := database.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())

		inputs, outputs, err := database.GetBuildResources(oneOff.ID)
		Expect(err).NotTo(HaveOccurred())

		Expect(inputs).To(BeEmpty())
		Expect(outputs).To(BeEmpty())
	})

	It("can create one-off builds with increasing names", func() {
		oneOff, err := database.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
		Expect(oneOff.ID).NotTo(BeZero())
		Expect(oneOff.JobName).To(BeZero())
		Expect(oneOff.Name).To(Equal("1"))
		Expect(oneOff.Status).To(Equal(db.StatusPending))

		oneOffGot, found, err := database.GetBuild(oneOff.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(oneOffGot).To(Equal(oneOff))

		jobBuild, err := pipelineDB.CreateJobBuild("some-other-job")
		Expect(err).NotTo(HaveOccurred())
		Expect(jobBuild.Name).To(Equal("1"))

		nextOneOff, err := database.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
		Expect(nextOneOff.ID).NotTo(BeZero())
		Expect(nextOneOff.ID).NotTo(Equal(oneOff.ID))
		Expect(nextOneOff.JobName).To(BeZero())
		Expect(nextOneOff.Name).To(Equal("2"))
		Expect(nextOneOff.Status).To(Equal(db.StatusPending))

		allBuilds, _, err := database.GetBuilds(db.Page{Limit: 100})
		Expect(err).NotTo(HaveOccurred())
		Expect(allBuilds).To(Equal([]db.Build{nextOneOff, jobBuild, oneOff}))
	})

	Describe("GetAllStartedBuilds", func() {
		var build1 db.Build
		var build2 db.Build
		BeforeEach(func() {
			var err error

			build1, err = database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			build2, err = pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			_, err = database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			started, err := database.StartBuild(build1.ID, "some-engine", "so-meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())

			started, err = database.StartBuild(build2.ID, "some-engine", "so-meta")
			Expect(err).NotTo(HaveOccurred())
			Expect(started).To(BeTrue())
		})

		It("returns all builds that have been started, regardless of pipeline", func() {
			builds, err := database.GetAllStartedBuilds()
			Expect(err).NotTo(HaveOccurred())

			Expect(len(builds)).To(Equal(2))

			build1, found, err := database.GetBuild(build1.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			build2, found, err := database.GetBuild(build2.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(builds).To(ConsistOf(build1, build2))
		})
	})

	Describe("GetBuilds", func() {
		var allBuilds [5]db.Build

		BeforeEach(func() {
			for i := 0; i < 3; i++ {
				var err error
				allBuilds[i], err = database.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())
			}

			for i := 3; i < 5; i++ {
				var err error
				allBuilds[i], err = pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("returns all builds that have been started, regardless of pipeline", func() {
			builds, pagination, err := database.GetBuilds(db.Page{Limit: 2})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(builds)).To(Equal(2))
			Expect(builds[0]).To(Equal(allBuilds[4]))
			Expect(builds[1]).To(Equal(allBuilds[3]))

			Expect(pagination.Previous).To(BeNil())
			Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[3].ID, Limit: 2}))

			builds, pagination, err = database.GetBuilds(*pagination.Next)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(builds)).To(Equal(2))
			Expect(builds[0]).To(Equal(allBuilds[2]))
			Expect(builds[1]).To(Equal(allBuilds[1]))

			Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].ID, Limit: 2}))
			Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].ID, Limit: 2}))

			builds, pagination, err = database.GetBuilds(*pagination.Next)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(builds)).To(Equal(1))
			Expect(builds[0]).To(Equal(allBuilds[0]))

			Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[0].ID, Limit: 2}))
			Expect(pagination.Next).To(BeNil())

			builds, pagination, err = database.GetBuilds(*pagination.Previous)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(builds)).To(Equal(2))
			Expect(builds[0]).To(Equal(allBuilds[2]))
			Expect(builds[1]).To(Equal(allBuilds[1]))

			Expect(pagination.Previous).To(Equal(&db.Page{Until: allBuilds[2].ID, Limit: 2}))
			Expect(pagination.Next).To(Equal(&db.Page{Since: allBuilds[1].ID, Limit: 2}))
		})
	})
})
