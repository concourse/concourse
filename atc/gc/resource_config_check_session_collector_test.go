package gc_test

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCheckSessionCollector", func() {
	var (
		collector                           GcCollector
		resourceConfigCheckSessionLifecycle db.ResourceConfigCheckSessionLifecycle
		resourceConfigScope                 db.ResourceConfigScope
		ownerExpiries                       db.ContainerOwnerExpiries
		resource                            db.Resource
	)

	BeforeEach(func() {
		resourceConfigCheckSessionLifecycle = db.NewResourceConfigCheckSessionLifecycle(dbConn)
		resourceConfigFactory = db.NewResourceConfigFactory(dbConn, lockFactory)
		collector = gc.NewResourceConfigCheckSessionCollector(resourceConfigCheckSessionLifecycle)
	})

	Describe("Run", func() {
		var runErr error
		var owner db.ContainerOwner

		ownerExpiries = db.ContainerOwnerExpiries{
			Min: 10 * time.Second,
			Max: 10 * time.Second,
		}

		BeforeEach(func() {
			var err error
			var found bool
			resource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			resourceConfigScope, err = resource.SetResourceConfig(
				atc.Source{
					"some": "source",
				},
				atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			resourceConfig := resourceConfigScope.ResourceConfig()
			owner = db.NewResourceConfigCheckSessionContainerOwner(resourceConfig.ID(), resourceConfig.OriginBaseResourceType().ID, ownerExpiries)

			workerFactory := db.NewWorkerFactory(dbConn)
			defaultWorkerPayload := atc.Worker{
				ResourceTypes: []atc.WorkerResourceType{
					atc.WorkerResourceType{
						Type:    "some-base-type",
						Image:   "/path/to/image",
						Version: "some-brt-version",
					},
				},
				Name:            "default-worker",
				GardenAddr:      "1.2.3.4:7777",
				BaggageclaimURL: "5.6.7.8:7878",
			}
			worker, err := workerFactory.SaveWorker(defaultWorkerPayload, 0)
			Expect(err).NotTo(HaveOccurred())

			_, err = worker.CreateContainer(owner, db.ContainerMetadata{})
			Expect(err).NotTo(HaveOccurred())
		})

		resourceConfigCheckSessionExists := func(resourceConfig db.ResourceConfig) bool {
			var count int
			err = psql.Select("COUNT(*)").
				From("resource_config_check_sessions").
				Where(sq.Eq{"resource_config_id": resourceConfig.ID()}).
				RunWith(dbConn).
				QueryRow().
				Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			return count == 1
		}

		JustBeforeEach(func() {
			runErr = collector.Run(context.TODO())
			Expect(runErr).ToNot(HaveOccurred())
		})

		Context("when the resource config check session is expired", func() {
			BeforeEach(func() {
				time.Sleep(ownerExpiries.Max)
			})

			It("removes the resource config check session", func() {
				Expect(resourceConfigCheckSessionExists(resourceConfigScope.ResourceConfig())).To(BeFalse())
			})
		})

		Context("when the resource config check session is not expired", func() {
			It("keeps the resource config check session", func() {
				Expect(resourceConfigCheckSessionExists(resourceConfigScope.ResourceConfig())).To(BeTrue())
			})
		})

		Context("when the resource is active", func() {
			It("keeps the resource config check session", func() {
				Expect(resourceConfigCheckSessionExists(resourceConfigScope.ResourceConfig())).To(BeTrue())
			})
		})

		Context("when the resource is unactive", func() {
			BeforeEach(func() {
				atcConfig := atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-job",
						},
						{
							Name: "some-other-job",
						},
					},
				}

				defaultPipeline, _, err = defaultTeam.SavePipeline(defaultPipelineRef, atcConfig, db.ConfigVersion(1), false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes the resource config check session", func() {
				Expect(resourceConfigCheckSessionExists(resourceConfigScope.ResourceConfig())).To(BeFalse())
			})
		})
	})
})
