package db_test

import (
	"sync"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigFactory", func() {
	var build db.Build

	Describe("CleanUnreferencedConfigs", func() {
		BeforeEach(func() {
			var err error
			job, found, err := defaultPipeline.Job("some-job")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			build, err = job.CreateBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the resource config is concurrently deleted and created", func() {
			BeforeEach(func() {
				Expect(build.Finish(db.BuildStatusSucceeded)).To(Succeed())
				Expect(build.SetInterceptible(false)).To(Succeed())
			})

			It("consistently is able to be used", func() {
				// enable concurrent use of database. this is set to 1 by default to
				// ensure methods don't require more than one in a single connection,
				// which can cause deadlocking as the pool is limited.
				dbConn.SetMaxOpenConns(2)

				done := make(chan struct{})

				wg := new(sync.WaitGroup)
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()

					for {
						select {
						case <-done:
							return
						default:
							Expect(resourceConfigFactory.CleanUnreferencedConfigs()).To(Succeed())
						}
					}
				}()

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer close(done)
					defer wg.Done()

					for i := 0; i < 100; i++ {
						_, err := resourceConfigFactory.FindOrCreateResourceConfig("some-base-resource-type", atc.Source{"some": "unique-source"}, atc.VersionedResourceTypes{})
						Expect(err).ToNot(HaveOccurred())
					}
				}()

				wg.Wait()
			})
		})
	})

	Describe("FindResourceConfigByID", func() {
		var (
			resourceConfigID      int
			resourceConfig        db.ResourceConfig
			createdResourceConfig db.ResourceConfig
			found                 bool
			err                   error
		)

		JustBeforeEach(func() {
			resourceConfig, found, err = resourceConfigFactory.FindResourceConfigByID(resourceConfigID)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the resource config does exist", func() {
			Context("when the resource config uses a base resource type", func() {
				BeforeEach(func() {
					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "base-resource-type-name",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					createdResourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig("base-resource-type-name", atc.Source{}, atc.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())
					Expect(createdResourceConfig).ToNot(BeNil())

					resourceConfigID = createdResourceConfig.ID()
				})

				It("should find the resource config using the resource's config id", func() {
					Expect(found).To(BeTrue())
					Expect(resourceConfig).ToNot(BeNil())
					Expect(resourceConfig.ID()).To(Equal(resourceConfigID))
					Expect(resourceConfig.CreatedByBaseResourceType()).To(Equal(createdResourceConfig.CreatedByBaseResourceType()))
				})
			})

			Context("when the resource config uses a custom resource type", func() {
				BeforeEach(func() {
					pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
					Expect(err).ToNot(HaveOccurred())

					createdResourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig("some-type", atc.Source{}, pipelineResourceTypes.Deserialize())
					Expect(err).ToNot(HaveOccurred())
					Expect(createdResourceConfig).ToNot(BeNil())

					resourceConfigID = createdResourceConfig.ID()
				})

				It("should find the resource config using the resource's config id", func() {
					Expect(found).To(BeTrue())
					Expect(resourceConfig).ToNot(BeNil())
					Expect(resourceConfig.ID()).To(Equal(resourceConfigID))
					Expect(resourceConfig.CreatedByResourceCache().ID()).To(Equal(createdResourceConfig.CreatedByResourceCache().ID()))
					Expect(resourceConfig.CreatedByResourceCache().ResourceConfig().ID()).To(Equal(createdResourceConfig.CreatedByResourceCache().ResourceConfig().ID()))
				})
			})
		})

		Context("when the resource config id does not exist", func() {
			BeforeEach(func() {
				resourceConfigID = 123
			})

			It("should not find the resource config", func() {
				Expect(found).To(BeFalse())
			})
		})
	})
})
