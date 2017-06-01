package dbng_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline", func() {
	var (
		pipeline       dbng.Pipeline
		team           dbng.Team
		pipelineConfig atc.Config
		job            dbng.Job
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
							TaskConfig: &atc.LoadTaskConfig{
								TaskConfig: &atc.TaskConfig{
									RootfsURI: "some-image",
								},
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
		pipeline, created, err = team.SavePipeline("fake-pipeline", pipelineConfig, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
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
			latestVR             dbng.SavedVersionedResource
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
				pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
				Expect(err).ToNot(HaveOccurred())

				resource, _, err := pipeline.Resource("some-resource")
				Expect(err).NotTo(HaveOccurred())

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
				Expect(err).NotTo(HaveOccurred())

				latestVR, found, err = pipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
			})

			It("gets latest version of resource", func() {
				Expect(found).To(BeTrue())

				Expect(latestVR.Version).To(Equal(dbng.ResourceVersion{"ref": "v3"}))
				Expect(latestVR.CheckOrder).To(Equal(2))
			})
		})

		Context("when the resource does not exist", func() {
			BeforeEach(func() {
				var err error
				latestVR, found, err = pipeline.GetLatestVersionedResource("dummy")
				Expect(err).NotTo(HaveOccurred())
			})

			It("gets latest version of resource", func() {
				Expect(found).To(BeFalse())
				Expect(latestVR).To(Equal(dbng.SavedVersionedResource{}))
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
				_, _, found, err := pipeline.GetResourceVersions("nope", dbng.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when resource has versions created in order of check order", func() {
			var versions []atc.Version
			var expectedVersions []dbng.SavedVersionedResource

			BeforeEach(func() {
				versions = nil
				expectedVersions = nil
				for i := 0; i < 10; i++ {
					version := atc.Version{"version": fmt.Sprintf("%d", i+1)}
					versions = append(versions, version)
					expectedVersions = append(expectedVersions,
						dbng.SavedVersionedResource{
							ID:      i + 1,
							Enabled: true,
							VersionedResource: dbng.VersionedResource{
								Resource: resource.Name,
								Type:     resource.Type,
								Version:  dbng.ResourceVersion(version),
								Metadata: nil,
							},
							CheckOrder: i + 1,
						})
				}

				err := pipeline.SaveResourceVersions(resource, versions)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("with no since/until", func() {
				It("returns the first page, with the given limit, and a next page", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{expectedVersions[9], expectedVersions[8]}))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(Equal(&dbng.Page{Since: expectedVersions[8].ID, Limit: 2}))
				})
			})

			Context("with a since that places it in the middle of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Since: expectedVersions[6].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{expectedVersions[5], expectedVersions[4]}))
					Expect(pagination.Previous).To(Equal(&dbng.Page{Until: expectedVersions[5].ID, Limit: 2}))
					Expect(pagination.Next).To(Equal(&dbng.Page{Since: expectedVersions[4].ID, Limit: 2}))
				})
			})

			Context("with a since that places it at the end of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Since: expectedVersions[2].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{expectedVersions[1], expectedVersions[0]}))
					Expect(pagination.Previous).To(Equal(&dbng.Page{Until: expectedVersions[1].ID, Limit: 2}))
					Expect(pagination.Next).To(BeNil())
				})
			})

			Context("with an until that places it in the middle of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Until: expectedVersions[6].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{expectedVersions[8], expectedVersions[7]}))
					Expect(pagination.Previous).To(Equal(&dbng.Page{Until: expectedVersions[8].ID, Limit: 2}))
					Expect(pagination.Next).To(Equal(&dbng.Page{Since: expectedVersions[7].ID, Limit: 2}))
				})
			})

			Context("with a until that places it at the beginning of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Until: expectedVersions[7].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{expectedVersions[9], expectedVersions[8]}))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(Equal(&dbng.Page{Since: expectedVersions[8].ID, Limit: 2}))
				})
			})

			Context("when the version has metadata", func() {
				BeforeEach(func() {
					metadata := []dbng.ResourceMetadataField{{Name: "name1", Value: "value1"}}

					expectedVersions[9].Metadata = metadata

					job, found, err := pipeline.Job("job-name")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					build, err := job.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					build.SaveInput(dbng.BuildInput{
						Name:              "some-input",
						VersionedResource: expectedVersions[9].VersionedResource,
						FirstOccurrence:   true,
					})
					// We resaved a previous SavedVersionedResource in SaveInput()
					// creating a new newest VersionedResource
					expectedVersions[9].CheckOrder = 10
				})

				It("returns the metadata in the version history", func() {
					historyPage, _, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 1})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{expectedVersions[9]}))
				})
			})

			Context("when a version is disabled", func() {
				BeforeEach(func() {
					pipeline.DisableVersionedResource(10)

					expectedVersions[9].Enabled = false
				})

				It("returns a disabled version", func() {
					historyPage, _, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 1})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{expectedVersions[9]}))
				})
			})
		})

		Context("when check orders are different than versions ids", func() {
			type versionData struct {
				ID         int
				CheckOrder int
				Version    atc.Version
			}

			dbVersion := func(vd versionData) dbng.SavedVersionedResource {
				return dbng.SavedVersionedResource{
					ID:      vd.ID,
					Enabled: true,
					VersionedResource: dbng.VersionedResource{
						Resource: resource.Name,
						Type:     resource.Type,
						Version:  dbng.ResourceVersion(vd.Version),
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
				Expect(err).NotTo(HaveOccurred())

				err = pipeline.SaveResourceVersions(resource, []atc.Version{
					{"v": "2"}, // id: 4, check_order: 4
					{"v": "3"}, // id: 2, check_order: 5
					{"v": "4"}, // id: 3, check_order: 6
				})
				Expect(err).NotTo(HaveOccurred())

				// ids ordered by check order now: [3, 2, 4, 1]
			})

			Context("with no since/until", func() {
				It("returns versions ordered by check order", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 4})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(4))
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{
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
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Since: 3, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
					}))
					Expect(pagination.Previous).To(Equal(&dbng.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&dbng.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with from", func() {
				It("returns the builds, with previous/next pages including from", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{From: 2, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
					}))
					Expect(pagination.Previous).To(Equal(&dbng.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&dbng.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with a until", func() {
				It("returns the builds, with previous/next pages excluding until", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Until: 1, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
					}))
					Expect(pagination.Previous).To(Equal(&dbng.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&dbng.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with to", func() {
				It("returns the builds, with previous/next pages including to", func() {
					historyPage, pagination, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{To: 4, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage).To(Equal([]dbng.SavedVersionedResource{
						dbVersion(versionData{ID: 2, CheckOrder: 5, Version: atc.Version{"v": "3"}}),
						dbVersion(versionData{ID: 4, CheckOrder: 4, Version: atc.Version{"v": "2"}}),
					}))
					Expect(pagination.Previous).To(Equal(&dbng.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&dbng.Page{Since: 4, Limit: 2}))
				})
			})
		})
	})

	Describe("SaveResourceVersions", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       atc.ResourceConfig
			pipeline             dbng.Pipeline
			resource             dbng.Resource
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
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			resource, _, err = pipeline.Resource("some-resource")
			Expect(err).NotTo(HaveOccurred())

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
			Expect(err).NotTo(HaveOccurred())

			latestVR, found, err := pipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(dbng.ResourceVersion{"ref": "v3"}))
			Expect(latestVR.CheckOrder).To(Equal(2))

			pretendCheckResults := []atc.Version{
				{"ref": "v2"},
				{"ref": "v3"},
			}

			err = pipeline.SaveResourceVersions(resourceConfig, pretendCheckResults)
			Expect(err).NotTo(HaveOccurred())

			latestVR, found, err = pipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(dbng.ResourceVersion{"ref": "v3"}))
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
		var savedVersion2 dbng.SavedVersionedResource
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
			pipeline, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
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
			Expect(err).NotTo(HaveOccurred())

			// save metadata for v2
			job, found, err := pipeline.Job("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).ToNot(HaveOccurred())

			err = build.SaveInput(dbng.BuildInput{
				Name: "some-input",
				VersionedResource: dbng.VersionedResource{
					Resource: "some-resource",
					Type:     "some-type",
					Version:  dbng.ResourceVersion{"version": "v2"},
					Metadata: []dbng.ResourceMetadataField{{Name: "name1", Value: "value1"}},
				},
				FirstOccurrence: true,
			})
			Expect(err).NotTo(HaveOccurred())

			savedVersions, _, found, err := pipeline.GetResourceVersions("some-resource", dbng.Page{Limit: 2})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedVersions).To(HaveLen(2))
			pipeline.DisableVersionedResource(savedVersions[0].ID)
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
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the SavedVersionedResource matching the given resource name and atc version", func() {
			By("returning versions that exist")
			actualSavedVersion, found, err := pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v2"},
				"some-resource",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(actualSavedVersion).To(Equal(savedVersion2))

			By("returning not found for versions that don't exist")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"versioni": "v2"},
				"some-resource",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("returning not found for versions that only exist in another resource")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v1"},
				"some-other-resource",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("returning not found for disabled versions")
			_, found, err = pipeline.GetVersionedResourceByVersion(
				atc.Version{"version": "v3"},
				"some-resource",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("Resource Versions", func() {
		resourceName := "some-resource"
		otherResourceName := "some-other-resource"
		reallyOtherResourceName := "some-really-other-resource"

		var (
			dbngPipeline        dbng.Pipeline
			otherDBNGPipeline   dbng.Pipeline
			resource            dbng.Resource
			otherResource       dbng.Resource
			reallyOtherResource dbng.Resource
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
								TaskConfig: &atc.LoadTaskConfig{
									TaskConfig: &atc.TaskConfig{
										RootfsURI: "some-image",
									},
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
			dbngPipeline, _, err = team.SavePipeline("pipeline-name", pipelineConfig, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			otherDBNGPipeline, _, err = team.SavePipeline("other-pipeline-name", otherPipelineConfig, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			resource, _, err = dbngPipeline.Resource(resourceName)
			Expect(err).NotTo(HaveOccurred())

			otherResource, _, err = dbngPipeline.Resource(otherResourceName)
			Expect(err).NotTo(HaveOccurred())

			reallyOtherResource, _, err = dbngPipeline.Resource(reallyOtherResourceName)
			Expect(err).NotTo(HaveOccurred())

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
			job, found, err := dbngPipeline.Job("some-job")
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			otherJob, found, err := dbngPipeline.Job("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			aJob, found, err := dbngPipeline.Job("a-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			sharedJob, found, err := dbngPipeline.Job("shared-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			randomJob, found, err := dbngPipeline.Job("random-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			otherSerialGroupJob, found, err := dbngPipeline.Job("other-serial-group-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := dbngPipeline.Job("different-serial-group-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err := dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
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
			_, found, err = dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including saved versioned resources of the current pipeline")
			err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).NotTo(HaveOccurred())

			savedVR1, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(savedVR1.ModifiedTime).NotTo(BeNil())
			Expect(savedVR1.ModifiedTime).To(BeTemporally(">", time.Time{}))

			err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "2"}})
			Expect(err).NotTo(HaveOccurred())

			savedVR2, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
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
			otherPipelineResource, _, err := otherDBNGPipeline.Resource("some-other-resource")
			Expect(err).NotTo(HaveOccurred())

			err = otherDBNGPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   otherPipelineResource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).NotTo(HaveOccurred())

			otherPipelineSavedVR, found, err := otherDBNGPipeline.GetLatestVersionedResource(otherPipelineResource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
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
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.SaveOutput(savedVR1.VersionedResource, false)
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
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
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.SaveOutput(savedVR1.VersionedResource, false)
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.Finish(dbng.BuildStatusFailed)
			Expect(err).NotTo(HaveOccurred())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
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
			anotherJob, found, err := otherDBNGPipeline.Job("a-job")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			otherPipelineBuild, err := anotherJob.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = otherPipelineBuild.SaveOutput(otherPipelineSavedVR.VersionedResource, false)
			Expect(err).NotTo(HaveOccurred())

			err = otherPipelineBuild.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())
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
			aJob, found, err = dbngPipeline.Job("a-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			build1DB, err = aJob.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.SaveInput(dbng.BuildInput{
				Name:              "some-input-name",
				VersionedResource: savedVR1.VersionedResource,
			})
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			versions, err = dbngPipeline.LoadVersionsDB()
			Expect(err).NotTo(HaveOccurred())

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
			_, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			By("including saved versioned resources of the current pipeline")
			err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}})
			Expect(err).NotTo(HaveOccurred())

			savedVR1, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "2"}})
			Expect(err).NotTo(HaveOccurred())

			savedVR2, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(savedVR1.Version).To(Equal(dbng.ResourceVersion{"version": "1"}))
			Expect(savedVR2.Version).To(Equal(dbng.ResourceVersion{"version": "2"}))

			By("not including saved versioned resources of other pipelines")
			_, _, err = otherDBNGPipeline.Resource("some-other-resource")
			Expect(err).NotTo(HaveOccurred())

			err = otherDBNGPipeline.SaveResourceVersions(atc.ResourceConfig{
				Name:   resource.Name(),
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}, []atc.Version{{"version": "1"}, {"version": "2"}, {"version": "3"}})
			Expect(err).NotTo(HaveOccurred())

			otherPipelineSavedVR, found, err := otherDBNGPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(otherPipelineSavedVR.Version).To(Equal(dbng.ResourceVersion{"version": "3"}))

			By("including disabled versions")
			err = dbngPipeline.DisableVersionedResource(savedVR2.ID)
			Expect(err).NotTo(HaveOccurred())

			latestVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version).To(Equal(dbng.ResourceVersion{"version": "2"}))
		})

		Describe("enabling and disabling versioned resources", func() {
			It("returns an error if the resource or version is bogus", func() {
				err := dbngPipeline.EnableVersionedResource(42)
				Expect(err).To(HaveOccurred())

				err = dbngPipeline.DisableVersionedResource(42)
				Expect(err).To(HaveOccurred())
			})

			It("does not affect explicitly fetching the latest version", func() {
				err := dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).NotTo(HaveOccurred())

				savedVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(dbng.ResourceVersion{"version": "1"}))
				initialTime := savedVR.ModifiedTime

				err = dbngPipeline.DisableVersionedResource(savedVR.ID)
				Expect(err).NotTo(HaveOccurred())

				disabledVR := savedVR
				disabledVR.Enabled = false

				latestVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Resource).To(Equal(disabledVR.Resource))
				Expect(latestVR.Type).To(Equal(disabledVR.Type))
				Expect(latestVR.Version).To(Equal(disabledVR.Version))
				Expect(latestVR.Enabled).To(BeFalse())
				Expect(latestVR.ModifiedTime).To(BeTemporally(">", initialTime))

				tmp_modified_time := latestVR.ModifiedTime

				err = dbngPipeline.EnableVersionedResource(savedVR.ID)
				Expect(err).NotTo(HaveOccurred())

				enabledVR := savedVR
				enabledVR.Enabled = true

				latestVR, found, err = dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Resource).To(Equal(enabledVR.Resource))
				Expect(latestVR.Type).To(Equal(enabledVR.Type))
				Expect(latestVR.Version).To(Equal(enabledVR.Version))
				Expect(latestVR.Enabled).To(BeTrue())
				Expect(latestVR.ModifiedTime).To(BeTemporally(">", tmp_modified_time))
			})

			It("doesn't change the check_order when saving a new build input", func() {
				err := dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).NotTo(HaveOccurred())

				job, found, err := dbngPipeline.Job("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				beforeVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).NotTo(HaveOccurred())

				input := dbng.BuildInput{
					Name:              "input-name",
					VersionedResource: beforeVR.VersionedResource,
				}

				err = build.SaveInput(input)
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't change the check_order when saving a new implicit build output", func() {
				err := dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).NotTo(HaveOccurred())

				job, found, err := dbngPipeline.Job("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				beforeVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).NotTo(HaveOccurred())

				err = build.SaveOutput(beforeVR.VersionedResource, false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't change the check_order when saving a new implicit build output", func() {
				err := dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				})
				Expect(err).NotTo(HaveOccurred())

				job, found, err := dbngPipeline.Job("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				beforeVR, found, err := dbngPipeline.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{
					{"version": "4"},
					{"version": "5"},
				})
				Expect(err).NotTo(HaveOccurred())

				err = build.SaveOutput(beforeVR.VersionedResource, true)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("saving versioned resources", func() {
			It("updates the latest versioned resource", func() {
				err := dbngPipeline.SaveResourceVersions(
					atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					},
					[]atc.Version{{"version": "1"}},
				)
				Expect(err).NotTo(HaveOccurred())

				savedResource, _, err := dbngPipeline.Resource("some-resource")
				Expect(err).NotTo(HaveOccurred())

				savedVR, found, err := dbngPipeline.GetLatestVersionedResource(savedResource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(dbng.ResourceVersion{"version": "1"}))

				err = dbngPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "2"}, {"version": "3"}})
				Expect(err).NotTo(HaveOccurred())

				savedVR, found, err = dbngPipeline.GetLatestVersionedResource(savedResource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Resource).To(Equal("some-resource"))
				Expect(savedVR.Type).To(Equal("some-type"))
				Expect(savedVR.Version).To(Equal(dbng.ResourceVersion{"version": "3"}))
			})
		})

		It("initially has no pending build for a job", func() {
			job, found, err := dbngPipeline.Job("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			pendingBuilds, err := job.GetPendingBuilds()
			Expect(err).NotTo(HaveOccurred())
			Expect(pendingBuilds).To(HaveLen(0))
		})

		Describe("marking resource checks as errored", func() {
			BeforeEach(func() {
				var err error
				resource, _, err = dbngPipeline.Resource("some-resource")
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the resource is first created", func() {
				It("is not errored", func() {
					Expect(resource.CheckError()).To(BeNil())
				})
			})

			Context("when a resource check is marked as errored", func() {
				It("is then marked as errored", func() {
					originalCause := errors.New("on fire")

					err := dbngPipeline.SetResourceCheckError(resource, originalCause)
					Expect(err).NotTo(HaveOccurred())

					returnedResource, _, err := dbngPipeline.Resource("some-resource")
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedResource.CheckError()).To(Equal(originalCause))
				})
			})

			Context("when a resource is cleared of check errors", func() {
				It("is not marked as errored again", func() {
					originalCause := errors.New("on fire")

					err := dbngPipeline.SetResourceCheckError(resource, originalCause)
					Expect(err).NotTo(HaveOccurred())

					err = dbngPipeline.SetResourceCheckError(resource, nil)
					Expect(err).NotTo(HaveOccurred())

					returnedResource, _, err := dbngPipeline.Resource("some-resource")
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedResource.CheckError()).To(BeNil())
				})
			})
		})
	})

	Describe("Disable and Enable Resource Versions", func() {
		var pipelineDB dbng.Pipeline
		var resource dbng.Resource

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
			pipelineDB, _, err = team.SavePipeline("some-pipeline", pipelineConfig, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())

			var found bool
			resource, found, err = pipelineDB.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})
		Context("when a version is disabled", func() {
			It("omits the version from the versions DB", func() {
				aJob, found, err := pipelineDB.Job("a-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				build1, err := aJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   resource.Name(),
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "disabled"}})
				Expect(err).NotTo(HaveOccurred())

				disabledVersion, found, err := pipelineDB.GetLatestVersionedResource(resource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = build1.SaveInput(dbng.BuildInput{
					Name:              "disabled-input",
					VersionedResource: disabledVersion.VersionedResource,
				})
				Expect(err).NotTo(HaveOccurred())

				err = build1.SaveOutput(disabledVersion.VersionedResource, false)
				Expect(err).NotTo(HaveOccurred())

				err = pipelineDB.SaveResourceVersions(atc.ResourceConfig{
					Name:   resource.Name(),
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "enabled"}})
				Expect(err).NotTo(HaveOccurred())

				enabledVersion, found, err := pipelineDB.GetLatestVersionedResource("some-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = build1.SaveInput(dbng.BuildInput{
					Name:              "enabled-input",
					VersionedResource: enabledVersion.VersionedResource,
				})
				Expect(err).NotTo(HaveOccurred())

				err = build1.SaveOutput(enabledVersion.VersionedResource, false)
				Expect(err).NotTo(HaveOccurred())

				err = build1.Finish(dbng.BuildStatusSucceeded)
				Expect(err).NotTo(HaveOccurred())

				pipelineDB.DisableVersionedResource(disabledVersion.ID)

				pipelineDB.DisableVersionedResource(enabledVersion.ID)
				pipelineDB.EnableVersionedResource(enabledVersion.ID)

				versions, err := pipelineDB.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())

				aJob, found, err = pipelineDB.Job("a-job")
				Expect(found).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())

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
			Expect(err).NotTo(HaveOccurred())

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
			Expect(err).NotTo(HaveOccurred())

			By("populating builds")
			job, found, err := pipeline.Job("job-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			By("populating build inputs")
			err = build.SaveInput(dbng.BuildInput{
				Name: "build-input",
				VersionedResource: dbng.VersionedResource{
					Resource: "resource-name",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("populating build outputs")
			err = build.SaveOutput(dbng.VersionedResource{
				Resource: "resource-name",
			}, false)
			Expect(err).NotTo(HaveOccurred())

			By("populating build events")
			err = build.SaveEvent(event.StartTask{})
			Expect(err).NotTo(HaveOccurred())

			// populate image_resource_versions table
			err = build.SaveImageResourceVersion("some-plan-id", atc.Version{"digest": "readers"}, `docker{"some":"source"}`)
			Expect(err).NotTo(HaveOccurred())

			err = pipeline.Destroy()
			Expect(err).NotTo(HaveOccurred())

			found, err = pipeline.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			found, err = build.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())

			_, found, err = team.Pipeline(pipeline.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("GetPendingBuilds/GetAllPendingBuilds", func() {
		Context("when a build is created", func() {
			BeforeEach(func() {
				_, err := job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the build", func() {
				pendingBuildsForJob, err := job.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuildsForJob).To(HaveLen(1))

				pendingBuilds, err := pipeline.GetAllPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuilds).To(HaveLen(1))
				Expect(pendingBuilds["job-name"]).NotTo(BeNil())
			})
		})
	})

	Describe("VersionsDB caching", func() {
		var otherPipeline dbng.Pipeline
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
			otherPipeline, _, err = team.SavePipeline("other-pipeline-name", otherPipelineConfig, 0, dbng.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when build outputs are added", func() {
			var build dbng.Build
			var savedVR dbng.SavedVersionedResource

			BeforeEach(func() {
				var err error
				job, found, err := pipeline.Job("job-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				build, err = job.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				err = pipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).NotTo(HaveOccurred())

				savedResource, _, err := pipeline.Resource("some-resource")
				Expect(err).NotTo(HaveOccurred())

				savedVR, found, err = pipeline.GetLatestVersionedResource(savedResource.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("will cache VersionsDB if no change has occured", func() {
				err := build.SaveOutput(savedVR.VersionedResource, true)
				Expect(err).NotTo(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())
				Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
			})

			It("will not cache VersionsDB if a change occured", func() {
				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())

				err = build.SaveOutput(savedVR.VersionedResource, true)
				Expect(err).NotTo(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())
				Expect(versionsDB != cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be different objects")
			})

			Context("when the build outputs are added for a different pipeline", func() {
				It("does not invalidate the cache for the original pipeline", func() {
					job, found, err := otherPipeline.Job("some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					otherBuild, err := job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())

					err = otherPipeline.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": "1"}})
					Expect(err).NotTo(HaveOccurred())

					otherSavedResource, _, err := otherPipeline.Resource("some-other-resource")
					Expect(err).NotTo(HaveOccurred())

					otherSavedVR, found, err := otherPipeline.GetLatestVersionedResource(otherSavedResource.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					versionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).NotTo(HaveOccurred())

					err = otherBuild.SaveOutput(otherSavedVR.VersionedResource, true)
					Expect(err).NotTo(HaveOccurred())

					cachedVersionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).NotTo(HaveOccurred())
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
				Expect(err).NotTo(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())
				Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
			})

			It("will not cache VersionsDB if a change occured", func() {
				err := pipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).NotTo(HaveOccurred())

				versionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())

				err = pipeline.SaveResourceVersions(atc.ResourceConfig{
					Name:   "some-other-resource",
					Type:   "some-type",
					Source: atc.Source{"some": "source"},
				}, []atc.Version{{"version": "1"}})
				Expect(err).NotTo(HaveOccurred())

				cachedVersionsDB, err := pipeline.LoadVersionsDB()
				Expect(err).NotTo(HaveOccurred())
				Expect(versionsDB != cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be different objects")
			})

			Context("when the versioned resources are added for a different pipeline", func() {
				It("does not invalidate the cache for the original pipeline", func() {
					err := pipeline.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": "1"}})
					Expect(err).NotTo(HaveOccurred())

					versionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).NotTo(HaveOccurred())

					err = otherPipeline.SaveResourceVersions(atc.ResourceConfig{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "source"},
					}, []atc.Version{{"version": "1"}})
					Expect(err).NotTo(HaveOccurred())

					cachedVersionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).NotTo(HaveOccurred())
					Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same object")
				})
			})
		})
	})

	Describe("Dashboard", func() {
		It("returns a Dashboard object with a DashboardJob corresponding to each configured job", func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			err = job.UpdateFirstLoggedBuildID(57)
			Expect(err).NotTo(HaveOccurred())

			otherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			aJob, found, err := pipeline.Job("a-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			sharedJob, found, err := pipeline.Job("shared-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			randomJob, found, err := pipeline.Job("random-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			otherSerialGroupJob, found, err := pipeline.Job("other-serial-group-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			differentSerialGroupJob, found, err := pipeline.Job("different-serial-group-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			By("returning jobs with no builds")
			expectedDashboard := dbng.Dashboard{
				{
					Job:           job,
					NextBuild:     nil,
					FinishedBuild: nil,
				},
				{
					Job:           otherJob,
					NextBuild:     nil,
					FinishedBuild: nil,
				},
				{
					Job:           aJob,
					NextBuild:     nil,
					FinishedBuild: nil,
				},
				{
					Job:           sharedJob,
					NextBuild:     nil,
					FinishedBuild: nil,
				},
				{
					Job:           randomJob,
					NextBuild:     nil,
					FinishedBuild: nil,
				},
				{
					Job:           otherSerialGroupJob,
					NextBuild:     nil,
					FinishedBuild: nil,
				},
				{
					Job:           differentSerialGroupJob,
					NextBuild:     nil,
					FinishedBuild: nil,
				},
			}

			actualDashboard, groups, err := pipeline.Dashboard()
			Expect(err).NotTo(HaveOccurred())

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
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			jobBuildOldDB, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			expectedDashboard[0].NextBuild = jobBuildOldDB

			actualDashboard, _, err = pipeline.Dashboard()
			Expect(err).NotTo(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(jobBuildOldDB.ID()))

			By("returning a job's most recent started build")
			jobBuildOldDB.Start("engine", "metadata")

			found, err = jobBuildOldDB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			expectedDashboard[0].NextBuild = jobBuildOldDB

			actualDashboard, _, err = pipeline.Dashboard()
			Expect(err).NotTo(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(jobBuildOldDB.ID()))
			Expect(actualDashboard[0].NextBuild.Status()).To(Equal(dbng.BuildStatusStarted))
			Expect(actualDashboard[0].NextBuild.Engine()).To(Equal("engine"))
			Expect(actualDashboard[0].NextBuild.EngineMetadata()).To(Equal("metadata"))

			By("returning a job's most recent started build even if there is a newer pending build")
			job, found, err = pipeline.Job("job-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			jobBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			expectedDashboard[0].NextBuild = jobBuildOldDB

			actualDashboard, _, err = pipeline.Dashboard()
			Expect(err).NotTo(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(jobBuildOldDB.ID()))

			By("returning a job's most recent finished build")
			err = jobBuild.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			found, err = jobBuild.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			expectedDashboard[0].FinishedBuild = jobBuild
			expectedDashboard[0].NextBuild = jobBuildOldDB

			actualDashboard, _, err = pipeline.Dashboard()
			Expect(err).NotTo(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(jobBuildOldDB.ID()))
			Expect(actualDashboard[0].FinishedBuild.ID()).To(Equal(jobBuild.ID()))

			By("returning a job's most recent finished build even when there is a newer unfinished build")
			job, found, err = pipeline.Job("job-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			jobBuildNewDB, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())
			jobBuildNewDB.Start("engine", "metadata")
			found, err = jobBuildNewDB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			expectedDashboard[0].FinishedBuild = jobBuild
			expectedDashboard[0].NextBuild = jobBuildNewDB

			actualDashboard, _, err = pipeline.Dashboard()
			Expect(err).NotTo(HaveOccurred())

			Expect(actualDashboard[0].Job.Name()).To(Equal(job.Name()))
			Expect(actualDashboard[0].NextBuild.ID()).To(Equal(jobBuildNewDB.ID()))
			Expect(actualDashboard[0].NextBuild.Status()).To(Equal(dbng.BuildStatusStarted))
			Expect(actualDashboard[0].NextBuild.Engine()).To(Equal("engine"))
			Expect(actualDashboard[0].NextBuild.EngineMetadata()).To(Equal("metadata"))
			Expect(actualDashboard[0].FinishedBuild.ID()).To(Equal(jobBuild.ID()))
		})
	})

	Describe("DeleteBuildEventsByBuildIDs", func() {
		It("deletes all build logs corresponding to the given build ids", func() {
			build1DB, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.SaveEvent(event.Log{
				Payload: "log 1",
			})
			Expect(err).NotTo(HaveOccurred())

			build2DB, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.SaveEvent(event.Log{
				Payload: "log 2",
			})
			Expect(err).NotTo(HaveOccurred())

			build3DB, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			err = build3DB.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			err = build1DB.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			err = build2DB.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			build4DB, err := team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			By("doing nothing if the list is empty")
			err = pipeline.DeleteBuildEventsByBuildIDs([]int{})
			Expect(err).NotTo(HaveOccurred())

			By("not returning an error")
			err = pipeline.DeleteBuildEventsByBuildIDs([]int{build3DB.ID(), build4DB.ID(), build1DB.ID()})
			Expect(err).NotTo(HaveOccurred())

			err = build4DB.Finish(dbng.BuildStatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			By("deleting events for build 1")
			events1, err := build1DB.Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events1.Close()

			_, err = events1.Next()
			Expect(err).To(Equal(dbng.ErrEndOfBuildEventStream))

			By("preserving events for build 2")
			events2, err := build2DB.Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events2.Close()

			build2Event1, err := events2.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(build2Event1).To(Equal(envelope(event.Log{
				Payload: "log 2",
			})))

			_, err = events2.Next() // finish event
			Expect(err).NotTo(HaveOccurred())

			_, err = events2.Next()
			Expect(err).To(Equal(dbng.ErrEndOfBuildEventStream))

			By("deleting events for build 3")
			events3, err := build3DB.Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events3.Close()

			_, err = events3.Next()
			Expect(err).To(Equal(dbng.ErrEndOfBuildEventStream))

			By("being unflapped by build 4, which had no events at the time")
			events4, err := build4DB.Events(0)
			Expect(err).NotTo(HaveOccurred())
			defer events4.Close()

			_, err = events4.Next() // finish event
			Expect(err).NotTo(HaveOccurred())

			_, err = events4.Next()
			Expect(err).To(Equal(dbng.ErrEndOfBuildEventStream))

			By("updating ReapTime for the affected builds")
			found, err := build1DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build1DB.ReapTime()).To(BeTemporally(">", build1DB.EndTime()))

			found, err = build2DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build2DB.ReapTime()).To(BeZero())

			found, err = build3DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build3DB.ReapTime()).To(Equal(build1DB.ReapTime()))

			found, err = build4DB.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			// Not required behavior, just a sanity check for what I think will happen
			Expect(build4DB.ReapTime()).To(Equal(build1DB.ReapTime()))
		})
	})

	Describe("Jobs", func() {
		var jobs []dbng.Job

		BeforeEach(func() {
			var err error
			jobs, err = pipeline.Jobs()
			Expect(err).NotTo(HaveOccurred())
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
		var expectedBuilds []dbng.Build

		BeforeEach(func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()

			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = someOtherJob.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			dbngBuild, found, err := buildFactory.Build(build.ID())
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

			dbngSecondBuild, found, err := buildFactory.Build(secondBuild.ID())
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
			builds, err := pipeline.GetBuildsWithVersionAsInput(savedVersionedResourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsInput(savedVersionedResourceID + 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(Equal([]dbng.Build{}))
		})
	})

	Describe("GetBuildsWithVersionAsOutput", func() {
		var savedVersionedResourceID int
		var expectedBuilds []dbng.Build

		BeforeEach(func() {
			job, found, err := pipeline.Job("job-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, build)

			secondBuild, err := job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())
			expectedBuilds = append(expectedBuilds, secondBuild)

			someOtherJob, found, err := pipeline.Job("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			_, err = someOtherJob.CreateBuild()
			Expect(err).NotTo(HaveOccurred())

			dbngBuild, found, err := buildFactory.Build(build.ID())
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

			dbngSecondBuild, found, err := buildFactory.Build(secondBuild.ID())
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
			builds, err := pipeline.GetBuildsWithVersionAsOutput(savedVersionedResourceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(ConsistOf(expectedBuilds))
		})

		It("returns an empty slice of builds when the provided version id doesn't exist", func() {
			builds, err := pipeline.GetBuildsWithVersionAsOutput(savedVersionedResourceID + 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(builds).To(Equal([]dbng.Build{}))
		})
	})
})
