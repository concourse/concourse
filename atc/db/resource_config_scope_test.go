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

var _ = Describe("Resource Config Scope", func() {
	var resourceScope db.ResourceConfigScope
	var resource db.Resource

	BeforeEach(func() {
		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := db.BaseResourceType{
			Name: "some-type",
		}

		_, err = brt.FindOrCreate(setupTx, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		pipeline, _, err := defaultTeam.SavePipeline("scope-pipeline", atc.Config{
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

		resourceScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("LatestVersions", func() {
		var (
			latestCV  []db.ResourceVersion
			latestErr error
		)

		JustBeforeEach(func() {
			latestCV, latestErr = resourceScope.LatestVersions()
		})

		Context("when the version exists", func() {
			BeforeEach(func() {
				saveVersions(resourceScope, []atc.SpaceVersion{
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

			Context("when the scope has a latest version set", func() {
				BeforeEach(func() {
					err := resourceScope.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"ref": "v3"})
					Expect(err).ToNot(HaveOccurred())
				})

				It("gets latest version of resource", func() {
					Expect(latestErr).ToNot(HaveOccurred())

					Expect(latestCV).To(HaveLen(1))
					Expect(latestCV[0].Version()).To(Equal(db.Version{"ref": "v3"}))
					Expect(latestCV[0].CheckOrder()).To(Equal(2))
				})
			})

			Context("when the scope does not have the latest version set", func() {
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
			saveVersions(resourceScope, []atc.SpaceVersion{
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
			latestCV, found, findErr = resourceScope.FindVersion(findSpace, findVersion)
		})

		Context("when the version exists", func() {
			BeforeEach(func() {
				findSpace = atc.Space("space")
				findVersion = atc.Version{"ref": "v1"}
			})

			It("gets the version of resource", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(latestCV.ResourceConfigScope().ID()).To(Equal(resourceScope.ID()))
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
				err := resourceScope.SavePartialVersion(atc.Space("space"), atc.Version{"ref": "partial"}, nil)
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
			defaultSpaceErr = resourceScope.SaveDefaultSpace(atc.Space(defaultSpace))
		})

		Context("when the space exists", func() {
			BeforeEach(func() {
				err := resourceScope.SaveSpace(atc.Space("space"))
				Expect(err).ToNot(HaveOccurred())

				defaultSpace = "space"
			})

			It("saves the default space", func() {
				Expect(defaultSpaceErr).ToNot(HaveOccurred())

				resourceScope, err := resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())
				Expect(resourceScope.DefaultSpace()).To(Equal(atc.Space("space")))
			})
		})

		// XXX: Is this supposed to save even if it doesn't exist???
		Context("when the space does not exist", func() {
			BeforeEach(func() {
				defaultSpace = "space"
			})

			It("still saves the default space", func() {
				Expect(defaultSpaceErr).ToNot(HaveOccurred())

				resourceScope, err := resource.SetResourceConfig(logger, atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())
				Expect(resourceScope.DefaultSpace()).To(Equal(atc.Space("space")))
			})
		})
	})

	Describe("SavePartialVersion/SaveSpace", func() {
		Context("when the space exists", func() {
			BeforeEach(func() {
				err := resourceScope.SaveSpace(atc.Space("space"))
				Expect(err).ToNot(HaveOccurred())
			})

			It("saves the partial version successfully", func() {
				err := resourceScope.SavePartialVersion(atc.Space("space"), atc.Version{"some": "version"}, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the version is partial", func() {
				BeforeEach(func() {
					err := resourceScope.SavePartialVersion(atc.Space("space"), atc.Version{"some": "version"}, nil)
					Expect(err).ToNot(HaveOccurred())
				})

				It("is not found", func() {
					version, found, err := resourceScope.FindVersion(atc.Space("space"), atc.Version{"some": "version"})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeFalse())
					Expect(version).To(BeNil())
				})
			})

			Context("when the version is finished saving", func() {
				BeforeEach(func() {
					err := resourceScope.SavePartialVersion(atc.Space("space"), atc.Version{"some": "version"}, nil)
					Expect(err).ToNot(HaveOccurred())

					err = resourceScope.FinishSavingVersions()
					Expect(err).ToNot(HaveOccurred())

					err = resourceScope.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"some": "version"})
					Expect(err).ToNot(HaveOccurred())
				})

				It("is found", func() {
					version, found, err := resourceScope.FindVersion(atc.Space("space"), atc.Version{"some": "version"})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(version.Space()).To(Equal(atc.Space("space")))
					Expect(version.Version()).To(Equal(db.Version{"some": "version"}))
					Expect(version.Partial()).To(BeFalse())

					latestVR, err := resourceScope.LatestVersions()
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

				saveVersions(resourceScope, originalVersionSlice)

				err := resourceScope.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"ref": "v3"})
				Expect(err).ToNot(HaveOccurred())

				latestVR, err := resourceScope.LatestVersions()
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

				saveVersions(resourceScope, pretendCheckResults)

				err = resourceScope.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"ref": "v3"})
				Expect(err).ToNot(HaveOccurred())

				latestVR, err = resourceScope.LatestVersions()
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
				saveVersions(resourceScope, newVersionSlice)

				err := resourceScope.SaveSpaceLatestVersion(atc.Space("space"), atc.Version{"ref": "v3"})
				Expect(err).ToNot(HaveOccurred())

				latestVR, err := resourceScope.LatestVersions()
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
			err := resourceScope.SaveSpace(atc.Space("space"))
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

			saveVersions(resourceScope, []atc.SpaceVersion{otherSpaceVersion, spaceVersion})

			err = resourceScope.SaveSpace(atc.Space("space2"))
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

			saveVersions(resourceScope, []atc.SpaceVersion{spaceVersion2})
		})

		Context("when the version exists", func() {
			BeforeEach(func() {
				err := resourceScope.SaveSpaceLatestVersion(spaceVersion.Space, spaceVersion.Version)
				Expect(err).ToNot(HaveOccurred())

				err = resourceScope.SaveSpaceLatestVersion(spaceVersion2.Space, spaceVersion2.Version)
				Expect(err).ToNot(HaveOccurred())
			})

			It("saves the version into the space", func() {
				latestVersions, latestErr := resourceScope.LatestVersions()
				Expect(latestErr).ToNot(HaveOccurred())
				Expect(latestVersions).To(HaveLen(2))
				Expect(latestVersions[0].Version()).To(Equal(db.Version(spaceVersion.Version)))
				Expect(latestVersions[1].Version()).To(Equal(db.Version(spaceVersion2.Version)))
			})
		})

		Context("when the version does not exist", func() {
			It("does not save the version into the space", func() {
				err := resourceScope.SaveSpaceLatestVersion(spaceVersion.Space, atc.Version{"non-existant": "version"})
				Expect(err).ToNot(HaveOccurred())

				latestVersions, latestErr := resourceScope.LatestVersions()
				Expect(latestErr).ToNot(HaveOccurred())
				Expect(latestVersions).To(HaveLen(0))
			})
		})

		Context("when the space does not exist", func() {
			It("does not save the version into the space", func() {
				err := resourceScope.SaveSpaceLatestVersion(atc.Space("non-existant-space"), atc.Version{"some": "version"})
				Expect(err).ToNot(HaveOccurred())

				latestVersions, latestErr := resourceScope.LatestVersions()
				Expect(latestErr).ToNot(HaveOccurred())
				Expect(latestVersions).To(HaveLen(0))
			})
		})
	})

	Describe("UpdateLastChecked", func() {
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

			vrt, err := pipelineResourceTypes.Deserialize()
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = someResource.SetResourceConfig(
				logger,
				someResource.Source(),
				creds.NewVersionedResourceTypes(template.StaticVariables{}, vrt),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there has not been a check", func() {
			It("should update the last checked", func() {
				updated, err := resourceConfigScope.UpdateLastChecked(1*time.Second, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())
			})

			Context("when immediate", func() {
				It("should update the last checked", func() {
					updated, err := resourceConfigScope.UpdateLastChecked(1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})
			})
		})

		Context("when there has been a check recently", func() {
			BeforeEach(func() {
				updated, err := resourceConfigScope.UpdateLastChecked(1*time.Second, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(updated).To(BeTrue())
			})

			Context("when not immediate", func() {
				It("does not update the last checked", func() {
					updated, err := resourceConfigScope.UpdateLastChecked(1*time.Second, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeFalse())
				})

				It("updates the last checked and stops others from periodically updating at the same time", func() {
					Consistently(func() bool {
						updated, err := resourceConfigScope.UpdateLastChecked(1*time.Second, false)
						Expect(err).ToNot(HaveOccurred())

						return updated
					}, time.Second, 100*time.Millisecond).Should(BeFalse())

					time.Sleep(time.Second)

					updated, err := resourceConfigScope.UpdateLastChecked(1*time.Second, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})
			})

			Context("when it is immediate", func() {
				It("updates the last checked and stops others from updating too", func() {
					updated, err := resourceConfigScope.UpdateLastChecked(1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})
			})
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

			vrt, err := pipelineResourceTypes.Deserialize()
			Expect(err).ToNot(HaveOccurred())

			resourceConfigScope, err = someResource.SetResourceConfig(
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
				lock, acquired, err = resourceConfigScope.AcquireResourceCheckingLock(logger, 1*time.Second)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())
			})

			AfterEach(func() {
				_ = lock.Release()
			})

			It("does not get the lock", func() {
				_, acquired, err := resourceConfigScope.AcquireResourceCheckingLock(logger, 1*time.Second)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeFalse())
			})

			Context("and the lock gets released", func() {
				BeforeEach(func() {
					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})

				It("gets the lock", func() {
					lock, acquired, err := resourceConfigScope.AcquireResourceCheckingLock(logger, 1*time.Second)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			It("gets and keeps the lock and stops others from periodically getting it", func() {
				lock, acquired, err := resourceConfigScope.AcquireResourceCheckingLock(logger, 1*time.Second)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())

				Consistently(func() bool {
					_, acquired, err = resourceConfigScope.AcquireResourceCheckingLock(logger, 1*time.Second)
					Expect(err).ToNot(HaveOccurred())

					return acquired
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				err = lock.Release()
				Expect(err).ToNot(HaveOccurred())

				time.Sleep(time.Second)

				lock, acquired, err = resourceConfigScope.AcquireResourceCheckingLock(logger, 1*time.Second)
				Expect(err).ToNot(HaveOccurred())
				Expect(acquired).To(BeTrue())

				err = lock.Release()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})

func saveVersions(resourceConfigScope db.ResourceConfigScope, versions []atc.SpaceVersion) {
	for _, version := range versions {
		err := resourceConfigScope.SaveSpace(version.Space)
		Expect(err).ToNot(HaveOccurred())

		err = resourceConfigScope.SavePartialVersion(version.Space, version.Version, version.Metadata)
		Expect(err).ToNot(HaveOccurred())
	}

	err := resourceConfigScope.FinishSavingVersions()
	Expect(err).ToNot(HaveOccurred())
}
