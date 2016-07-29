package db_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource History", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB
	var pipelineDB db.PipelineDB
	var savedPipeline db.SavedPipeline

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		sqlDB = db.NewSQL(dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus)

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

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus)
		teamDB := teamDBFactory.GetTeamDB("some-team")
		savedPipeline, _, err = teamDB.SaveConfig("a-pipeline-name", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("GetResourceVersions", func() {
		var resource atc.ResourceConfig
		var versions []atc.Version
		var expectedVersions []db.SavedVersionedResource

		BeforeEach(func() {
			resource = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}

			versions = nil
			expectedVersions = nil
			for i := 0; i < 10; i++ {
				version := atc.Version{"version": fmt.Sprintf("%d", i+1)}
				versions = append(versions, version)
				expectedVersions = append(expectedVersions,
					db.SavedVersionedResource{
						ID:      i + 1,
						Enabled: true,
						VersionedResource: db.VersionedResource{
							Resource:   resource.Name,
							Type:       resource.Type,
							Version:    db.Version(version),
							Metadata:   nil,
							PipelineID: savedPipeline.ID,
						},
						CheckOrder: i + 1,
					})
			}

			err := pipelineDB.SaveResourceVersions(resource, versions)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the resource does not exist", func() {
			It("returns false and no error", func() {
				_, _, found, err := pipelineDB.GetResourceVersions("nope", db.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("with no since/until", func() {
			It("returns the first page, with the given limit, and a next page", func() {
				historyPage, pagination, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9], expectedVersions[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[8].CheckOrder, Limit: 2}))
			})
		})

		Context("with a since that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				historyPage, pagination, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Since: expectedVersions[6].CheckOrder, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[5], expectedVersions[4]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[5].CheckOrder, Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[4].CheckOrder, Limit: 2}))
			})
		})

		Context("with a since that places it at the end of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				historyPage, pagination, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Since: expectedVersions[2].CheckOrder, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[1], expectedVersions[0]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[1].CheckOrder, Limit: 2}))
				Expect(pagination.Next).To(BeNil())
			})
		})

		Context("with an until that places it in the middle of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				historyPage, pagination, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Until: expectedVersions[6].CheckOrder, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[8], expectedVersions[7]}))
				Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[8].CheckOrder, Limit: 2}))
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[7].CheckOrder, Limit: 2}))
			})
		})

		Context("with a until that places it at the beginning of the builds", func() {
			It("returns the builds, with previous/next pages", func() {
				historyPage, pagination, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Until: expectedVersions[7].ID, Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9], expectedVersions[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[8].CheckOrder, Limit: 2}))
			})
		})

		Context("when the version has metadata", func() {
			BeforeEach(func() {
				metadata := []db.MetadataField{{Name: "name1", Value: "value1"}}

				expectedVersions[9].Metadata = metadata

				build, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).ToNot(HaveOccurred())

				pipelineDB.SaveInput(build.ID(), db.BuildInput{
					Name:              "some-input",
					VersionedResource: expectedVersions[9].VersionedResource,
					FirstOccurrence:   true,
				})
			})

			It("returns the metadata in the version history", func() {
				historyPage, _, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 1})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				// We resaved a previous SavedVersionedResource in SaveInput()
				// creating a new newest VersionedResource
				expectedVersions[9].CheckOrder = 10
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9]}))
			})
		})

		Context("when a version is disabled", func() {
			BeforeEach(func() {
				pipelineDB.DisableVersionedResource(10)

				expectedVersions[9].Enabled = false
			})

			It("returns a disabled version", func() {
				historyPage, _, found, err := pipelineDB.GetResourceVersions("some-resource", db.Page{Limit: 1})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9]}))
			})
		})
	})

	Context("GetBuildsWithVersionAsInput", func() {
		var savedVersionedResource db.SavedVersionedResource
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

			savedVersionedResource, err = pipelineDB.SaveInput(build.ID(), db.BuildInput{
				Name: "some-input",
				VersionedResource: db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: db.Version{
						"version": "v1",
					},
					Metadata: []db.MetadataField{
						{
							Name:  "some",
							Value: "value",
						},
					},
					PipelineID: savedPipeline.ID,
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())

			savedVersionedResource, err = pipelineDB.SaveInput(secondBuild.ID(), db.BuildInput{
				Name: "some-input",
				VersionedResource: db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: db.Version{
						"version": "v1",
					},
					Metadata: []db.MetadataField{
						{
							Name:  "some",
							Value: "value",
						},
					},
					PipelineID: savedPipeline.ID,
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())

		})

		It("returns the builds for which the provided version id was an input", func() {
			builds, err := pipelineDB.GetBuildsWithVersionAsInput(savedVersionedResource.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipelineDB.GetBuildsWithVersionAsInput(savedVersionedResource.ID + 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})
	})

	Context("GetBuildsWithVersionAsOutput", func() {
		var savedVersionedResource db.SavedVersionedResource
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

			savedVersionedResource, err = pipelineDB.SaveOutput(build.ID(), db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: db.Version{
					"version": "v1",
				},
				Metadata: []db.MetadataField{
					{
						Name:  "some",
						Value: "value",
					},
				},
				PipelineID: savedPipeline.ID,
			}, false)
			Expect(err).NotTo(HaveOccurred())

			savedVersionedResource, err = pipelineDB.SaveOutput(secondBuild.ID(), db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: db.Version{
					"version": "v1",
				},
				Metadata: []db.MetadataField{
					{
						Name:  "some",
						Value: "value",
					},
				},
				PipelineID: savedPipeline.ID,
			}, false)
			Expect(err).NotTo(HaveOccurred())

		})

		It("returns the builds for which the provided version id was an output", func() {
			builds, err := pipelineDB.GetBuildsWithVersionAsOutput(savedVersionedResource.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipelineDB.GetBuildsWithVersionAsOutput(savedVersionedResource.ID + 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})
	})
})
