package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCheckSessionLifecycle", func() {
	var (
		lifecycle db.ResourceConfigCheckSessionLifecycle
	)

	BeforeEach(func() {
		lifecycle = db.NewResourceConfigCheckSessionLifecycle(dbConn)
	})

	Describe("CleanInactiveResourceConfigCheckSessions", func() {
		expiry := db.ContainerOwnerExpiries{
			Min: 1 * time.Minute,
			Max: 1 * time.Minute,
		}

		Context("for resources", func() {
			findOrCreateSessionForDefaultResource := func() int {
				resourceConfigScope, err := defaultResource.SetResourceConfig(defaultResource.Source(), atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				owner := db.NewResourceConfigCheckSessionContainerOwner(
					resourceConfigScope.ResourceConfig().ID(),
					resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
					expiry,
				)

				var query sq.Eq
				var found bool
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

			var oldRccsID int

			BeforeEach(func() {
				By("creating the session")
				oldRccsID = findOrCreateSessionForDefaultResource()
			})

			It("keeps check sessions for active resources", func() {
				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newRccsID := findOrCreateSessionForDefaultResource()

				By("finding the same rccs as before")
				Expect(oldRccsID).To(Equal(newRccsID))
			})

			It("removes check sessions for inactive resources", func() {
				By("removing the default resource from the pipeline config")
				_, _, err := defaultTeam.SavePipeline(defaultPipelineRef, atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
					},
					Resources: atc.ResourceConfigs{},
					ResourceTypes: atc.ResourceTypes{
						{
							Name: "some-type",
							Type: "some-base-resource-type",
							Source: atc.Source{
								"some-type": "source",
							},
						},
					},
				}, defaultPipeline.ConfigVersion(), false)
				Expect(err).NotTo(HaveOccurred())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSessionForDefaultResource()

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})

			It("removes check sessions for resources in paused pipelines", func() {
				By("pausing the pipeline")
				Expect(defaultPipeline.Pause()).To(Succeed())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSessionForDefaultResource()

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})
		})

		Context("for resource types", func() {
			findOrCreateSessionForDefaultResourceType := func() int {
				resourceConfigScope, err := defaultResourceType.SetResourceConfig(
					defaultResourceType.Source(),
					atc.VersionedResourceTypes{},
				)
				Expect(err).ToNot(HaveOccurred())

				owner := db.NewResourceConfigCheckSessionContainerOwner(
					resourceConfigScope.ResourceConfig().ID(),
					resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
					expiry,
				)

				var query sq.Eq
				var found bool
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

			var oldRccsID int

			BeforeEach(func() {
				By("creating the session")
				oldRccsID = findOrCreateSessionForDefaultResourceType()
			})

			It("keeps check sessions for active resource types", func() {
				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSessionForDefaultResourceType()

				By("finding the same session as before")
				Expect(rccsID).To(Equal(oldRccsID))
			})

			It("removes check sessions for inactive resource types", func() {
				By("removing the default resource from the pipeline config")
				_, _, err := defaultTeam.SavePipeline(defaultPipelineRef, atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
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
					},
					ResourceTypes: atc.ResourceTypes{},
				}, defaultPipeline.ConfigVersion(), false)
				Expect(err).NotTo(HaveOccurred())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSessionForDefaultResourceType()

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})

			It("removes check sessions for resource types in paused pipelines", func() {
				By("pausing the pipeline")
				Expect(defaultPipeline.Pause()).To(Succeed())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				rccsID := findOrCreateSessionForDefaultResourceType()

				By("having created a new session, as the old one was removed")
				Expect(rccsID).ToNot(Equal(oldRccsID))
			})
		})
	})
})
