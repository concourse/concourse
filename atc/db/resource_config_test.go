package db_test

import (
	"time"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfig", func() {
	var resourceConfig db.ResourceConfig
	var resource db.Resource

	BeforeEach(func() {
		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := db.BaseResourceType{
			Name: "some-type",
		}

		_, err = brt.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		pipeline, _, err := defaultTeam.SavePipeline("some-pipeline", atc.Config{
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"some": "source",
					},
				},
			},
		}, db.ConfigVersion(0), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		var found bool
		resource, found, err = pipeline.Resource("some-resource")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		resourceConfig, err = resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("LatestVersions", func() {
		var (
			latestCV  []db.ResourceVersion
			latestErr error
		)

		JustBeforeEach(func() {
			latestCV, latestErr = resourceConfig.LatestVersions()
		})

		Context("when the version exists", func() {
			BeforeEach(func() {
				saveVersions(resourceConfig, []atc.SpaceVersion{
					atc.SpaceVersion{
						Version: atc.Version{"ref": "v1"},
						Space:   atc.Space("space"),
					},
					atc.SpaceVersion{
						Version: atc.Version{"ref": "v3"},
						Space:   atc.Space("space"),
					},
				})
			})

			Context("when the space has a latest version set", func() {
				BeforeEach(func() {
					err := resourceConfig.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"ref": "v3"})
					Expect(err).ToNot(HaveOccurred())
				})

				It("gets latest version of resource", func() {
					Expect(latestErr).ToNot(HaveOccurred())

					Expect(latestCV).To(HaveLen(1))
					Expect(latestCV[0].Version()).To(Equal(db.Version{"ref": "v3"}))
					Expect(latestCV[0].CheckOrder()).To(Equal(2))
				})
			})

			Context("when the space does not have the latest version set", func() {
				It("returns nil", func() {
					Expect(latestErr).ToNot(HaveOccurred())
					Expect(latestCV).To(Equal([]db.ResourceVersion{}))
				})
			})
		})

		Context("when the version does not exist", func() {
			It("returns nil", func() {
				Expect(latestErr).ToNot(HaveOccurred())
				Expect(latestCV).To(Equal([]db.ResourceVersion{}))
			})
		})
	})

	Describe("FindVersion", func() {
		var (
			latestCV db.ResourceVersion
			found    bool
			findErr  error

			findVersion atc.Version
			findSpace   atc.Space
		)

		BeforeEach(func() {
			saveVersions(resourceConfig, []atc.SpaceVersion{
				atc.SpaceVersion{
					Version: atc.Version{"ref": "v1"},
					Space:   atc.Space("space"),
					Metadata: atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					},
				},
				atc.SpaceVersion{
					Version: atc.Version{"ref": "v3"},
					Space:   atc.Space("space"),
					Metadata: atc.Metadata{
						atc.MetadataField{
							Name:  "other",
							Value: "metadata",
						},
					},
				},
			})
		})

		JustBeforeEach(func() {
			latestCV, found, findErr = resourceConfig.FindVersion(findSpace, findVersion)
		})

		Context("when the version exists", func() {
			BeforeEach(func() {
				findSpace = atc.Space("space")
				findVersion = atc.Version{"ref": "v1"}
			})

			It("gets the version of resource", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(latestCV.ResourceConfig().ID()).To(Equal(resourceConfig.ID()))
				Expect(latestCV.Version()).To(Equal(db.Version{"ref": "v1"}))
				Expect(latestCV.Space()).To(Equal(atc.Space("space")))
				Expect(latestCV.Partial()).To(BeFalse())
				Expect(latestCV.Metadata()).To(Equal(db.ResourceConfigMetadataFields{{Name: "some", Value: "metadata"}}))
				Expect(latestCV.CheckOrder()).To(Equal(1))
			})
		})

		Context("when the version does not exist", func() {
			BeforeEach(func() {
				findSpace = atc.Space("space")
				findVersion = atc.Version{"ref": "v2"}
			})

			It("does not get the version of resource", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the space does not exist", func() {
			BeforeEach(func() {
				findSpace = atc.Space("non-existant")
				findVersion = atc.Version{"ref": "v2"}
			})

			It("does not get the version of resource", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the version is partial", func() {
			BeforeEach(func() {
				err := resourceConfig.SavePartialVersion(atc.Space("space"), atc.Version{"ref": "partial"}, nil)
				Expect(err).ToNot(HaveOccurred())

				findSpace = atc.Space("space")
				findVersion = atc.Version{"ref": "partial"}
			})

			It("does not get the version of resource", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("SaveDefaultSpace", func() {
		var (
			defaultSpace    string
			defaultSpaceErr error
		)

		JustBeforeEach(func() {
			defaultSpaceErr = resourceConfig.SaveDefaultSpace(atc.Space(defaultSpace))
		})

		Context("when the space exists", func() {
			BeforeEach(func() {
				err := resourceConfig.SaveSpace(atc.Space("space"))
				Expect(err).ToNot(HaveOccurred())

				defaultSpace = "space"
			})

			It("saves the default space", func() {
				Expect(defaultSpaceErr).ToNot(HaveOccurred())

				resourceConfig, err := resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())
				Expect(resourceConfig.DefaultSpace()).To(Equal(atc.Space("space")))
			})
		})

		// XXX: Is this supposed to save even if it doesn't exist???
		Context("when the space does not exist", func() {
			BeforeEach(func() {
				defaultSpace = "space"
			})

			It("still saves the default space", func() {
				Expect(defaultSpaceErr).ToNot(HaveOccurred())

				resourceConfig, err := resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())
				Expect(resourceConfig.DefaultSpace()).To(Equal(atc.Space("space")))
			})
		})
	})

	Describe("SavePartialVersion/SaveSpace", func() {
		Context("when the space exists", func() {
			BeforeEach(func() {
				err := resourceConfig.SaveSpace(atc.Space("space"))
				Expect(err).ToNot(HaveOccurred())
			})

			It("saves the partial version successfully", func() {
				err := resourceConfig.SavePartialVersion(atc.Space("space"), atc.Version{"some": "version"}, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the version is partial", func() {
				BeforeEach(func() {
					err := resourceConfig.SavePartialVersion(atc.Space("space"), atc.Version{"some": "version"}, nil)
					Expect(err).ToNot(HaveOccurred())
				})

				It("is not found", func() {
					version, found, err := resourceConfig.FindVersion(atc.Space("space"), atc.Version{"some": "version"})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeFalse())
					Expect(version).To(BeNil())
				})
			})

			Context("when the version is finished saving", func() {
				BeforeEach(func() {
					err := resourceConfig.SavePartialVersion(atc.Space("space"), atc.Version{"some": "version"}, nil)
					Expect(err).ToNot(HaveOccurred())

					err = resourceConfig.FinishSavingVersions()
					Expect(err).ToNot(HaveOccurred())

					err = resourceConfig.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"some": "version"})
					Expect(err).ToNot(HaveOccurred())
				})

				It("is found", func() {
					version, found, err := resourceConfig.FindVersion(atc.Space("space"), atc.Version{"some": "version"})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(version.Space()).To(Equal(atc.Space("space")))
					Expect(version.Version()).To(Equal(db.Version{"some": "version"}))
					Expect(version.Partial()).To(BeFalse())

					latestVR, err := resourceConfig.LatestVersions()
					Expect(err).ToNot(HaveOccurred())
					Expect(latestVR).ToNot(BeEmpty())
					Expect(latestVR[0].Version()).To(Equal(db.Version{"some": "version"}))
					Expect(latestVR[0].CheckOrder()).To(Equal(1))
				})
			})
		})

		Context("when saving multiple versions", func() {
			It("ensures versions have the correct check_order", func() {
				originalVersionSlice := []atc.SpaceVersion{
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v1"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v3"},
					},
				}

				saveVersions(resourceConfig, originalVersionSlice)

				err := resourceConfig.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"ref": "v3"})
				Expect(err).ToNot(HaveOccurred())

				latestVR, err := resourceConfig.LatestVersions()
				Expect(err).ToNot(HaveOccurred())
				Expect(latestVR[0].Version()).To(Equal(db.Version{"ref": "v3"}))
				Expect(latestVR[0].CheckOrder()).To(Equal(2))

				pretendCheckResults := []atc.SpaceVersion{
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v2"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v3"},
					},
				}

				saveVersions(resourceConfig, pretendCheckResults)

				err = resourceConfig.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"ref": "v3"})
				Expect(err).ToNot(HaveOccurred())

				latestVR, err = resourceConfig.LatestVersions()
				Expect(err).ToNot(HaveOccurred())
				Expect(latestVR[0].Version()).To(Equal(db.Version{"ref": "v3"}))
				Expect(latestVR[0].CheckOrder()).To(Equal(4))
			})
		})

		Context("when the versions already exists", func() {
			var newVersionSlice []atc.SpaceVersion

			BeforeEach(func() {
				newVersionSlice = []atc.SpaceVersion{
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v1"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v3"},
					},
				}
			})

			It("does not change the check order", func() {
				saveVersions(resourceConfig, newVersionSlice)

				err := resourceConfig.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"ref": "v3"})
				Expect(err).ToNot(HaveOccurred())

				latestVR, err := resourceConfig.LatestVersions()
				Expect(err).ToNot(HaveOccurred())

				Expect(latestVR[0].Version()).To(Equal(db.Version{"ref": "v3"}))
				Expect(latestVR[0].CheckOrder()).To(Equal(2))
			})
		})
	})

	Describe("SaveSpaceLatestVersion/LatestVersions", func() {
		var (
			spaceVersion  atc.SpaceVersion
			spaceVersion2 atc.SpaceVersion
		)

		BeforeEach(func() {
			err := resourceConfig.SaveSpace(atc.Space("space"))
			Expect(err).ToNot(HaveOccurred())

			otherSpaceVersion := atc.SpaceVersion{
				Space:   "space",
				Version: atc.Version{"some": "other-version"},
				Metadata: atc.Metadata{
					atc.MetadataField{
						Name:  "some",
						Value: "metadata",
					},
				},
			}

			spaceVersion = atc.SpaceVersion{
				Space:   "space",
				Version: atc.Version{"some": "version"},
				Metadata: atc.Metadata{
					atc.MetadataField{
						Name:  "some",
						Value: "metadata",
					},
				},
			}

			saveVersions(resourceConfig, []atc.SpaceVersion{otherSpaceVersion, spaceVersion})

			err = resourceConfig.SaveSpace(atc.Space("space2"))
			Expect(err).ToNot(HaveOccurred())

			spaceVersion2 = atc.SpaceVersion{
				Space:   "space2",
				Version: atc.Version{"some": "version2"},
				Metadata: atc.Metadata{
					atc.MetadataField{
						Name:  "some",
						Value: "metadata",
					},
				},
			}

			saveVersions(resourceConfig, []atc.SpaceVersion{spaceVersion2})
		})

		Context("when the version exists", func() {
			BeforeEach(func() {
				err := resourceConfig.SaveSpaceLatestVersion(spaceVersion.Space, spaceVersion.Version)
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfig.SaveSpaceLatestVersion(spaceVersion2.Space, spaceVersion2.Version)
				Expect(err).ToNot(HaveOccurred())
			})

			It("saves the version into the space", func() {
				latestVersions, latestErr := resourceConfig.LatestVersions()
				Expect(latestErr).ToNot(HaveOccurred())
				Expect(latestVersions).To(HaveLen(2))
				Expect(latestVersions[0].Version()).To(Equal(db.Version(spaceVersion.Version)))
				Expect(latestVersions[1].Version()).To(Equal(db.Version(spaceVersion2.Version)))
			})
		})

		Context("when the version does not exist", func() {
			It("does not save the version into the space", func() {
				err := resourceConfig.SaveSpaceLatestVersion(spaceVersion.Space, atc.Version{"non-existant": "version"})
				Expect(err).ToNot(HaveOccurred())

				latestVersions, latestErr := resourceConfig.LatestVersions()
				Expect(latestErr).ToNot(HaveOccurred())
				Expect(latestVersions).To(HaveLen(0))
			})
		})

		Context("when the space does not exist", func() {
			It("does not save the version into the space", func() {
				err := resourceConfig.SaveSpaceLatestVersion(atc.Space("non-existant-space"), atc.Version{"some": "version"})
				Expect(err).ToNot(HaveOccurred())

				latestVersions, latestErr := resourceConfig.LatestVersions()
				Expect(latestErr).ToNot(HaveOccurred())
				Expect(latestVersions).To(HaveLen(0))
			})
		})
	})

	Describe("UpdateLastChecked", func() {
		var (
			someResource   db.Resource
			resourceConfig db.ResourceConfig
		)

		BeforeEach(func() {
			var err error
			var found bool

			someResource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			vrt, err := pipelineResourceTypes.Deserialize()
			Expect(err).ToNot(HaveOccurred())

			resourceConfig, err = someResource.SetResourceConfig(
				logger,
				someResource.Source(),
				creds.NewVersionedResourceTypes(template.StaticVariables{}, vrt),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there has not been a check", func() {
			It("should update the last checked", func() {
				updated, err := resourceConfig.UpdateLastChecked(1*time.Second, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())
			})

			Context("when immediate", func() {
				It("should update the last checked", func() {
					updated, err := resourceConfig.UpdateLastChecked(1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})
			})
		})

		Context("when there has been a check recently", func() {
			interval := 1 * time.Second

			BeforeEach(func() {
				updated, err := resourceConfig.UpdateLastChecked(interval, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())
			})

			Context("when not immediate", func() {
				It("does not update the last checked until the interval has elapsed", func() {
					updated, err := resourceConfig.UpdateLastChecked(interval, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeFalse())
				})

				Context("when the interval has elapsed", func() {
					BeforeEach(func() {
						time.Sleep(interval)
					})

					It("updates the last checked", func() {
						updated, err := resourceConfig.UpdateLastChecked(interval, false)
						Expect(err).ToNot(HaveOccurred())
						Expect(updated).To(BeTrue())
					})
				})
			})

			Context("when it is immediate", func() {
				It("updates the last checked", func() {
					updated, err := resourceConfig.UpdateLastChecked(1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})
			})
		})
	})

	Describe("UpdateLastCheckFinished", func() {
		var (
			someResource   db.Resource
			resourceConfig db.ResourceConfig
		)

		BeforeEach(func() {
			var err error
			var found bool

			someResource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			vrts, err := pipelineResourceTypes.Deserialize()
			Expect(err).ToNot(HaveOccurred())

			resourceConfig, err = someResource.SetResourceConfig(
				logger,
				someResource.Source(),
				creds.NewVersionedResourceTypes(template.StaticVariables{}, vrts),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should update last check finished", func() {
			updated, err := resourceConfig.UpdateLastCheckFinished()
			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeTrue())

			someResource.Reload()
			Expect(someResource.LastCheckFinished()).To(BeTemporally("~", time.Now(), 100*time.Millisecond))
		})
	})

	Describe("AcquireResourceCheckingLock", func() {
		var (
			someResource   db.Resource
			resourceConfig db.ResourceConfig
		)

		BeforeEach(func() {
			var err error
			var found bool

			someResource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			vrt, err := pipelineResourceTypes.Deserialize()
			Expect(err).ToNot(HaveOccurred())

			resourceConfig, err = someResource.SetResourceConfig(
				logger,
				someResource.Source(),
				creds.NewVersionedResourceTypes(template.StaticVariables{}, vrt),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there has been a check recently", func() {
			var lock lock.Lock
			var err error

			BeforeEach(func() {
				var err error
				var acquired bool
				lock, acquired, err = resourceConfig.AcquireResourceCheckingLock(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())
			})

			AfterEach(func() {
				_ = lock.Release()
			})

			It("does not get the lock", func() {
				_, acquired, err := resourceConfig.AcquireResourceCheckingLock(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeFalse())
			})

			Context("and the lock gets released", func() {
				BeforeEach(func() {
					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})

				It("gets the lock", func() {
					lock, acquired, err := resourceConfig.AcquireResourceCheckingLock(logger)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			It("gets and keeps the lock and stops others from periodically getting it", func() {
				lock, acquired, err := resourceConfig.AcquireResourceCheckingLock(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())

				Consistently(func() bool {
					_, acquired, err = resourceConfig.AcquireResourceCheckingLock(logger)
					Expect(err).ToNot(HaveOccurred())

					return acquired
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				err = lock.Release()
				Expect(err).ToNot(HaveOccurred())

				time.Sleep(time.Second)

				lock, acquired, err = resourceConfig.AcquireResourceCheckingLock(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())

				err = lock.Release()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})

func saveVersions(resourceConfig db.ResourceConfig, versions []atc.SpaceVersion) {
	for _, version := range versions {
		err := resourceConfig.SaveSpace(version.Space)
		Expect(err).ToNot(HaveOccurred())

		err = resourceConfig.SavePartialVersion(version.Space, version.Version, version.Metadata)
		Expect(err).ToNot(HaveOccurred())
	}

	err := resourceConfig.FinishSavingVersions()
	Expect(err).ToNot(HaveOccurred())
}
