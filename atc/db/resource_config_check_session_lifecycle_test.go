package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCheckSessionLifecycle", func() {
	var (
		lifecycle      db.ResourceConfigCheckSessionLifecycle
		scenario       *dbtest.Scenario
		pipelineConfig atc.Config
	)

	BeforeEach(func() {
		lifecycle = db.NewResourceConfigCheckSessionLifecycle(dbConn)
		pipelineConfig = atc.Config{
			Resources: atc.ResourceConfigs{
				{
					Name:   "some-resource",
					Type:   "some-base-resource-type",
					Source: atc.Source{"some": "source"},
				},
			},
			ResourceTypes: atc.ResourceTypes{
				{
					Name: "some-type",
					Type: "some-base-resource-type",
					Source: atc.Source{
						"some-type": "source",
					},
				},
			},
			Prototypes: atc.Prototypes{
				{
					Name: "some-prototype",
					Type: "some-base-resource-type",
					Source: atc.Source{
						"some-prototype": "source",
					},
				},
			},
		}

		scenario = dbtest.Setup(builder.WithPipeline(pipelineConfig))
	})

	findOrCreateSession := func(resourceConfigID int) int {
		resourceConfig, found, err := resourceConfigFactory.FindResourceConfigByID(resourceConfigID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		owner := db.NewResourceConfigCheckSessionContainerOwner(
			resourceConfig.ID(),
			resourceConfig.OriginBaseResourceType().ID,
			db.ContainerOwnerExpiries{
				Min: 1 * time.Minute,
				Max: 1 * time.Minute,
			},
		)

		var query sq.Eq
		query, found, err = owner.Find(dbConn)
		Expect(err).ToNot(HaveOccurred())

		if !found {
			tx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			query, err = owner.Create(tx, defaultWorker.Name())
			Expect(err).ToNot(HaveOccurred())

			err = tx.Commit()
			Expect(err).ToNot(HaveOccurred())

			return query["resource_config_check_session_id"].(int)
		} else {
			rccsIDs := query["resource_config_check_session_id"].([]int)
			Expect(rccsIDs).To(HaveLen(1))
			return rccsIDs[0]
		}
	}

	Describe("CleanInactiveResourceConfigCheckSessions", func() {
		Context("for resources", func() {
			var oldRccsID int

			BeforeEach(func() {
				By("creating the session")
				scenario.Run(builder.WithResourceVersions("some-resource"))
				oldRccsID = findOrCreateSession(scenario.Resource("some-resource").ResourceConfigID())
			})

			It("keeps check sessions for active resources", func() {
				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newRccsID := findOrCreateSession(scenario.Resource("some-resource").ResourceConfigID())

				By("finding the same rccs as before")
				Expect(oldRccsID).To(Equal(newRccsID))
			})

			It("removes check sessions for inactive resources", func() {
				resources := pipelineConfig.Resources

				By("removing the default resource from the pipeline config")
				pipelineConfig.Resources = atc.ResourceConfigs{}
				scenario.Run(builder.WithPipeline(pipelineConfig))

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				pipelineConfig.Resources = resources
				scenario.Run(
					builder.WithPipeline(pipelineConfig),
					builder.WithResourceVersions("some-resource"),
				)
				rccsID := findOrCreateSession(scenario.Resource("some-resource").ResourceConfigID())

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})

			It("removes check sessions for resources in paused pipelines", func() {
				By("pausing the pipeline")
				Expect(scenario.Pipeline.Pause("")).To(Succeed())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSession(scenario.Resource("some-resource").ResourceConfigID())

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})
		})

		Context("for resource types", func() {
			var oldRccsID int

			BeforeEach(func() {
				By("creating the session")
				scenario.Run(builder.WithResourceTypeVersions("some-type"))
				oldRccsID = findOrCreateSession(scenario.ResourceType("some-type").ResourceConfigID())
			})

			It("keeps check sessions for active resource types", func() {
				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSession(scenario.ResourceType("some-type").ResourceConfigID())

				By("finding the same session as before")
				Expect(rccsID).To(Equal(oldRccsID))
			})

			It("removes check sessions for inactive resource types", func() {
				resourceTypes := pipelineConfig.ResourceTypes

				By("removing the default resource from the pipeline config")
				pipelineConfig.ResourceTypes = atc.ResourceTypes{}
				scenario.Run(builder.WithPipeline(pipelineConfig))

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				pipelineConfig.ResourceTypes = resourceTypes
				scenario.Run(builder.WithPipeline(pipelineConfig))
				rccsID := findOrCreateSession(scenario.ResourceType("some-type").ResourceConfigID())

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})

			It("removes check sessions for resource types in paused pipelines", func() {
				By("pausing the pipeline")
				Expect(scenario.Pipeline.Pause("")).To(Succeed())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSession(scenario.ResourceType("some-type").ResourceConfigID())

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})
		})

		Context("for prototypes", func() {
			var oldRccsID int

			BeforeEach(func() {
				By("creating the session")
				scenario.Run(builder.WithPrototypeVersions("some-prototype"))
				oldRccsID = findOrCreateSession(scenario.Prototype("some-prototype").ResourceConfigID())
			})

			It("keeps check sessions for active prototypes", func() {
				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSession(scenario.Prototype("some-prototype").ResourceConfigID())

				By("finding the same session as before")
				Expect(rccsID).To(Equal(oldRccsID))
			})

			It("removes check sessions for inactive prototypes", func() {
				prototypes := pipelineConfig.Prototypes

				By("removing the default prototype from the pipeline config")
				pipelineConfig.Prototypes = atc.Prototypes{}
				scenario.Run(builder.WithPipeline(pipelineConfig))

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				pipelineConfig.Prototypes = prototypes
				scenario.Run(builder.WithPipeline(pipelineConfig))
				rccsID := findOrCreateSession(scenario.Prototype("some-prototype").ResourceConfigID())

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})

			It("removes check sessions for resource types in paused pipelines", func() {
				By("pausing the pipeline")
				Expect(scenario.Pipeline.Pause("")).To(Succeed())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSession(scenario.Prototype("some-prototype").ResourceConfigID())

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})
		})
	})
})
