package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource Factory", func() {
	var resourceFactory db.ResourceFactory

	BeforeEach(func() {
		resourceFactory = db.NewResourceFactory(dbConn, lockFactory)
	})

	Describe("Public And Private Resources", func() {
		var publicPipeline, privatePipeline db.Pipeline

		BeforeEach(func() {
			otherTeam, err := teamFactory.CreateTeam(atc.Team{Name: "other-team"})
			Expect(err).NotTo(HaveOccurred())

			publicPipeline, _, err = otherTeam.SavePipeline("public-pipeline", atc.Config{
				Resources: atc.ResourceConfigs{
					{Name: "public-pipeline-resource"},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())
			Expect(publicPipeline.Expose()).To(Succeed())

			privatePipeline, _, err = otherTeam.SavePipeline("private-pipeline", atc.Config{
				Resources: atc.ResourceConfigs{
					{Name: "private-pipeline-resource"},
				},
			}, db.ConfigVersion(0), false)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("VisibleResources", func() {
			It("returns resources in the provided teams and resources in public pipelines", func() {
				visibleResources, err := resourceFactory.VisibleResources([]string{"default-team"})
				Expect(err).ToNot(HaveOccurred())

				Expect(len(visibleResources)).To(Equal(2))
				Expect(visibleResources[0].Name()).To(Equal("some-resource"))
				Expect(visibleResources[1].Name()).To(Equal("public-pipeline-resource"))
			})

			It("returns team name and groups for each resource", func() {
				visibleResources, err := resourceFactory.VisibleResources([]string{"default-team"})
				Expect(err).ToNot(HaveOccurred())

				Expect(visibleResources[0].TeamName()).To(Equal("default-team"))
				Expect(visibleResources[1].TeamName()).To(Equal("other-team"))
			})

		})

		Context("AllResources", func() {
			It("returns all private and public resources from all teams", func() {
				visibleResources, err := resourceFactory.AllResources()
				Expect(err).ToNot(HaveOccurred())

				Expect(len(visibleResources)).To(Equal(3))
				Expect(visibleResources[0].Name()).To(Equal("some-resource"))
				Expect(visibleResources[1].Name()).To(Equal("public-pipeline-resource"))
				Expect(visibleResources[2].Name()).To(Equal("private-pipeline-resource"))
			})

			It("returns team name and groups for each resource", func() {
				visibleResources, err := resourceFactory.AllResources()
				Expect(err).ToNot(HaveOccurred())

				Expect(visibleResources[0].TeamName()).To(Equal("default-team"))
				Expect(visibleResources[1].TeamName()).To(Equal("other-team"))
			})
		})

		Context("ResourcesForPipelines", func() {
			var multipleResourcesPipeline db.Pipeline

			BeforeEach(func() {
				var err error
				multipleResourcesPipeline, _, err = defaultTeam.SavePipeline("multiple-resource-pipeline", atc.Config{
					Resources: atc.ResourceConfigs{
						{Name: "resource-1"},
						{Name: "resource-2"},
						{Name: "inactive-resource"},
					},
				}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				pausedPipeline, _, err := defaultTeam.SavePipeline("paused-pipeline", atc.Config{
					Resources: atc.ResourceConfigs{
						{Name: "resource-paused"},
					},
				}, db.ConfigVersion(0), false)
				Expect(err).ToNot(HaveOccurred())

				err = pausedPipeline.Pause()
				Expect(err).ToNot(HaveOccurred())

				_, err = dbConn.Exec(`UPDATE resources SET active = false WHERE name = 'inactive-resource'`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a map of all resources for all pipelines", func() {
				resources, err := resourceFactory.ResourcesForPipelines()
				Expect(err).ToNot(HaveOccurred())

				Expect(resources).To(HaveLen(4))

				defaultPipelineResource, found, err := defaultPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				publicPipelineResource, found, err := publicPipeline.Resource("public-pipeline-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				privatePipelineResource, found, err := privatePipeline.Resource("private-pipeline-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				resource1, found, err := multipleResourcesPipeline.Resource("resource-1")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				resource2, found, err := multipleResourcesPipeline.Resource("resource-2")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				expectedResources := map[int][]int{
					defaultPipelineResource.ID():   []int{defaultPipelineResource.ID()},
					publicPipelineResource.ID():    []int{publicPipelineResource.ID()},
					privatePipelineResource.ID():   []int{privatePipelineResource.ID()},
					multipleResourcesPipeline.ID(): []int{resource1.ID(), resource2.ID()},
				}

				Expect(resources).To(Equal(expectedResources))
			})
		})
	})
})
