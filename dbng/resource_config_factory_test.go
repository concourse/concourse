package dbng_test

import (
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigFactory", func() {
	var build dbng.Build

	BeforeEach(func() {
		var err error
		build, err = defaultPipeline.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("FindOrCreateResourceConfig", func() {
		It("returns finds resource config", func() {
			resourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).NotTo(HaveOccurred())

			usedResourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
				logger,
				dbng.ForBuild(build.ID()),
				"some-type",
				atc.Source{"a": "b"},
				resourceTypes.Deserialize(),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(usedResourceConfig).NotTo(BeNil())
			Expect(usedResourceConfig.CreatedByResourceCache).NotTo(BeNil())
		})

		Context("when provided custom resource type does not have a version", func() {
			It("returns an error", func() {
				_, err := resourceConfigFactory.FindOrCreateResourceConfig(
					logger,
					dbng.ForBuild(build.ID()),
					"some-type",
					atc.Source{"a": "b"},
					[]atc.VersionedResourceType{
						{
							ResourceType: atc.ResourceType{
								Name: "some-type",
								Type: "some-base-resource-type",
								Source: atc.Source{
									"some-type": "source",
								},
							},
						},
					},
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ustom resource type 'some-type' version not found"))
			})
		})
	})

	Describe("AcquireResourceCheckingLock", func() {
		It("acquires only one lock when running in parallel", func() {
			start := make(chan struct{})
			wg := sync.WaitGroup{}

			resourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).NotTo(HaveOccurred())

			acquiredLocks := []lock.Lock{}
			var l sync.RWMutex

			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					<-start
					lock, acquired, err := resourceConfigFactory.AcquireResourceCheckingLock(
						logger,
						dbng.ForBuild(build.ID()),
						"some-type",
						atc.Source{"a": "b"},
						resourceTypes.Deserialize(),
					)
					Expect(err).NotTo(HaveOccurred())
					if acquired {
						l.Lock()
						acquiredLocks = append(acquiredLocks, lock)
						l.Unlock()
					}

					wg.Done()
				}()
			}

			close(start)
			wg.Wait()

			l.RLock()
			defer l.RUnlock()
			Expect(acquiredLocks).To(HaveLen(1))
		})
	})

	DescribeTable("CleanConfigUsesForFinishedBuilds",
		func(i bool, diff int) {
			err := build.SetInterceptible(i)
			Expect(err).NotTo(HaveOccurred())

			_, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, dbng.ForBuild(build.ID()), "some-base-resource-type", atc.Source{}, atc.VersionedResourceTypes{})
			Expect(err).NotTo(HaveOccurred())

			var (
				rcuCountBefore int
				rcuCountAfter  int
			)

			dbConn.QueryRow("select count(*) from resource_config_uses").Scan(&rcuCountBefore)

			resourceConfigFactory.CleanConfigUsesForFinishedBuilds()
			Expect(err).NotTo(HaveOccurred())

			dbConn.QueryRow("select count(*) from resource_config_uses").Scan(&rcuCountAfter)

			Expect(rcuCountBefore - rcuCountAfter).To(Equal(diff))
		},
		Entry("non-interceptible builds are deleted", false, 1),
		Entry("interceptible builds are not deleted", true, 0),
	)

	Context("when the user no longer exists", func() {
		BeforeEach(func() {
			Expect(defaultPipeline.Destroy()).To(Succeed())
		})

		It("returns UserDisappearedError", func() {
			user := dbng.ForBuild(build.ID())

			_, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, user, "some-base-resource-type", atc.Source{}, atc.VersionedResourceTypes{})
			Expect(err).To(Equal(dbng.UserDisappearedError{user}))
			Expect(err.Error()).To(Equal("resource user disappeared: build #1"))
		})
	})

	Context("when the resource config is concurrently deleted and created", func() {
		BeforeEach(func() {
			Expect(build.Finish(dbng.BuildStatusSucceeded)).To(Succeed())
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
						Expect(resourceConfigFactory.CleanConfigUsesForFinishedBuilds()).To(Succeed())
						Expect(resourceConfigFactory.CleanUselessConfigs()).To(Succeed())
					}
				}
			}()

			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer close(done)
				defer wg.Done()

				for i := 0; i < 100; i++ {
					_, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, dbng.ForBuild(build.ID()), "some-base-resource-type", atc.Source{"some": "unique-source"}, atc.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())
				}
			}()

			wg.Wait()
		})
	})
})
