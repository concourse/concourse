package getresource_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/web/getresource"
	"github.com/concourse/atc/web/group"
	"github.com/concourse/go-concourse/concourse"
	cfakes "github.com/concourse/go-concourse/concourse/concoursefakes"
)

var _ = Describe("FetchTemplateData", func() {
	var fakeTeam *cfakes.FakeTeam
	var templateData TemplateData
	var fetchErr error

	BeforeEach(func() {
		fakeTeam = new(cfakes.FakeTeam)
	})

	JustBeforeEach(func() {
		templateData, fetchErr = FetchTemplateData("some-pipeline", "some-resource", fakeTeam, concourse.Page{
			Since: 398,
			Until: 2,
		})
	})

	It("calls to get the pipeline config", func() {
		Expect(fakeTeam.PipelineCallCount()).To(Equal(1))
		Expect(fakeTeam.PipelineArgsForCall(0)).To(Equal("some-pipeline"))
	})

	Context("when getting the pipeline returns an error", func() {
		var expectedErr error

		BeforeEach(func() {
			expectedErr = errors.New("disaster")
			fakeTeam.PipelineReturns(atc.Pipeline{}, false, expectedErr)
		})

		It("returns an error if the config could not be loaded", func() {
			Expect(fetchErr).To(Equal(expectedErr))
		})
	})

	Context("when the pipeline is not found", func() {
		BeforeEach(func() {
			fakeTeam.PipelineReturns(atc.Pipeline{}, false, nil)
		})

		It("returns an error if the config could not be loaded", func() {
			Expect(fetchErr).To(Equal(ErrConfigNotFound))
		})
	})

	Context("when the api returns the pipeline", func() {
		BeforeEach(func() {
			fakeTeam.PipelineReturns(atc.Pipeline{
				Groups: atc.GroupConfigs{
					{
						Name:      "group-with-resource",
						Resources: []string{"some-resource"},
					},
					{
						Name:      "group-without-resource",
						Resources: []string{"some-other-resource"},
					},
				},
			}, true, nil)
		})

		It("calls to get the resource", func() {
			Expect(fakeTeam.ResourceCallCount()).To(Equal(1))
			pipelineName, resourceName := fakeTeam.ResourceArgsForCall(0)
			Expect(pipelineName).To(Equal("some-pipeline"))
			Expect(resourceName).To(Equal("some-resource"))
		})

		Context("when the call returns an error", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("nope")
				fakeTeam.ResourceReturns(atc.Resource{}, false, expectedErr)
			})

			It("errors", func() {
				Expect(fetchErr).To(Equal(expectedErr))
			})
		})

		Context("when it can't find the resource", func() {
			BeforeEach(func() {
				fakeTeam.ResourceReturns(atc.Resource{}, false, nil)
			})

			It("returns a resource not found error", func() {
				Expect(fetchErr).To(Equal(ErrResourceNotFound))
			})
		})

		Context("when the call to get a resource is successful", func() {
			var expectedResource atc.Resource

			BeforeEach(func() {
				expectedResource = atc.Resource{
					Name:           "some-resource",
					CheckError:     "some-error",
					FailingToCheck: true,
				}
				fakeTeam.ResourceReturns(expectedResource, true, nil)
			})

			It("calls to get the resource versions", func() {
				Expect(fakeTeam.ResourceVersionsCallCount()).To(Equal(1))
				actualPipelineName, actualResourceName, actualPage := fakeTeam.ResourceVersionsArgsForCall(0)
				Expect(actualPipelineName).To(Equal("some-pipeline"))
				Expect(actualResourceName).To(Equal("some-resource"))
				Expect(actualPage).To(Equal(concourse.Page{
					Since: 398,
					Until: 2,
				}))
			})

			Context("when the call to get resource versions returns an error", func() {
				var expectedErr error
				BeforeEach(func() {
					expectedErr = errors.New("nope")
					fakeTeam.ResourceVersionsReturns(nil, concourse.Pagination{}, false, expectedErr)
				})

				It("errors", func() {
					Expect(fetchErr).To(Equal(expectedErr))
				})
			})

			Context("when it can't find the resource versions", func() {
				BeforeEach(func() {
					fakeTeam.ResourceVersionsReturns(nil, concourse.Pagination{}, false, nil)
				})

				It("returns a resource not found error", func() {
					Expect(fetchErr).To(Equal(ErrResourceNotFound))
				})
			})

			Context("when it returns resource versions", func() {
				var versionedResources []atc.VersionedResource
				var pagination concourse.Pagination

				BeforeEach(func() {
					versionedResources = []atc.VersionedResource{
						{
							ID:         2,
							PipelineID: 57,
							Type:       "some-type",
							Metadata: []atc.MetadataField{
								{
									Name:  "some",
									Value: "metadata",
								},
							},
							Resource: "some-resource",
							Version: atc.Version{
								"version": "v1",
							},
						},
						{
							ID:         3,
							PipelineID: 57,
							Type:       "some-type",
							Metadata: []atc.MetadataField{
								{
									Name:  "some",
									Value: "metadata",
								},
							},
							Resource: "some-resource",
							Version: atc.Version{
								"version": "v2",
							},
						},
					}

					pagination = concourse.Pagination{
						Previous: &concourse.Page{
							Until: 42,
							Limit: 100,
						},
						Next: &concourse.Page{
							Since: 43,
							Limit: 100,
						},
					}
					fakeTeam.ResourceVersionsReturns(versionedResources, pagination, true, nil)
				})

				It("calls to get builds that the versions were inputs to", func() {
					Expect(fakeTeam.BuildsWithVersionAsInputCallCount()).To(Equal(2))
					firstCallPipelineName, firstCallResourceName, firstCallResourceVersionID := fakeTeam.BuildsWithVersionAsInputArgsForCall(0)
					Expect(firstCallPipelineName).To(Equal("some-pipeline"))
					Expect(firstCallResourceName).To(Equal("some-resource"))
					Expect(firstCallResourceVersionID).To(Equal(2))

					secondCallPipelineName, secondCallResourceName, secondCallResourceVersionID := fakeTeam.BuildsWithVersionAsInputArgsForCall(1)
					Expect(secondCallPipelineName).To(Equal("some-pipeline"))
					Expect(secondCallResourceName).To(Equal("some-resource"))
					Expect(secondCallResourceVersionID).To(Equal(3))
				})

				Context("when it errors attempting to acquire builds", func() {
					var expectedErr error
					BeforeEach(func() {
						expectedErr = errors.New("nope")
						fakeTeam.BuildsWithVersionAsInputReturns(nil, false, expectedErr)
					})

					It("returns an error", func() {
						Expect(fetchErr).To(Equal(expectedErr))
					})
				})

				Context("when it returns builds", func() {
					var expectedBuildsForInput []atc.Build

					BeforeEach(func() {
						expectedBuildsForInput = []atc.Build{
							{
								ID:           7,
								Name:         "1",
								Status:       "started",
								JobName:      "some-job",
								URL:          "some-url",
								APIURL:       "some-api-url",
								PipelineName: "some-pipeline",
								StartTime:    time.Now().Unix(),
								EndTime:      time.Now().Unix(),
							},
							{
								ID:           9,
								Name:         "2",
								Status:       "started",
								JobName:      "some-job",
								URL:          "some-url",
								APIURL:       "some-api-url",
								PipelineName: "some-pipeline",
								StartTime:    time.Now().Unix(),
								EndTime:      time.Now().Unix(),
							},
						}
						fakeTeam.BuildsWithVersionAsInputReturns(expectedBuildsForInput, true, nil)
					})

					It("calls to get the outputs", func() {
						Expect(fakeTeam.BuildsWithVersionAsOutputCallCount()).To(Equal(2))
						firstCallPipelineName, firstCallResourceName, firstCallResourceVersionID := fakeTeam.BuildsWithVersionAsOutputArgsForCall(0)
						Expect(firstCallPipelineName).To(Equal("some-pipeline"))
						Expect(firstCallResourceName).To(Equal("some-resource"))
						Expect(firstCallResourceVersionID).To(Equal(2))

						secondCallPipelineName, secondCallResourceName, secondCallResourceVersionID := fakeTeam.BuildsWithVersionAsOutputArgsForCall(1)
						Expect(secondCallPipelineName).To(Equal("some-pipeline"))
						Expect(secondCallResourceName).To(Equal("some-resource"))
						Expect(secondCallResourceVersionID).To(Equal(3))
					})

					Context("when it errors attempting to acquire builds", func() {
						var expectedErr error
						BeforeEach(func() {
							expectedErr = errors.New("nope")
							fakeTeam.BuildsWithVersionAsOutputReturns(nil, false, expectedErr)
						})

						It("returns an error", func() {
							Expect(fetchErr).To(Equal(expectedErr))
						})
					})

					Context("when it successfully acquires builds", func() {
						var expectedBuildsForOutput []atc.Build

						BeforeEach(func() {
							expectedBuildsForOutput = []atc.Build{
								{
									ID:           117,
									Name:         "1",
									Status:       "started",
									JobName:      "some-job",
									URL:          "some-url",
									APIURL:       "some-api-url",
									PipelineName: "some-pipeline",
									StartTime:    time.Now().Unix(),
									EndTime:      time.Now().Unix(),
								},
								{
									ID:           1337,
									Name:         "2",
									Status:       "started",
									JobName:      "some-job",
									URL:          "some-url",
									APIURL:       "some-api-url",
									PipelineName: "some-pipeline",
									StartTime:    time.Now().Unix(),
									EndTime:      time.Now().Unix(),
								},
							}
							fakeTeam.BuildsWithVersionAsOutputReturns(expectedBuildsForOutput, true, nil)
							fakeTeam.NameReturns("some-team")
						})

						It("returns the correct TemplateData", func() {
							Expect(templateData.Resource).To(Equal(expectedResource))
							Expect(templateData.Versions).To(Equal([]VersionedResourceWithInputsAndOutputs{
								{
									VersionedResource: versionedResources[0],
									InputsTo:          map[string][]atc.Build{"some-job": expectedBuildsForInput},
									OutputsOf:         map[string][]atc.Build{"some-job": expectedBuildsForOutput},
								},
								{
									VersionedResource: versionedResources[1],
									InputsTo:          map[string][]atc.Build{"some-job": expectedBuildsForInput},
									OutputsOf:         map[string][]atc.Build{"some-job": expectedBuildsForOutput},
								},
							}))
							Expect(templateData.TeamName).To(Equal("some-team"))
							Expect(templateData.PipelineName).To(Equal("some-pipeline"))
							Expect(templateData.GroupStates).To(ConsistOf([]group.State{
								{
									Name:    "group-with-resource",
									Enabled: true,
								},
								{
									Name:    "group-without-resource",
									Enabled: false,
								},
							}))
							Expect(templateData.Pagination).To(Equal(pagination))
						})
					})
				})
			})
		})
	})
})
