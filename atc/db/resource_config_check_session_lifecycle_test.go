package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"

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
			GraceTime: 5 * time.Second,
			Min:       1 * time.Minute,
			Max:       1 * time.Minute,
		}

		Context("for resources", func() {
			findOrCreateSessionForDefaultResource := func() db.ResourceConfigCheckSession {
				resourceConfigCheckSession, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(logger,
					defaultResource.Type(),
					defaultResource.Source(),
					creds.VersionedResourceTypes{},
					expiry,
				)
				Expect(err).ToNot(HaveOccurred())

				return resourceConfigCheckSession
			}

			var resourceConfigCheckSession db.ResourceConfigCheckSession

			BeforeEach(func() {
				By("creating the session")
				resourceConfigCheckSession = findOrCreateSessionForDefaultResource()

				By("mapping the resource to the session's config")
				Expect(defaultResource.SetResourceConfig(resourceConfigCheckSession.ResourceConfig().ID())).To(Succeed())
			})

			It("keeps check sessions for active resources", func() {
				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newResourceConfigCheckSession := findOrCreateSessionForDefaultResource()

				By("finding the same session as before")
				Expect(newResourceConfigCheckSession.ID()).To(Equal(resourceConfigCheckSession.ID()))
			})

			It("removes check sessions for inactive resources", func() {
				By("removing the default resource from the pipeline config")
				_, _, err := defaultTeam.SavePipeline("default-pipeline", atc.Config{
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
				}, defaultPipeline.ConfigVersion(), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newResourceConfigCheckSession := findOrCreateSessionForDefaultResource()

				By("having created a new session, as the old one was removed")
				Expect(newResourceConfigCheckSession.ID()).ToNot(Equal(resourceConfigCheckSession.ID()))
			})

			It("removes check sessions for paused resources", func() {
				By("pausing the resource")
				Expect(defaultResource.Pause()).To(Succeed())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newResourceConfigCheckSession := findOrCreateSessionForDefaultResource()

				By("having created a new session, as the old one was removed")
				Expect(newResourceConfigCheckSession.ID()).ToNot(Equal(resourceConfigCheckSession.ID()))
			})

			It("removes check sessions for resources in paused pipelines", func() {
				By("pausing the pipeline")
				Expect(defaultPipeline.Pause()).To(Succeed())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newResourceConfigCheckSession := findOrCreateSessionForDefaultResource()

				By("having created a new session, as the old one was removed")
				Expect(newResourceConfigCheckSession.ID()).ToNot(Equal(resourceConfigCheckSession.ID()))
			})
		})

		Context("for resource types", func() {
			findOrCreateSessionForDefaultResourceType := func() db.ResourceConfigCheckSession {
				resourceConfigCheckSession, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(logger,
					defaultResourceType.Type(),
					defaultResourceType.Source(),
					creds.VersionedResourceTypes{},
					expiry,
				)
				Expect(err).ToNot(HaveOccurred())

				return resourceConfigCheckSession
			}

			var resourceConfigCheckSession db.ResourceConfigCheckSession

			BeforeEach(func() {
				By("creating the session")
				resourceConfigCheckSession = findOrCreateSessionForDefaultResourceType()

				By("mapping the resource to the session's config")
				Expect(defaultResourceType.SetResourceConfig(resourceConfigCheckSession.ResourceConfig().ID())).To(Succeed())
			})

			It("keeps check sessions for active resource types", func() {
				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newResourceConfigCheckSession := findOrCreateSessionForDefaultResourceType()

				By("finding the same session as before")
				Expect(newResourceConfigCheckSession.ID()).To(Equal(resourceConfigCheckSession.ID()))
			})

			It("removes check sessions for inactive resource types", func() {
				By("removing the default resource from the pipeline config")
				_, _, err := defaultTeam.SavePipeline("default-pipeline", atc.Config{
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
				}, defaultPipeline.ConfigVersion(), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newResourceConfigCheckSession := findOrCreateSessionForDefaultResourceType()

				By("having created a new session, as the old one was removed")
				Expect(newResourceConfigCheckSession.ID()).ToNot(Equal(resourceConfigCheckSession.ID()))
			})

			It("removes check sessions for resource types in paused pipelines", func() {
				By("pausing the pipeline")
				Expect(defaultPipeline.Pause()).To(Succeed())

				By("cleaning up inactive sessions")
				Expect(lifecycle.CleanInactiveResourceConfigCheckSessions()).To(Succeed())

				By("find-or-creating the session again")
				newResourceConfigCheckSession := findOrCreateSessionForDefaultResourceType()

				By("having created a new session, as the old one was removed")
				Expect(newResourceConfigCheckSession.ID()).ToNot(Equal(resourceConfigCheckSession.ID()))
			})
		})
	})
})
