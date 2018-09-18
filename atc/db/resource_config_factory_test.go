package db_test

import (
	"sync"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigFactory", func() {
	var build db.Build

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

		ownerExpiries := db.ContainerOwnerExpiries{
			GraceTime: 5 * time.Second,
			Min:       10 * time.Second,
			Max:       10 * time.Second,
		}

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
					_, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(logger, "some-base-resource-type", atc.Source{"some": "unique-source"}, creds.VersionedResourceTypes{}, ownerExpiries)
					Expect(err).ToNot(HaveOccurred())
				}
			}()

			wg.Wait()
		})
	})
})
