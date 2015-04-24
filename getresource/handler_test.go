package getresource_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/getresource/fakes"
	"github.com/concourse/atc/web/group"

	. "github.com/concourse/atc/web/getresource"
)

var _ = Describe("FetchTemplateData", func() {
	var fakeDB *fakes.FakeResourcesDB

	BeforeEach(func() {
		fakeDB = new(fakes.FakeResourcesDB)
	})

	Context("when the config database returns an error", func() {
		BeforeEach(func() {
			fakeDB.GetConfigReturns(atc.Config{}, db.ConfigVersion(1), errors.New("disaster"))
		})

		It("returns an error if the config could not be loaded", func() {
			_, err := FetchTemplateData(fakeDB, "resource-name")
			Ω(err).Should(HaveOccurred())
		})
	})

	Context("when the config database returns a config", func() {
		var configResource atc.ResourceConfig

		BeforeEach(func() {
			configResource = atc.ResourceConfig{
				Name: "resource-name",
			}

			config := atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "group-with-resource",
						Resources: []string{"resource-name"},
					},
					{
						Name: "group-without-resource",
					},
				},
				Resources: atc.ResourceConfigs{configResource},
			}

			fakeDB.GetConfigReturns(config, db.ConfigVersion(1), nil)
		})

		It("returns not found if the resource cannot be found in the config", func() {
			_, err := FetchTemplateData(fakeDB, "not-a-resource-name")
			Ω(err).Should(HaveOccurred())
			Ω(err).Should(MatchError(ErrResourceConfigNotFound))
		})

		Context("when the resource history lookup returns an error", func() {
			BeforeEach(func() {
				fakeDB.GetResourceHistoryReturns(nil, errors.New("disaster"))
			})

			It("returns an error if the resource's history could not be retreived", func() {
				_, err := FetchTemplateData(fakeDB, "resource-name")
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when the resource history lookup returns history", func() {

			Context("when the resource lookup returns an error", func() {
				BeforeEach(func() {
					fakeDB.GetResourceReturns(db.SavedResource{}, errors.New("disaster"))
				})

				It("returns an error if the resource's history could not be retreived", func() {
					_, err := FetchTemplateData(fakeDB, "resource-name")
					Ω(err).Should(HaveOccurred())
				})
			})

			Context("when the resource lookup returns a resource", func() {
				var resource db.SavedResource
				var history []*db.VersionHistory

				BeforeEach(func() {
					resource = db.SavedResource{
						CheckError: nil,
						Paused:     false,
						Resource: db.Resource{
							Name: "resource-name",
						},
					}

					history = []*db.VersionHistory{
						&db.VersionHistory{
							VersionedResource: db.SavedVersionedResource{
								ID: 2,
								VersionedResource: db.VersionedResource{
									Resource: "resource-name",
								},
							},
						},
					}

					fakeDB.GetResourceReturns(resource, nil)
					fakeDB.GetResourceHistoryReturns(history, nil)
				})

				It("has the correct template data", func() {
					templateData, err := FetchTemplateData(fakeDB, "resource-name")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(templateData.GroupStates).Should(ConsistOf([]group.State{
						{
							Name:    "group-with-resource",
							Enabled: true,
						},
						{
							Name:    "group-without-resource",
							Enabled: false,
						},
					}))

					Ω(templateData.History).Should(Equal(history))
					Ω(templateData.Resource).Should(Equal(configResource))
					Ω(templateData.DBResource).Should(Equal(resource))
				})
			})
		})
	})
})
