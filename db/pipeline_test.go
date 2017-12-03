package db_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline", func() {
	var (
		pipeline       db.Pipeline
		team           db.Team
		pipelineConfig atc.Config
		job            db.Job
	)

	BeforeEach(func() {
		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "some-team"})
		Expect(err).ToNot(HaveOccurred())

		pipelineConfig = atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"job-1", "job-2"},
					Resources: []string{"some-resource", "some-other-resource"},
				},
			},
			Jobs: atc.JobConfigs{
				{
					Name: "job-name",

					Public: true,

					Serial: true,

					SerialGroups: []string{"serial-group"},

					Plan: atc.PlanSequence{
						{
							Put: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
						},
						{
							Get:      "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed:  []string{"job-1", "job-2"},
							Trigger: true,
						},
						{
							Task:           "some-task",
							Privileged:     true,
							TaskConfigPath: "some/config/path.yml",
							TaskConfig: &atc.TaskConfig{
								RootfsURI: "some-image",
							},
						},
					},
				},
				{
					Name:   "some-other-job",
					Serial: true,
				},
				{
					Name: "a-job",
				},
				{
					Name: "shared-job",
				},
				{
					Name: "random-job",
				},
				{
					Name:         "other-serial-group-job",
					SerialGroups: []string{"serial-group", "really-different-group"},
				},
				{
					Name:         "different-serial-group-job",
					SerialGroups: []string{"different-serial-group"},
				},
			},
			Resources: atc.ResourceConfigs{
				{Name: "resource-name"},
				{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				},
			},
		}
		var created bool
		pipeline, created, err = team.SavePipeline("fake-pipeline", pipelineConfig, db.ConfigVersion(0), db.PipelineUnpaused)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())

		var found bool
		job, found, err = pipeline.Job("job-name")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
	})

	Describe("CheckPaused", func() {
		var paused bool
		JustBeforeEach(func() {
			var err error
			paused, err = pipeline.CheckPaused()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the pipeline is unpaused", func() {
			BeforeEach(func() {
				Expect(pipeline.Unpause()).To(Succeed())
			})

			It("returns the pipeline is paused", func() {
				Expect(paused).To(BeFalse())
			})
		})

		Context("when the pipeline is paused", func() {
			BeforeEach(func() {
				Expect(pipeline.Pause()).To(Succeed())
			})

			It("returns the pipeline is paused", func() {
				Expect(paused).To(BeTrue())
			})
		})
	})

	Describe("Pause", func() {
		JustBeforeEach(func() {
			Expect(pipeline.Pause()).To(Succeed())

			found, err := pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when the pipeline is unpaused", func() {
			BeforeEach(func() {
				Expect(pipeline.Unpause()).To(Succeed())
			})

			It("pauses the pipeline", func() {
				Expect(pipeline.Paused()).To(BeTrue())
			})
		})
	})

	Describe("Unpause", func() {
		JustBeforeEach(func() {
			Expect(pipeline.Unpause()).To(Succeed())

			found, err := pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when the pipeline is paused", func() {
			BeforeEach(func() {
				Expect(pipeline.Pause()).To(Succeed())
			})

			It("unpauses the pipeline", func() {
				Expect(pipeline.Paused()).To(BeFalse())
			})
		})
	})

	Describe("Rename", func() {
		JustBeforeEach(func() {
			Expect(pipeline.Rename("oopsies")).To(Succeed())
		})

		It("renames the pipeline", func() {
			pipeline, found, err := team.Pipeline("oopsies")
			Expect(pipeline.Name()).To(Equal("oopsies"))
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("GetLatestVersionedResource", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       atc.ResourceConfig
			latestVR             db.SavedVersionedResource
			found                bool
		)

		Context("when the resource exists", func() {
			BeforeEach(func() {
				var err error
				pipelineConfig := atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name: "some-resource",
							Type: "some-type",
							Source: atc.Source{
								"source-config": "some-value",
							},
						},
					},
				}
				pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				resource, _, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				resourceConfig = atc.ResourceConfig{
					Name:   resource.Name(),
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}

				originalVersionSlice = []atc.Version{
					{"ref": "v1"},
					{"ref": "v3"},
				}

				err = pipeline.SaveResourceVersions(resourceConfig, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				latestVR, found, err = pipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).ToNot(HaveOccurred())
			})

			It("gets latest version of resource", func() {
				Expect(found).To(BeTrue())

				Expect(latestVR.Version).To(Equal(db.ResourceVersion{"ref": "v3"}))
				Expect(latestVR.CheckOrder).To(Equal(2))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				var err error
				latestVR, found, err = pipeline.GetLatestVersionedResource("dummy")
				Expect(err).ToNot(HaveOccurred())
			})

			It("gets latest version of resource", func() {
				Expect(found).To(BeFalse())
				Expect(latestVR).To(Equal(db.SavedVersionedResource{}))
			})
		})
	})

	Context("GetResourceVersions", func() {
		var resource atc.ResourceConfig

		BeforeEach(func() {
			resource = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}
		})

		Context("when the resource does not exist", func() {
			It("returns false and no error", func() {
				_, _, found, err := pipeline.GetResourceVersions("nope", db.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when resource has versions created in order of check order", func() {
			var versions []atc.Version
			var expectedVersions []db.SavedVersionedResource

			BeforeEach(func() {
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
								Resource: resource.Name,
								Type:     resource.Type,
								Version:  db.ResourceVersion(version),
								Metadata: nil,
							},
							CheckOrder: i + 1,
						})
				}

				err := pipeline.SaveResourceVersions(resource, versions)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("with no since/until", func() {
				It("returns the first page, with the given limit, and a next page", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9], expectedVersions[8]}))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[8].ID, Limit: 2}))
				})
			})

			Context("with a since that places it in the middle of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Since: expectedVersions[6].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[5], expectedVersions[4]}))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[5].ID, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[4].ID, Limit: 2}))
				})
			})

			Context("with a since that places it at the end of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Since: expectedVersions[2].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[1], expectedVersions[0]}))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[1].ID, Limit: 2}))
					Expect(pagination.Next).To(BeNil())
				})
			})

			Context("with an until that places it in the middle of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Until: expectedVersions[6].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[8], expectedVersions[7]}))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: expectedVersions[8].ID, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[7].ID, Limit: 2}))
				})
			})

			Context("with a until that places it at the beginning of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Until: expectedVersions[7].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9], expectedVersions[8]}))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(Equal(&db.Page{Since: expectedVersions[8].ID, Limit: 2}))
				})
			})

			Context("when the version has metadata", func() {
				BeforeEach(func() {
					metadata := []db.ResourceMetadataField{{Name: "name1", Value: "value1"}}

					expectedVersions[9].Metadata = metadata

					job, found, err := pipeline.Job("job-name")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					build, err := job.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					err = build.SaveInput(db.BuildInput{
						Name:              "some-input",
						VersionedResource: expectedVersions[9].VersionedResource,
						FirstOccurrence:   true,
					})
					Expect(err).ToNot(HaveOccurred())
					// We resaved a previous SavedVersionedResource in SaveInput()
					// creating a new newest VersionedResource
					expectedVersions[9].CheckOrder = 10
				})

				It("returns the metadata in the version history", func() {
					historyPage, _, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Limit: 1})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9]}))
				})
			})

			Context("when a version is disabled", func() {
				BeforeEach(func() {
					err := pipeline.DisableVersionedResource(10)
					Expect(err).ToNot(HaveOccurred())

					expectedVersions[9].Enabled = false
				})

				It("returns a disabled version", func() {
					historyPage, _, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Limit: 1})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{expectedVersions[9]}))
				})
			})
		})

		Context("when check orders are different than versions ids", func() {
			type versionData struct {
				ID         int
				CheckOrder int
				Version    atc.Version
			}

			dbVersion := func(vd versionData) db.SavedVersionedResource {
				return db.SavedVersionedResource{
					ID:      vd.ID,
					Enabled: true,
					VersionedResource: db.VersionedResource{
						Resource: resource.Name,
						Type:     resource.Type,
						Version:  db.ResourceVersion(vd.Version),
						Metadata: nil,
					},
					CheckOrder: vd.CheckOrder,
				}
			}

			BeforeEach(func() {
				err := pipeline.SaveResourceVersions(resource, []atc.Version{
					{"v": "1"}, // id: 1, check_order: 1
					{"v": "3"}, // id: 2, check_order: 2
					{"v": "4"}, // id: 3, check_order: 3
				})
				Expect(err).ToNot(HaveOccurred())

				err = pipeline.SaveResourceVersions(resource, []atc.Version{
					{"v": "2"}, // id: 4, check_order: 4
					{"v": "3"}, // id: 2, check_order: 5
					{"v": "4"}, // id: 3, check_order: 6
				})
				Expect(err).ToNot(HaveOccurred())

				// ids ordered by check order now: [3, 2, 4, 1]
			})

			Context("with no since/until", func() {
				It("returns versions ordered by check order", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Limit: 4})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(4))
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{
						dbVersion(versionData{ID: 3, CheckOrder: 6, Version: atc.Version{"v": "4"}}),
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
						dbVersion(versionData{ID: 1, CheckOrder: 1, Version: atc.Version{"v": "1"}}),
					}))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(BeNil())
				})
			})

			Context("with a since", func() {
				It("returns the builds, with previous/next pages excluding since", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Since: 3, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
					}))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with from", func() {
				It("returns the builds, with previous/next pages including from", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{From: 2, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
					}))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with a until", func() {
				It("returns the builds, with previous/next pages excluding until", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Until: 1, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
					}))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with to", func() {
				It("returns the builds, with previous/next pages including to", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", db.Page{To: 4, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage).To(Equal([]db.SavedVersionedResource{
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
					}))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})
		})
	})

	Describe("SaveResourceVersions", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       atc.ResourceConfig
			pipeline             db.Pipeline
			resource             db.Resource
		)

		BeforeEach(func() {
			var err error
			pipelineConfig := atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},
			}
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			resource, _, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())

			resourceConfig = atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}

			originalVersionSlice = []atc.Version{
				{"ref": "v1"},
				{"ref": "v3"},
			}
		})

		It("ensures versioned resources have the correct check_order", func() {
			err := pipeline.SaveResourceVersions(resourceConfig, originalVersionSlice)
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err := pipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(db.ResourceVersion{"ref": "v3"}))
			Expect(latestVR.CheckOrder).To(Equal(2))

			pretendCheckResults := []atc.Version{
				{"ref": "v2"},
				{"ref": "v3"},
			}

			err = pipeline.SaveResourceVersions(resourceConfig, pretendCheckResults)
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err = pipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(db.ResourceVersion{"ref": "v3"}))
			Expect(latestVR.CheckOrder).To(Equal(4))
		})

		Context("resource not found", func() {
			BeforeEach(func() {
				resourceConfig = atc.ResourceConfig{
					Name:   "unknown-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}

				originalVersionSlice = []atc.Version{{"ref": "v1"}}
			})

			It("returns an error", func() {
				err := pipeline.SaveResourceVersions(resourceConfig, originalVersionSlice)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetVersionedResourceByVersion", func() {
		var savedVersion2 db.SavedVersionedResource
		BeforeEach(func() {
			var err error
			pipelineConfig := atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
					{
						Name: "some-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
			}
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			err = pipeline.SaveResourceVersions(
				atc.ResourceConfig{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
				[]atc.Version{
					{"version": "v1"},
					{"version": "v2"},
					{"version": "v3"}, // disabled
				},
			)
			Expect(err).ToNot(HaveOccurred())

			// save metadata for v2
			job, found, err := pipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build.SaveInput(db.BuildInput{
				Name: "some-input",
				VersionedResource: db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version:  db.ResourceVersion{"version": "v2"},
					Metadata: []db.ResourceMetadataField{{Name: "name1", Value: "value1"}},
				},
				FirstOccurrence: true,
			})
			Expect(err).ToNot(HaveOccurred())

			savedVersions, _, found, err := pipeline.GetResourceVersions("some-resource", db.Page{Limit: 2})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedVersions).To(HaveLen(2))
			err = pipeline.DisableVersionedResource(savedVersions[0].ID)
			Expect(err).ToNot(HaveOccurred())

			savedVersion2 = savedVersions[1]

			err = pipeline.SaveResourceVersions(
				atc.ResourceConfig{
					Name: "some-other-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
				[]atc.Version{
					{"version": "v2"},
				},
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the SavedVersionedResource matching the given resource name and atc version", func() {
			By("returning versions that exist")
			actualSavedVersion, found, err := pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v2"},
				"some-resource",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualSavedVersion).To(Equal(savedVersion2))

			By("returning not found for versions that don't exist")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"versioni": "v2"},
				"some-resource",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			By("returning not found for versions that only exist in another resource")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v1"},
				"some-other-resource",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			By("returning not found for disabled versions")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v3"},
				"some-resource",
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("Resource Versions", func() {
		resourceName := "some-resource"
		otherResourceName := "some-other-resource"
		reallyOtherResourceName := "some-really-other-resource"

		var (
			dbPipeline          db.Pipeline
			otherDBPipeline     db.Pipeline
			resource            db.Resource
			otherResource       db.Resource
			reallyOtherResource db.Resource
		)

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"some-resource", "some-other-resource"},
					},
				},

				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
					{
						Name: "some-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
					{
						Name: "some-really-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				ResourceTypes: atc.ResourceTypes{
					{
						Name: "some-resource-type",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "some-job",

						Public: true,

						Serial: true,

						SerialGroups: []string{"serial-group"},

						Plan: atc.PlanSequence{
							{
								Put: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
							},
							{
								Get:      "some-input",
								Resource: "some-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed:  []string{"job-1", "job-2"},
								Trigger: true,
							},
							{
								Task:           "some-task",
								Privileged:     true,
								TaskConfigPath: "some/config/path.yml",
								TaskConfig: &atc.TaskConfig{
									RootfsURI: "some-image",
								},
							},
						},
					},
					{
						Name:   "some-other-job",
						Serial: true,
					},
					{
						Name: "a-job",
					},
					{
						Name: "shared-job",
					},
					{
						Name: "random-job",
					},
					{
						Name:         "other-serial-group-job",
						SerialGroups: []string{"serial-group", "really-different-group"},
					},
					{
						Name:         "different-serial-group-job",
						SerialGroups: []string{"different-serial-group"},
					},
				},
			}

			otherPipelineConfig := atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"some-resource", "some-other-resource"},
					},
				},

				Resources: atc.ResourceConfigs{
					{
						Name: "some-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
					{
						Name: "some-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"source-config": "some-value",
						},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
					{
						Name: "some-other-job",
					},
					{
						Name: "a-job",
					},
					{
						Name: "shared-job",
					},
					{
						Name: "other-serial-group-job",
					},
				},
			}

			var err error
			dbPipeline, _, err = team.SavePipeline("pipeline-name", pipelineConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherDBPipeline, _, err = team.SavePipeline("other-pipeline-name", otherPipelineConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			resource, _, err = dbPipeline.Resource(resourceName)
			Expect(err).ToNot(HaveOccurred())

			otherResource, _, err = dbPipeline.Resource(otherResourceName)
			Expect(err).ToNot(HaveOccurred())

			reallyOtherResource, _, err = dbPipeline.Resource(reallyOtherResourceName)
			Expect(err).ToNot(HaveOccurred())

		})

		It("returns correct resource", func() {
			Expect(resource.Name()).To(Equal("some-resource"))
			Expect(resource.Paused()).To(Equal(false))
			Expect(resource.PipelineName()).To(Equal("pipeline-name"))
			Expect(resource.CheckError()).To(BeNil())
			Expect(resource.Type()).To(Equal("some-type"))
			Expect(resource.Source()).To(Equal(atc.Source{"source-config": "some-value"}))
		})

		It("can load up versioned resource information relevant to scheduling", func() {
			job, found, err := dbPipeline.Job("some-job")
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			otherJob, found, err := dbPipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			aJob, found, err := dbPipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			sharedJob, found, err := dbPipeline.Job("shared-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			randomJob, found, err := dbPipeline.Job("random-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherSerialGroupJob, found, err := dbPipeline.Job("other-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := dbPipeline.Job("different-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err := dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(BeEmpty())
			Expect(versions.BuildOutputs).To(BeEmpty())
			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"some-other-job":             otherJob.ID(),
				"a-job":                      aJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("initially having no latest versioned resource")
			_, found, err = dbPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including saved versioned resources of the current pipeline")
			err = dbPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).ToNot(HaveOccurred())

			savedVR1, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedVR1.ModifiedTime).ToNot(BeNil())
			Expect(savedVR1.ModifiedTime).To(BeTemporally(">", time.Time{}))

			err = dbPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "2"}})
			Expect(err).ToNot(HaveOccurred())

			savedVR2, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
			}))

			Expect(versions.BuildOutputs).To(BeEmpty())
			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"some-other-job":             otherJob.ID(),
				"a-job":                      aJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("not including saved versioned resources of other pipelines")
			otherPipelineResource, _, err := otherDBPipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())

			err = otherDBPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   otherPipelineResource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).ToNot(HaveOccurred())

			otherPipelineSavedVR, found, err := otherDBPipeline.GetLatestVersionedResource(otherPipelineResource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
			}))

			Expect(versions.BuildOutputs).To(BeEmpty())
			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"some-other-job":             otherJob.ID(),
				"a-job":                      aJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("including outputs of successful builds")
			build1DB, err := aJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.SaveOutput(savedVR1.VersionedResource, false)
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
			}))

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder,
					},
					JobID:   aJob.ID(),
					BuildID: build1DB.ID(),
				},
			}))

			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"a-job":                      aJob.ID(),
				"some-other-job":             otherJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("not including outputs of failed builds")
			build2DB, err := aJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.SaveOutput(savedVR1.VersionedResource, false)
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
			}))

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder,
					},
					JobID:   aJob.ID(),
					BuildID: build1DB.ID(),
				},
			}))

			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"a-job":                      aJob.ID(),
				"some-other-job":             otherJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("not including outputs of builds in other pipelines")
			anotherJob, found, err := otherDBPipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherPipelineBuild, err := anotherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = otherPipelineBuild.SaveOutput(otherPipelineSavedVR.VersionedResource, false)
			Expect(err).ToNot(HaveOccurred())

			err = otherPipelineBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())
			Expect(versions.ResourceVersions).To(ConsistOf([]algorithm.ResourceVersion{
				{VersionID: savedVR1.ID, ResourceID: resource.ID(), CheckOrder: savedVR1.CheckOrder},
				{VersionID: savedVR2.ID, ResourceID: resource.ID(), CheckOrder: savedVR2.CheckOrder},
			}))

			Expect(versions.BuildOutputs).To(ConsistOf([]algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder,
					},
					JobID:   aJob.ID(),
					BuildID: build1DB.ID(),
				},
			}))

			Expect(versions.ResourceIDs).To(Equal(map[string]int{
				resource.Name():            resource.ID(),
				otherResource.Name():       otherResource.ID(),
				reallyOtherResource.Name(): reallyOtherResource.ID(),
			}))

			Expect(versions.JobIDs).To(Equal(map[string]int{
				"some-job":                   job.ID(),
				"a-job":                      aJob.ID(),
				"some-other-job":             otherJob.ID(),
				"shared-job":                 sharedJob.ID(),
				"random-job":                 randomJob.ID(),
				"other-serial-group-job":     otherSerialGroupJob.ID(),
				"different-serial-group-job": differentSerialGroupJob.ID(),
			}))

			By("including build inputs")
			aJob, found, err = dbPipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build1DB, err = aJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.SaveInput(db.BuildInput{
				Name:              "some-input-name",
				VersionedResource: savedVR1.VersionedResource,
			})
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			versions, err = dbPipeline.LoadVersionsDB()
			Expect(err).ToNot(HaveOccurred())

			Expect(versions.BuildInputs).To(ConsistOf([]algorithm.BuildInput{
				{
					ResourceVersion: algorithm.ResourceVersion{
						VersionID:  savedVR1.ID,
						ResourceID: resource.ID(),
						CheckOrder: savedVR1.CheckOrder,
					},
					JobID:     aJob.ID(),
					BuildID:   build1DB.ID(),
					InputName: "some-input-name",
				},
			}))
		})

		It("can load up the latest versioned resource, enabled or not", func() {
			By("initially having no latest versioned resource")
			_, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including saved versioned resources of the current pipeline")
			err = dbPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).ToNot(HaveOccurred())

			savedVR1, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "2"}})
			Expect(err).ToNot(HaveOccurred())

			savedVR2, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(savedVR1.Version).To(Equal(db.ResourceVersion{"version": "1"}))
			Expect(savedVR2.Version).To(Equal(db.ResourceVersion{"version": "2"}))

			By("not including saved versioned resources of other pipelines")
			_, _, err = otherDBPipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())

			err = otherDBPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}, {"version": "2"}, {"version": "3"}})
			Expect(err).ToNot(HaveOccurred())

			otherPipelineSavedVR, found, err := otherDBPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(otherPipelineSavedVR.Version).To(Equal(db.ResourceVersion{"version": "3"}))

			By("including disabled versions")
			err = dbPipeline.DisableVersionedResource(savedVR2.ID)
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(db.ResourceVersion{"version": "2"}))
		})

		Describe("enabling and disabling versioned resources", func() {
			It("returns an error if the resource or version is bogus", func() {
				err := dbPipeline.EnableVersionedResource(42)
				Expect(err).To(HaveOccurred())

				err = dbPipeline.DisableVersionedResource(42)
				Expect(err).To(HaveOccurred())
			})

			It("does not affect explicitly fetching the latest version", func() {
				err := dbPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				savedVR, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(db.ResourceVersion{"version": "1"}))
				initialTime := savedVR.ModifiedTime

				err = dbPipeline.DisableVersionedResource(savedVR.ID)
				Expect(err).ToNot(HaveOccurred())

				disabledVR := savedVR
				disabledVR.Enabled = false

				latestVR, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Resource).To(Equal(disabledVR.Resource))
				Expect(latestVR.Type).To(Equal(disabledVR.Type))
				Expect(latestVR.Version).To(Equal(disabledVR.Version))
				Expect(latestVR.Enabled).To(BeFalse())
				Expect(latestVR.ModifiedTime).To(BeTemporally(">", initialTime))

				tmp_modified_time := latestVR.ModifiedTime

				err = dbPipeline.EnableVersionedResource(savedVR.ID)
				Expect(err).ToNot(HaveOccurred())

				enabledVR := savedVR
				enabledVR.Enabled = true

				latestVR, found, err = dbPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Resource).To(Equal(enabledVR.Resource))
				Expect(latestVR.Type).To(Equal(enabledVR.Type))
				Expect(latestVR.Version).To(Equal(enabledVR.Version))
				Expect(latestVR.Enabled).To(BeTrue())
				Expect(latestVR.ModifiedTime).To(BeTemporally(">", tmp_modified_time))
			})

			It("doesn't change the check_order when saving a new build input", func() {
				err := dbPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).ToNot(HaveOccurred())

				job, found, err := dbPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				beforeVR, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).ToNot(HaveOccurred())

				input := db.BuildInput{
					Name:              "input-name",
					VersionedResource: beforeVR.VersionedResource,
				}

				err = build.SaveInput(input)
				Expect(err).ToNot(HaveOccurred())
			})

			It("doesn't change the check_order when saving a new implicit build output", func() {
				err := dbPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).ToNot(HaveOccurred())

				job, found, err := dbPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				beforeVR, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveOutput(beforeVR.VersionedResource, false)
				Expect(err).ToNot(HaveOccurred())
			})

			It("doesn't change the check_order when saving a new implicit build output", func() {
				err := dbPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).ToNot(HaveOccurred())

				job, found, err := dbPipeline.Job("some-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				beforeVR, found, err := dbPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveOutput(beforeVR.VersionedResource, true)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("saving versioned resources", func() {
			It("updates the latest versioned resource", func() {
				err := dbPipeline.SaveResourceVersions(
					atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					[]atc.Version{{"version": "1"}},
				)
				Expect(err).ToNot(HaveOccurred())

				savedResource, _, err := dbPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				savedVR, found, err := dbPipeline.GetLatestVersionedResource(savedResource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(db.ResourceVersion{"version": "1"}))

				err = dbPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}, {"version": "3"}})
				Expect(err).ToNot(HaveOccurred())

				savedVR, found, err = dbPipeline.GetLatestVersionedResource(savedResource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(db.ResourceVersion{"version": "3"}))
			})
		})

		It("initially has no pending build for a job", func() {
			job, found, err := dbPipeline.Job("some-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pendingBuilds, err := job.GetPendingBuilds()
			Expect(err).ToNot(HaveOccurred())
			Expect(pendingBuilds).To(HaveLen(0))
		})

		Describe("marking resource checks as errored", func() {
			BeforeEach(func() {
				var err error
				resource, _, err = dbPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the resource is first created", func() {
				It("is not errored", func() {
					Expect(resource.CheckError()).To(BeNil())
				})
			})

			Context("when a resource check is marked as errored", func() {
				It("is then marked as errored", func() {
					originalCause := errors.New("on fire")

					err := dbPipeline.SetResourceCheckError(resource, originalCause)
					Expect(err).ToNot(HaveOccurred())

					returnedResource, _, err := dbPipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())

					Expect(returnedResource.CheckError()).To(Equal(originalCause))
				})
			})

			Context("when a resource is cleared of check errors", func() {
				It("is not marked as errored again", func() {
					originalCause := errors.New("on fire")

					err := dbPipeline.SetResourceCheckError(resource, originalCause)
					Expect(err).ToNot(HaveOccurred())

					err = dbPipeline.SetResourceCheckError(resource, nil)
					Expect(err).ToNot(HaveOccurred())

					returnedResource, _, err := dbPipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())

					Expect(returnedResource.CheckError()).To(BeNil())
				})
			})
		})
	})

	Describe("Disable and Enable Resource Versions", func() {
		var pipelineDB db.Pipeline
		var resource db.Resource

		BeforeEach(func() {
			pipelineConfig := atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "a-job",
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
				},
			}
			var err error
			pipelineDB, _, err = team.SavePipeline("some-pipeline", pipelineConfig, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			resource, found, err = pipelineDB.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})
		Context("when a version is disabled", func() {
			It("omits the version from the versions DB", func() {
				aJob, found, err := pipelineDB.Job("a-job")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build1, err := aJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   resource.Name(),
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "disabled"}})
				Expect(err).ToNot(HaveOccurred())

				disabledVersion, found, err := pipelineDB.GetLatestVersionedResource(resource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = build1.SaveInput(db.BuildInput{
					Name:              "disabled-input",
					VersionedResource: disabledVersion.VersionedResource,
				})
				Expect(err).ToNot(HaveOccurred())

				err = build1.SaveOutput(disabledVersion.VersionedResource, false)
				Expect(err).ToNot(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   resource.Name(),
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "enabled"}})
				Expect(err).ToNot(HaveOccurred())

				enabledVersion, found, err := pipelineDB.GetLatestVersionedResource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = build1.SaveInput(db.BuildInput{
					Name:              "enabled-input",
					VersionedResource: enabledVersion.VersionedResource,
				})
				Expect(err).ToNot(HaveOccurred())

				err = build1.SaveOutput(enabledVersion.VersionedResource, false)
				Expect(err).ToNot(HaveOccurred())

				err = build1.Finish(db.BuildStatusSucceeded)
				Expect(err).ToNot(HaveOccurred())

				err = pipelineDB.DisableVersionedResource(disabledVersion.ID)
				Expect(err).ToNot(HaveOccurred())

				err = pipelineDB.DisableVersionedResource(enabledVersion.ID)
				Expect(err).ToNot(HaveOccurred())

				err = pipelineDB.EnableVersionedResource(enabledVersion.ID)
				Expect(err).ToNot(HaveOccurred())

				versions, err := pipelineDB.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				aJob, found, err = pipelineDB.Job("a-job")
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				By("omitting it from the list of resource versions")
				Expect(versions.ResourceVersions).To(ConsistOf(
					algorithm.ResourceVersion{
						VersionID:  enabledVersion.ID,
						ResourceID: resource.ID(),
						CheckOrder: enabledVersion.CheckOrder,
					},
				))

				By("omitting it from build outputs")
				Expect(versions.BuildOutputs).To(ConsistOf(
					algorithm.BuildOutput{
						ResourceVersion: algorithm.ResourceVersion{
							VersionID:  enabledVersion.ID,
							ResourceID: resource.ID(),
							CheckOrder: enabledVersion.CheckOrder,
						},
						JobID:   aJob.ID(),
						BuildID: build1.ID(),
					},
				))

				By("omitting it from build inputs")
				Expect(versions.BuildInputs).To(ConsistOf(
					algorithm.BuildInput{
						ResourceVersion: algorithm.ResourceVersion{
							VersionID:  enabledVersion.ID,
							ResourceID: resource.ID(),
							CheckOrder: enabledVersion.CheckOrder,
						},
						JobID:     aJob.ID(),
						BuildID:   build1.ID(),
						InputName: "enabled-input",
					},
				))
			})
		})
	})

	Describe("Destroy", func() {
		It("removes the pipeline and all of its data", func() {
			By("populating resources table")
			resource, found, err := pipeline.Resource("resource-name")
			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			By("populating resource versions")
			err = pipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   resource.Type(),
				Source: resource.Source(),
			}, []atc.Version{
				{
					"key": "value",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			By("populating builds")
			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			By("populating build inputs")
			err = build.SaveInput(db.BuildInput{
				Name: "build-input",
				VersionedResource: db.VersionedResource{
					Resource: "resource-name",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			By("populating build outputs")
			err = build.SaveOutput(db.VersionedResource{
				Resource: "resource-name",
			}, false)
			Expect(err).ToNot(HaveOccurred())

			By("populating build events")
			err = build.SaveEvent(event.StartTask{})
			Expect(err).ToNot(HaveOccurred())

			err = pipeline.Destroy()
			Expect(err).ToNot(HaveOccurred())

			found, err = pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			found, err = build.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			_, found, err = team.Pipeline(pipeline.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("GetPendingBuilds/GetAllPendingBuilds", func() {
		Context("when a build is created", func() {
			BeforeEach(func() {
				_, err := job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the build", func() {
				pendingBuildsForJob, err := job.GetPendingBuilds()
				Expect(err).ToNot(HaveOccurred())
				Expect(pendingBuildsForJob).To(HaveLen(1))

				pendingBuilds, err := pipeline.GetAllPendingBuilds()
				Expect(err).ToNot(HaveOccurred())
				Expect(pendingBuilds).To(HaveLen(1))
				Expect(pendingBuilds["job-name"]).ToNot(BeNil())
			})
		})
	})

	Describe("VersionsDB caching", func() {
		var otherPipeline db.Pipeline
		BeforeEach(func() {
			otherPipelineConfig := atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name: "some-other-resource",
						Type: "some-type",
						Source: atc.Source{
							"some": "source",
						},
					},
				},
				Jobs: atc.JobConfigs{
					{
						Name: "some-job",
					},
				},
			}
			var err error
			otherPipeline, _, err = team.SavePipeline("other-pipeline-name", otherPipelineConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when build outputs are added", func() {
			var build db.Build
			var savedVR db.SavedVersionedResource

			BeforeEach(func() {
				var err error
				job, found, err := pipeline.Job("job-name")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				err = pipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				savedResource, _, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				savedVR, found, err = pipeline.GetLatestVersionedResource(savedResource.Name())
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("will cache VersionsDB if no change has occured", func() {
				err := build.SaveOutput(savedVR.VersionedResource, true)
				Expect(err).ToNot(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
			})

			It("will not cache VersionsDB if a change occured", func() {
				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				err = build.SaveOutput(savedVR.VersionedResource, true)
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB != cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be different objects")
			})

			Context("when the build outputs are added for a different pipeline", func() {
				It("does not invalidate the cache for the original pipeline", func() {
					job, found, err := otherPipeline.Job("some-job")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					otherBuild, err := job.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					err = otherPipeline.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": "1"}})
					Expect(err).ToNot(HaveOccurred())

					otherSavedResource, _, err := otherPipeline.Resource("some-other-resource")
					Expect(err).ToNot(HaveOccurred())

					otherSavedVR, found, err := otherPipeline.GetLatestVersionedResource(otherSavedResource.Name())
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					versionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())

					err = otherBuild.SaveOutput(otherSavedVR.VersionedResource, true)
					Expect(err).ToNot(HaveOccurred())

					cachedVersionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())
					Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
				})
			})
		})

		Context("when versioned resources are added", func() {
			It("will cache VersionsDB if no change has occured", func() {
				err := pipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
			})

			It("will not cache VersionsDB if a change occured", func() {
				err := pipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())

				err = pipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).ToNot(HaveOccurred())
				Expect(versionsDB != cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be different objects")
			})

			Context("when the versioned resources are added for a different pipeline", func() {
				It("does not invalidate the cache for the original pipeline", func() {
					err := pipeline.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": "1"}})
					Expect(err).ToNot(HaveOccurred())

					versionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())

					err = otherPipeline.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": "1"}})
					Expect(err).ToNot(HaveOccurred())

					cachedVersionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())
					Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
				})
			})
		})
	})

	Describe("Dashboard", func() {
		It("returns a Dashboard object with a DashboardJob corresponding to each configured job", func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = job.UpdateFirstLoggedBuildID(57)
			Expect(err).ToNot(HaveOccurred())

			otherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			aJob, found, err := pipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			sharedJob, found, err := pipeline.Job("shared-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			randomJob, found, err := pipeline.Job("random-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherSerialGroupJob, found, err := pipeline.Job("other-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := pipeline.Job("different-serial-group-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			By("returning jobs with no builds")
			actualDashboard, groups, err := pipeline.Dashboard("")
			Expect(err).ToNot(HaveOccurred())

			Expect(groups).To(Equal(pipelineConfig.Groups))
			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[1].Job.Name()).To(Equal(otherJob.Name()))
			Expect(actualDashboard[2].Job.Name()).To(Equal(aJob.Name()))
			Expect(actualDashboard[3].Job.Name()).To(Equal(sharedJob.Name()))
			Expect(actualDashboard[4].Job.Name()).To(Equal(randomJob.Name()))
			Expect(actualDashboard[5].Job.Name()).To(Equal(otherSerialGroupJob.Name()))
			Expect(actualDashboard[6].Job.Name()).To(Equal(differentSerialGroupJob.Name()))

			By("returning a job's most recent pending build if there are no running builds")
			job, found, err = pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			firstJobBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, _, err = pipeline.Dashboard("")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(firstJobBuild.ID()))

			By("returning a job's most recent started build")
			found, err = firstJobBuild.Start("engine", `{"meta":"data"}`, atc.Plan{})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			found, err = firstJobBuild.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			actualDashboard, _, err = pipeline.Dashboard("")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(firstJobBuild.ID()))
			Expect(actualDashboard[0].NextBuild.Status()).To(Equal(db.BuildStatusStarted))
			Expect(actualDashboard[0].NextBuild.Engine()).To(Equal("engine"))
			Expect(actualDashboard[0].NextBuild.EngineMetadata()).To(Equal(`{"meta":"data"}`))

			By("returning a job's most recent started build even if there is a newer pending build")
			job, found, err = pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			secondJobBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, _, err = pipeline.Dashboard("")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(firstJobBuild.ID()))

			By("returning a job's most recent finished build")
			err = firstJobBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = secondJobBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			found, err = secondJobBuild.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			actualDashboard, _, err = pipeline.Dashboard("")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild).To(BeNil())
			Expect(actualDashboard[0].FinishedBuild.ID()).To(Equal(secondJobBuild.ID()))

			By("returning a job's transition build as nil when there are no builds")
			otherPipeline, _, err := team.SavePipeline("other-pipeline-name", pipelineConfig, 0, db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherJob, found, err = otherPipeline.Job("random-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherJobBuild, err := otherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = otherJobBuild.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			job, found, err = pipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			jobBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = jobBuild.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			_, found, err = pipeline.Job("random-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			actualDashboard, _, err = pipeline.Dashboard("transitionBuilds")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[4].Job.Name()).To(Equal(randomJob.Name()))
			Expect(actualDashboard[4].TransitionBuild).To(BeNil())

			By("returning a job's transition build as nil when there are only pending builds")
			job, found, err = pipeline.Job("random-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, _, err = pipeline.Dashboard("transitionBuilds")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[4].TransitionBuild).To(BeNil())

			By("returning a job's first build as transition build when all builds have the same status")
			job, found, err = pipeline.Job("random-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			transitionBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = transitionBuild.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			jobBuild, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = jobBuild.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, _, err = pipeline.Dashboard("transitionBuilds")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[4].Job.Name()).To(Equal(randomJob.Name()))
			Expect(actualDashboard[4].TransitionBuild.ID()).To(Equal(transitionBuild.ID()))

			By("returning a job's transition build when there are builds with different statuses")
			job, found, err = pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			jobBuild, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = jobBuild.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			jobBuild, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = jobBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			otherJobBuild, err = otherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = otherJobBuild.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			transitionBuild, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = transitionBuild.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			otherJobBuild, err = otherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = otherJobBuild.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			jobBuild, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = jobBuild.Finish(db.BuildStatusFailed)
			Expect(err).ToNot(HaveOccurred())

			_, err = job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			actualDashboard, _, err = pipeline.Dashboard("")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].TransitionBuild).To(BeNil())

			actualDashboard, _, err = pipeline.Dashboard("transitionBuilds")
			Expect(err).ToNot(HaveOccurred())

			Expect(actualDashboard[0].TransitionBuild.ID()).To(Equal(transitionBuild.ID()))
		})
	})

	Describe("DeleteBuildEventsByBuildIDs", func() {
		It("deletes all build logs corresponding to the given build ids", func() {
			build1DB, err := team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.SaveEvent(event.Log{
				Payload: "log 1",
			})
			Expect(err).ToNot(HaveOccurred())

			build2DB, err := team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.SaveEvent(event.Log{
				Payload: "log 2",
			})
			Expect(err).ToNot(HaveOccurred())

			build3DB, err := team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build3DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = build1DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = build2DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			build4DB, err := team.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			By("doing nothing if the list is empty")
			err = pipeline.DeleteBuildEventsByBuildIDs([]int{})
			Expect(err).ToNot(HaveOccurred())

			By("not returning an error")
			err = pipeline.DeleteBuildEventsByBuildIDs([]int{build3DB.ID(), build4DB.ID(), build1DB.ID()})
			Expect(err).ToNot(HaveOccurred())

			err = build4DB.Finish(db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			By("deleting events for build 1")
			events1, err := build1DB.Events(0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events1)

			_, err = events1.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("preserving events for build 2")
			events2, err := build2DB.Events(0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events2)

			build2Event1, err := events2.Next()
			Expect(err).ToNot(HaveOccurred())
			Expect(build2Event1).To(Equal(envelope(event.Log{
				Payload: "log 2",
			})))

			_, err = events2.Next() // finish event
			Expect(err).ToNot(HaveOccurred())

			_, err = events2.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("deleting events for build 3")
			events3, err := build3DB.Events(0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events3)

			_, err = events3.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("being unflapped by build 4, which had no events at the time")
			events4, err := build4DB.Events(0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events4)

			_, err = events4.Next() // finish event
			Expect(err).ToNot(HaveOccurred())

			_, err = events4.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("updating ReapTime for the affected builds")
			found, err := build1DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build1DB.ReapTime()).To(BeTemporally(">", build1DB.EndTime()))

			found, err = build2DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build2DB.ReapTime()).To(BeZero())

			found, err = build3DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build3DB.ReapTime()).To(Equal(build1DB.ReapTime()))

			found, err = build4DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			// Not required behavior, just a sanity check for what I think will happen
			Expect(build4DB.ReapTime()).To(Equal(build1DB.ReapTime()))
		})
	})

	Describe("Jobs", func() {
		var jobs []db.Job

		BeforeEach(func() {
			var err error
			jobs, err = pipeline.Jobs()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns all the jobs", func() {
			Expect(jobs[0].Name()).To(Equal("job-name"))
			Expect(jobs[1].Name()).To(Equal("some-other-job"))
			Expect(jobs[2].Name()).To(Equal("a-job"))
			Expect(jobs[3].Name()).To(Equal("shared-job"))
			Expect(jobs[4].Name()).To(Equal("random-job"))
			Expect(jobs[5].Name()).To(Equal("other-serial-group-job"))
			Expect(jobs[6].Name()).To(Equal("different-serial-group-job"))
		})
	})

	Describe("GetBuildsWithVersionAsInput", func() {
		var savedVersionedResourceID int
		var expectedBuilds []db.Build

		BeforeEach(func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()

			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = someOtherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			dbBuild, found, err := buildFactory.Build(build.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbBuild.SaveInput(db.BuildInput{
				Name: "some-input",
				VersionedResource: db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: db.ResourceVersion{
						"version": "v1",
					},
					Metadata: []db.ResourceMetadataField{
						{
							Name:  "some",
							Value: "value",
						},
					},
				},
				FirstOccurrence: true,
			})
			Expect(err).ToNot(HaveOccurred())
			versionedResources, err := dbBuild.GetVersionedResources()
			Expect(err).ToNot(HaveOccurred())
			Expect(versionedResources).To(HaveLen(1))

			dbSecondBuild, found, err := buildFactory.Build(secondBuild.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			err = dbSecondBuild.SaveInput(db.BuildInput{
				Name: "some-input",
				VersionedResource: db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version: db.ResourceVersion{
						"version": "v1",
					},
					Metadata: []db.ResourceMetadataField{
						{
							Name:  "some",
							Value: "value",
						},
					},
				},
				FirstOccurrence: true,
			})
			Expect(err).ToNot(HaveOccurred())
			secondVersionedResources, err := dbBuild.GetVersionedResources()
			Expect(err).ToNot(HaveOccurred())
			Expect(secondVersionedResources).To(HaveLen(1))
			Expect(secondVersionedResources[0].ID).To(Equal(versionedResources[0].ID))

			savedVersionedResourceID = versionedResources[0].ID
		})

		It("returns the builds for which the provided version id was an input", func() {
			builds, err := pipeline.GetBuildsWithVersionAsInput(savedVersionedResourceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsInput(savedVersionedResourceID + 100)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})
	})

	Describe("GetBuildsWithVersionAsOutput", func() {
		var savedVersionedResourceID int
		var expectedBuilds []db.Build

		BeforeEach(func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = someOtherJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			dbBuild, found, err := buildFactory.Build(build.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbBuild.SaveOutput(db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: db.ResourceVersion{
					"version": "v1",
				},
				Metadata: []db.ResourceMetadataField{
					{
						Name:  "some",
						Value: "value",
					},
				},
			}, true)
			Expect(err).ToNot(HaveOccurred())
			versionedResources, err := dbBuild.GetVersionedResources()
			Expect(err).ToNot(HaveOccurred())
			Expect(versionedResources).To(HaveLen(1))

			dbSecondBuild, found, err := buildFactory.Build(secondBuild.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbSecondBuild.SaveOutput(db.VersionedResource{
				Resource: "some-resource",
				Type:     "some-type",
				Version: db.ResourceVersion{
					"version": "v1",
				},
				Metadata: []db.ResourceMetadataField{
					{
						Name:  "some",
						Value: "value",
					},
				},
			}, true)
			Expect(err).ToNot(HaveOccurred())

			secondVersionedResources, err := dbBuild.GetVersionedResources()
			Expect(err).ToNot(HaveOccurred())
			Expect(secondVersionedResources).To(HaveLen(1))
			Expect(secondVersionedResources[0].ID).To(Equal(versionedResources[0].ID))

			savedVersionedResourceID = versionedResources[0].ID
		})

		It("returns the builds for which the provided version id was an output", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(savedVersionedResourceID)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(savedVersionedResourceID + 100)
			Expect(err).ToNot(HaveOccurred())
			Expect(builds).To(Equal([]db.Build{}))
		})
	})
})
