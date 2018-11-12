package inputconfig_test

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/scheduler/inputmapper/inputconfig"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transformer", func() {
	var (
		fakePipeline *dbfakes.FakePipeline
		transformer  inputconfig.Transformer
	)

	BeforeEach(func() {
		fakePipeline = new(dbfakes.FakePipeline)
		transformer = inputconfig.NewTransformer(fakePipeline)
	})

	Describe("TransformInputConfigs", func() {
		Context("when the job name exists in the versionsDB", func() {
			var (
				jobInputs       []atc.JobInput
				algorithmInputs algorithm.InputConfigs
				tranformErr     error
			)

			JustBeforeEach(func() {
				algorithmInputs, tranformErr = transformer.TransformInputConfigs(
					&algorithm.VersionsDB{
						JobIDs:      map[string]int{"j1": 1, "j2": 2},
						ResourceIDs: map[string]int{"r1": 11, "r2": 12},
					},
					"j1",
					jobInputs,
				)
			})

			Context("when an input has nil version", func() {
				BeforeEach(func() {
					jobInputs = []atc.JobInput{{
						Name:     "job-input-1",
						Resource: "r1",
						Version:  nil,
					}}
				})

				It("defaults to latest version", func() {
					Expect(algorithmInputs).To(ConsistOf(algorithm.InputConfig{
						Name:            "job-input-1",
						UseEveryVersion: false,
						PinnedVersionID: 0,
						ResourceID:      11,
						Passed:          algorithm.JobSet{},
						JobID:           1,
					}))
				})
			})

			Context("when an input has passed constraints", func() {
				BeforeEach(func() {
					jobInputs = []atc.JobInput{{
						Name:     "job-input-1",
						Resource: "r1",
						Version:  &atc.VersionConfig{Latest: true},
						Passed:   []string{"j1", "j2"},
					}}
				})

				It("expresses them as a JobSet", func() {
					Expect(algorithmInputs).To(ConsistOf(algorithm.InputConfig{
						Name:            "job-input-1",
						UseEveryVersion: false,
						PinnedVersionID: 0,
						ResourceID:      11,
						Passed:          algorithm.JobSet{1: struct{}{}, 2: struct{}{}},
						JobID:           1,
					}))
				})
			})

			Context("when an input has version: every", func() {
				BeforeEach(func() {
					jobInputs = []atc.JobInput{{
						Name:     "job-input-1",
						Resource: "r1",
						Version:  &atc.VersionConfig{Every: true, Latest: true}, // spice things up a bit
					}}
				})

				It("uses every version", func() {
					Expect(algorithmInputs).To(ConsistOf(algorithm.InputConfig{
						Name:            "job-input-1",
						UseEveryVersion: true,
						PinnedVersionID: 0,
						ResourceID:      11,
						Passed:          algorithm.JobSet{},
						JobID:           1,
					}))
				})
			})

			Context("when an input has a pinned version", func() {
				BeforeEach(func() {
					jobInputs = []atc.JobInput{
						{
							Name:     "job-input-1",
							Resource: "r1",
							Version:  &atc.VersionConfig{Pinned: atc.Version{"version": "v1"}},
						},
						{
							Name:     "job-input-2",
							Resource: "r2",
							Version:  &atc.VersionConfig{Latest: true},
						},
					}
				})

				Context("when looking up the resource fails", func() {
					BeforeEach(func() {
						fakePipeline.ResourceReturns(nil, false, errors.New("ah"))
					})

					It("returns the error", func() {
						Expect(tranformErr).To(Equal(errors.New("ah")))
					})
				})

				Context("when the resource is not found", func() {
					BeforeEach(func() {
						fakePipeline.ResourceReturns(nil, false, nil)
					})

					It("omits the entire input", func() {
						Expect(algorithmInputs).To(ConsistOf(algorithm.InputConfig{
							Name:            "job-input-2",
							UseEveryVersion: false,
							PinnedVersionID: 0,
							ResourceID:      12,
							Passed:          algorithm.JobSet{},
							JobID:           1,
						}))
					})
				})

				Context("when the resource is found", func() {
					var fakeResource *dbfakes.FakeResource

					BeforeEach(func() {
						fakeResource = new(dbfakes.FakeResource)
						fakePipeline.ResourceReturns(fakeResource, true, nil)
					})

					Context("when looking up the pinned version fails", func() {
						var disaster error

						BeforeEach(func() {
							disaster = errors.New("bad thing")
							fakeResource.ResourceConfigVersionIDReturns(0, false, disaster)
						})

						It("returns the error", func() {
							Expect(tranformErr).To(Equal(disaster))
						})

						It("looked up the version id with the right resource and version", func() {
							Expect(fakeResource.ResourceConfigVersionIDCallCount()).To(Equal(1))
							actualVersion := fakeResource.ResourceConfigVersionIDArgsForCall(0)
							Expect(actualVersion).To(Equal(atc.Version{"version": "v1"}))
						})
					})

					Context("when the pinned version is not found", func() {
						BeforeEach(func() {
							fakeResource.ResourceConfigVersionIDReturns(0, false, nil)
						})

						It("omits the entire input", func() {
							Expect(algorithmInputs).To(ConsistOf(algorithm.InputConfig{
								Name:            "job-input-2",
								UseEveryVersion: false,
								PinnedVersionID: 0,
								ResourceID:      12,
								Passed:          algorithm.JobSet{},
								JobID:           1,
							}))
						})
					})

					Context("when the pinned version is found", func() {
						BeforeEach(func() {
							fakeResource.ResourceConfigVersionIDReturns(99, true, nil)
						})

						It("sets the pinned version ID", func() {
							Expect(algorithmInputs).To(ConsistOf(
								algorithm.InputConfig{
									Name:            "job-input-1",
									UseEveryVersion: false,
									PinnedVersionID: 99,
									ResourceID:      11,
									Passed:          algorithm.JobSet{},
									JobID:           1,
								},
								algorithm.InputConfig{
									Name:            "job-input-2",
									UseEveryVersion: false,
									PinnedVersionID: 0,
									ResourceID:      12,
									Passed:          algorithm.JobSet{},
									JobID:           1,
								},
							))
						})
					})
				})
			})
		})

		Context("when an input has things that don't exist", func() {
			It("at least doesn't panic", func() {
				algorithmInputs, transformErr := transformer.TransformInputConfigs(
					&algorithm.VersionsDB{},
					"no",
					[]atc.JobInput{{
						Name:     "job-input-1",
						Resource: "nah",
						Version:  &atc.VersionConfig{},
						Passed:   []string{"nope", "gone"},
					}},
				)
				Expect(transformErr).NotTo(HaveOccurred())
				Expect(algorithmInputs).To(ConsistOf(algorithm.InputConfig{
					Name:            "job-input-1",
					UseEveryVersion: false,
					PinnedVersionID: 0,
					ResourceID:      0,
					Passed:          algorithm.JobSet{0: struct{}{}},
					JobID:           0,
				}))
			})
		})
	})
})
