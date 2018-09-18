package algorithm_test

import (
	"github.com/concourse/atc/db/algorithm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolve", func() {
	var (
		versionsDB   *algorithm.VersionsDB
		inputConfigs algorithm.InputConfigs
		inputMapping algorithm.InputMapping
	)

	BeforeEach(func() {
		versionsDB = &algorithm.VersionsDB{
			ResourceVersions: []algorithm.ResourceVersion{
				{VersionID: 1, ResourceID: 21, CheckOrder: 1},
				{VersionID: 2, ResourceID: 21, CheckOrder: 2},
			},
			BuildOutputs: []algorithm.BuildOutput{},
			BuildInputs:  []algorithm.BuildInput{},
			JobIDs:       map[string]int{"j1": 11, "j2": 12},
			ResourceIDs:  map[string]int{"r1": 21},
		}

		inputConfigs = algorithm.InputConfigs{
			{
				Name:       "some-input",
				JobName:    "j1",
				Passed:     algorithm.JobSet{},
				ResourceID: 21,
				JobID:      11,
			},
		}
	})

	JustBeforeEach(func() {
		var ok bool
		inputMapping, ok = inputConfigs.Resolve(versionsDB)
		Expect(ok).To(BeTrue())
	})

	Context("when the version was an input of the same job with the same name", func() {
		BeforeEach(func() {
			versionsDB.BuildInputs = []algorithm.BuildInput{
				{
					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
					BuildID:         31,
					JobID:           11,
					InputName:       "some-input",
				},
				{
					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
					BuildID:         31,
					JobID:           11,
					InputName:       "some-other-input",
				},
				{
					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
					BuildID:         32,
					JobID:           12,
					InputName:       "some-input",
				},
			}
		})

		It("sets FirstOccurrence to false", func() {
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputVersion{VersionID: 2, FirstOccurrence: false},
			}))
		})
	})

	Context("when the version was an input of the same job with a different name", func() {
		BeforeEach(func() {
			versionsDB.BuildInputs = []algorithm.BuildInput{
				{
					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
					BuildID:         31,
					JobID:           11,
					InputName:       "some-other-input",
				},
			}
		})

		It("sets FirstOccurrence to true", func() {
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputVersion{VersionID: 2, FirstOccurrence: true},
			}))
		})
	})

	Context("when the version was an input of a different job with the same name", func() {
		BeforeEach(func() {
			versionsDB.BuildInputs = []algorithm.BuildInput{
				{
					ResourceVersion: algorithm.ResourceVersion{VersionID: 2, ResourceID: 21, CheckOrder: 2},
					BuildID:         32,
					JobID:           12,
					InputName:       "some-input",
				},
			}
		})

		It("sets FirstOccurrence to true", func() {
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputVersion{VersionID: 2, FirstOccurrence: true},
			}))
		})
	})

	Context("when a different version was an input of the same job with the same name", func() {
		BeforeEach(func() {
			versionsDB.BuildInputs = []algorithm.BuildInput{
				{
					ResourceVersion: algorithm.ResourceVersion{VersionID: 1, ResourceID: 21, CheckOrder: 1},
					BuildID:         31,
					JobID:           11,
					InputName:       "some-input",
				},
			}
		})

		It("sets FirstOccurrence to true", func() {
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputVersion{VersionID: 2, FirstOccurrence: true},
			}))
		})
	})

	Context("when a different version was an output of the same job", func() {
		BeforeEach(func() {
			versionsDB.BuildOutputs = []algorithm.BuildOutput{
				{
					ResourceVersion: algorithm.ResourceVersion{VersionID: 1, ResourceID: 21, CheckOrder: 1},
					BuildID:         31,
					JobID:           11,
				},
			}
		})

		It("sets FirstOccurrence to true", func() {
			Expect(inputMapping).To(Equal(algorithm.InputMapping{
				"some-input": algorithm.InputVersion{VersionID: 2, FirstOccurrence: true},
			}))
		})
	})
})
