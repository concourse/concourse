package db_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource Factory", func() {
	var resourceFactory db.ResourceFactory

	BeforeEach(func() {
		resourceFactory = db.NewResourceFactory(dbConn, lockFactory)
	})

	Describe("VisibleResources", func() {
		BeforeEach(func() {
			otherTeam, err := teamFactory.CreateTeam(atc.Team{Name: "other-team"})
			Expect(err).NotTo(HaveOccurred())

			publicPipeline, _, err := otherTeam.SavePipeline("public-pipeline", atc.Config{
				Resources: atc.ResourceConfigs{
					{Name: "public-pipeline-resource"},
				},
			}, db.ConfigVersion(0), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
			Expect(publicPipeline.Expose()).To(Succeed())

			_, _, err = otherTeam.SavePipeline("private-pipeline", atc.Config{
				Resources: atc.ResourceConfigs{
					{Name: "private-pipeline-resource"},
				},
			}, db.ConfigVersion(0), db.PipelineUnpaused)
			Expect(err).ToNot(HaveOccurred())
		})

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
})
