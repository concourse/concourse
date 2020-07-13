package db_test

import (
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource Config Scope", func() {
	var resourceScope db.ResourceConfigScope
	var resource db.Resource
	var pipeline db.Pipeline

	BeforeEach(func() {
		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := db.BaseResourceType{
			Name: "some-type",
		}

		_, err = brt.FindOrCreate(setupTx, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		pipeline, _, err = defaultTeam.SavePipeline(atc.PipelineRef{Name: "scope-pipeline"}, atc.Config{
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"some": "source",
					},
				},
			},
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
					PlanSequence: []atc.Step{
						{
							Config: &atc.GetStep{
								Name: "some-resource",
							},
						},
					},
				},
				{
					Name: "downstream-job",
					PlanSequence: []atc.Step{
						{
							Config: &atc.GetStep{
								Name:   "some-resource",
								Passed: []string{"some-job"},
							},
						},
					},
				},
				{
					Name: "some-other-job",
				},
			},
		}, db.ConfigVersion(0), false)
		Expect(err).NotTo(HaveOccurred())

		var found bool
		resource, found, err = pipeline.Resource("some-resource")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("SaveVersions", func() {
		var (
			originalVersionSlice []atc.Version
		)

		BeforeEach(func() {
			originalVersionSlice = []atc.Version{
				{"ref": "v1"},
				{"ref": "v3"},
			}
		})

		// XXX: Can make test more resilient if there is a method that gives all versions by descending check order
		It("ensures versioned resources have the correct check_order", func() {
			err := resourceScope.SaveVersions(nil, originalVersionSlice)
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err := resourceScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version()).To(Equal(db.Version{"ref": "v3"}))
			Expect(latestVR.CheckOrder()).To(Equal(2))

			pretendCheckResults := []atc.Version{
				{"ref": "v2"},
				{"ref": "v3"},
			}

			err = resourceScope.SaveVersions(nil, pretendCheckResults)
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err = resourceScope.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version()).To(Equal(db.Version{"ref": "v3"}))
			Expect(latestVR.CheckOrder()).To(Equal(4))
		})

		Context("when the versions already exists", func() {
			var newVersionSlice []atc.Version

			BeforeEach(func() {
				newVersionSlice = []atc.Version{
					{"ref": "v1"},
					{"ref": "v3"},
				}

				err := resourceScope.SaveVersions(nil, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				latestVR, found, err := resourceScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(latestVR.Version()).To(Equal(db.Version{"ref": "v3"}))
				Expect(latestVR.CheckOrder()).To(Equal(2))
			})

			It("does not change the check order", func() {
				err := resourceScope.SaveVersions(nil, newVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				latestVR, found, err := resourceScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(latestVR.Version()).To(Equal(db.Version{"ref": "v3"}))
				Expect(latestVR.CheckOrder()).To(Equal(2))
			})

			Context("when a new version is added", func() {
				It("requests schedule on the jobs that use the resource", func() {
					err := resourceScope.SaveVersions(nil, originalVersionSlice)
					Expect(err).ToNot(HaveOccurred())

					job, found, err := pipeline.Job("some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					requestedSchedule := job.ScheduleRequestedTime()

					newVersions := []atc.Version{
						{"ref": "v0"},
						{"ref": "v3"},
					}
					err = resourceScope.SaveVersions(nil, newVersions)
					Expect(err).ToNot(HaveOccurred())

					found, err = job.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
				})

				It("does not request schedule on the jobs that use the resource but through passed constraints", func() {
					err := resourceScope.SaveVersions(nil, originalVersionSlice)
					Expect(err).ToNot(HaveOccurred())

					job, found, err := pipeline.Job("downstream-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					requestedSchedule := job.ScheduleRequestedTime()

					newVersions := []atc.Version{
						{"ref": "v0"},
						{"ref": "v3"},
					}
					err = resourceScope.SaveVersions(nil, newVersions)
					Expect(err).ToNot(HaveOccurred())

					found, err = job.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
				})

				It("does not request schedule on the jobs that do not use the resource", func() {
					err := resourceScope.SaveVersions(nil, originalVersionSlice)
					Expect(err).ToNot(HaveOccurred())

					job, found, err := pipeline.Job("some-other-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					requestedSchedule := job.ScheduleRequestedTime()

					newVersions := []atc.Version{
						{"ref": "v0"},
						{"ref": "v3"},
					}
					err = resourceScope.SaveVersions(nil, newVersions)
					Expect(err).ToNot(HaveOccurred())

					found, err = job.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
				})
			})
		})
	})

	Describe("LatestVersion", func() {
		Context("when the resource config exists", func() {
			var latestCV db.ResourceConfigVersion

			BeforeEach(func() {
				originalVersionSlice := []atc.Version{
					{"ref": "v1"},
					{"ref": "v3"},
				}

				err := resourceScope.SaveVersions(nil, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				latestCV, found, err = resourceScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("gets latest version of resource", func() {
				Expect(latestCV.Version()).To(Equal(db.Version{"ref": "v3"}))
				Expect(latestCV.CheckOrder()).To(Equal(2))
			})

			It("disabled versions do not affect fetching the latest version", func() {
				err := resourceScope.SaveVersions(nil, []atc.Version{{"version": "1"}})
				Expect(err).ToNot(HaveOccurred())

				savedRCV, found, err := resourceScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedRCV.Version()).To(Equal(db.Version{"version": "1"}))

				err = resource.DisableVersion(savedRCV.ID())
				Expect(err).ToNot(HaveOccurred())

				latestVR, found, err := resourceScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Version()).To(Equal(db.Version{"version": "1"}))

				err = resource.EnableVersion(savedRCV.ID())
				Expect(err).ToNot(HaveOccurred())

				latestVR, found, err = resourceScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(latestVR.Version()).To(Equal(db.Version{"version": "1"}))
			})

			It("saving versioned resources updates the latest versioned resource", func() {
				err := resourceScope.SaveVersions(nil, []atc.Version{{"ref": "4"}, {"ref": "5"}})
				Expect(err).ToNot(HaveOccurred())

				savedVR, found, err := resourceScope.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(savedVR.Version()).To(Equal(db.Version{"ref": "5"}))
			})
		})
	})

	Describe("FindVersion", func() {
		BeforeEach(func() {
			originalVersionSlice := []atc.Version{
				{"ref": "v1"},
				{"ref": "v3"},
			}

			err := resourceScope.SaveVersions(nil, originalVersionSlice)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the version exists", func() {
			var latestCV db.ResourceConfigVersion

			BeforeEach(func() {
				var err error
				var found bool
				latestCV, found, err = resourceScope.FindVersion(atc.Version{"ref": "v1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("gets the version of resource", func() {
				Expect(latestCV.ResourceConfigScope().ID()).To(Equal(resourceScope.ID()))
				Expect(latestCV.Version()).To(Equal(db.Version{"ref": "v1"}))
				Expect(latestCV.CheckOrder()).To(Equal(1))
			})
		})

		Context("when the version does not exist", func() {
			var found bool
			var latestCV db.ResourceConfigVersion

			BeforeEach(func() {
				var err error
				latestCV, found, err = resourceScope.FindVersion(atc.Version{"ref": "v2"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not get the version of resource", func() {
				Expect(found).To(BeFalse())
				Expect(latestCV).To(BeNil())
			})
		})
	})

	Describe("UpdateLastCheckStartTime", func() {
		var (
			someResource        db.Resource
			resourceConfigScope db.ResourceConfigScope
		)

		BeforeEach(func() {
			var err error
			var found bool

			someResource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = someResource.SetResourceConfig(
				someResource.Source(),
				pipelineResourceTypes.Deserialize(),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there has not been a check", func() {
			It("should update the last checked", func() {
				updated, err := resourceConfigScope.UpdateLastCheckStartTime(1*time.Second, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())
			})

			Context("when immediate", func() {
				It("should update the last checked", func() {
					updated, err := resourceConfigScope.UpdateLastCheckStartTime(1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})
			})
		})

		Context("when there has been a check recently", func() {
			interval := 1 * time.Second

			BeforeEach(func() {
				updated, err := resourceConfigScope.UpdateLastCheckStartTime(interval, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())
			})

			Context("when not immediate", func() {
				It("does not update the last checked until the interval has elapsed", func() {
					updated, err := resourceConfigScope.UpdateLastCheckStartTime(interval, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeFalse())
				})

				Context("when the interval has elapsed", func() {
					BeforeEach(func() {
						time.Sleep(interval)
					})

					It("updates the last checked", func() {
						updated, err := resourceConfigScope.UpdateLastCheckStartTime(interval, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(updated).To(BeTrue())
					})
				})
			})

			Context("when it is immediate", func() {
				It("updates the last checked", func() {
					updated, err := resourceConfigScope.UpdateLastCheckStartTime(1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})
			})
		})
	})

	Describe("UpdateLastCheckEndTime", func() {
		var (
			someResource        db.Resource
			resourceConfigScope db.ResourceConfigScope
		)

		BeforeEach(func() {
			var err error
			var found bool

			someResource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = someResource.SetResourceConfig(
				someResource.Source(),
				pipelineResourceTypes.Deserialize(),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should update last check finished", func() {
			updated, err := resourceConfigScope.UpdateLastCheckEndTime()
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeTrue())

			someResource.Reload()
			Expect(someResource.LastCheckEndTime()).To(BeTemporally("~", time.Now(), 100*time.Millisecond))
		})
	})

	Describe("AcquireResourceCheckingLock", func() {
		var (
			someResource        db.Resource
			resourceConfigScope db.ResourceConfigScope
		)

		BeforeEach(func() {
			var err error
			var found bool

			someResource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = someResource.SetResourceConfig(
				someResource.Source(),
				pipelineResourceTypes.Deserialize(),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there has been a check recently", func() {
			var lock lock.Lock
			var err error

			BeforeEach(func() {
				var err error
				var acquired bool
				lock, acquired, err = resourceConfigScope.AcquireResourceCheckingLock(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())
			})

			AfterEach(func() {
				_ = lock.Release()
			})

			It("does not get the lock", func() {
				_, acquired, err := resourceConfigScope.AcquireResourceCheckingLock(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeFalse())
			})

			Context("and the lock gets released", func() {
				BeforeEach(func() {
					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})

				It("gets the lock", func() {
					lock, acquired, err := resourceConfigScope.AcquireResourceCheckingLock(logger)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			It("gets and keeps the lock and stops others from periodically getting it", func() {
				lock, acquired, err := resourceConfigScope.AcquireResourceCheckingLock(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())

				Consistently(func() bool {
					_, acquired, err = resourceConfigScope.AcquireResourceCheckingLock(logger)
					Expect(err).ToNot(HaveOccurred())

					return acquired
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				err = lock.Release()
				Expect(err).ToNot(HaveOccurred())

				time.Sleep(time.Second)

				lock, acquired, err = resourceConfigScope.AcquireResourceCheckingLock(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())

				err = lock.Release()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
