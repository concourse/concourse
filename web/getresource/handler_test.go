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
			_, err := FetchTemplateData(fakeDB, false, "resource-name", 0, false)
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
			_, err := FetchTemplateData(fakeDB, false, "not-a-resource-name", 0, false)
			Ω(err).Should(HaveOccurred())
			Ω(err).Should(MatchError(ErrResourceConfigNotFound))
		})

		Context("when the resource history lookup returns an error", func() {
			BeforeEach(func() {
				fakeDB.GetResourceHistoryCursorReturns(nil, false, errors.New("disaster"))
			})

			It("returns an error if the resource's history could not be retreived", func() {
				_, err := FetchTemplateData(fakeDB, false, "resource-name", 0, false)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when the resource history lookup returns history", func() {
			Context("when the resource lookup returns an error", func() {
				BeforeEach(func() {
					fakeDB.GetResourceReturns(db.SavedResource{}, errors.New("disaster"))
				})

				It("returns an error if the resource's history could not be retreived", func() {
					_, err := FetchTemplateData(fakeDB, false, "resource-name", 0, false)
					Ω(err).Should(HaveOccurred())
				})
			})

			Context("when the resource lookup returns a resource", func() {
				var resource db.SavedResource
				var history []*db.VersionHistory

				BeforeEach(func() {
					resource = db.SavedResource{
						CheckError:   errors.New("a disaster!"),
						Paused:       false,
						PipelineName: "pipeline",
						Resource: db.Resource{
							Name: "resource-name",
						},
					}

					fakeDB.GetResourceReturns(resource, nil)
				})

				Context("when we are logged in", func() {
					authenticated := true

					Context("when looking up the max id fails", func() {
						BeforeEach(func() {
							fakeDB.GetResourceHistoryMaxIDReturns(0, errors.New("disaster"))
						})

						It("returns an error", func() {
							_, err := FetchTemplateData(fakeDB, false, "resource-name", 0, false)
							Ω(err).Should(HaveOccurred())
						})
					})

					Context("when looking up the max id succeeds", func() {
						Context("when there are more than 100 results", func() {
							const MaxID int = 150

							BeforeEach(func() {
								fakeDB.GetResourceHistoryMaxIDReturns(MaxID, nil)

								history = []*db.VersionHistory{
									&db.VersionHistory{
										VersionedResource: db.SavedVersionedResource{
											ID: 150,
											VersionedResource: db.VersionedResource{
												Resource: "resource-name",
											},
										},
									},
									&db.VersionHistory{
										VersionedResource: db.SavedVersionedResource{
											ID: 51,
											VersionedResource: db.VersionedResource{
												Resource: "resource-name",
											},
										},
									},
								}

								fakeDB.GetResourceHistoryCursorReturns(history, true, nil)
							})

							Context("when the passed in id is 0", func() {
								It("uses the max id to pull history", func() {
									templateData, err := FetchTemplateData(fakeDB, false, "resource-name", 0, false)
									Ω(err).ShouldNot(HaveOccurred())

									Ω(fakeDB.GetResourceHistoryCursorCallCount()).Should(Equal(1))
									resourceName, startingID, searchUpwards, numResults := fakeDB.GetResourceHistoryCursorArgsForCall(0)
									Ω(resourceName).Should(Equal("resource-name"))
									Ω(startingID).Should(Equal(MaxID))
									Ω(searchUpwards).Should(BeFalse())
									Ω(numResults).Should(Equal(100))
									Ω(templateData.PaginationData.HasPagination()).Should(BeTrue())
								})
							})

							It("has the correct template data", func() {
								templateData, err := FetchTemplateData(fakeDB, authenticated, "resource-name", 0, false)
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
								Ω(templateData.Resource).Should(Equal(atc.Resource{
									Name:           "resource-name",
									URL:            "/pipelines/pipeline/resources/resource-name",
									Groups:         []string{"group-with-resource"},
									FailingToCheck: true,
									CheckError:     "a disaster!",
								}))
								Ω(templateData.PaginationData.HasPagination()).Should(BeTrue())
							})
						})
					})
				})

				Context("when we are not logged in", func() {
					authenticated := false

					BeforeEach(func() {
						history = []*db.VersionHistory{}
						fakeDB.GetResourceHistoryCursorReturns(history, true, nil)
					})

					It("has the correct template data", func() {
						templateData, err := FetchTemplateData(fakeDB, authenticated, "resource-name", 0, false)
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
						Ω(templateData.Resource).Should(Equal(atc.Resource{
							Name:           "resource-name",
							URL:            "/pipelines/pipeline/resources/resource-name",
							Groups:         []string{"group-with-resource"},
							FailingToCheck: true,
							CheckError:     "",
						}))
					})
				})
			})
		})
	})
})
