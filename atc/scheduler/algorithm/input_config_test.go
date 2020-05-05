package algorithm_test

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/scheduler/algorithm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Inputs", func() {
	var (
		algorithm         Algorithm
		fakeResource      *dbfakes.FakeResource
		expectedResources db.Resources
		expectedJobIDs    NameToIDMap

		Latest   = &atc.VersionConfig{Latest: true}
		Every    = &atc.VersionConfig{Every: true}
		Version1 = atc.Version{"ver": "v1"}
		Version2 = atc.Version{"ver": "v2"}
		PinnedV1 = &atc.VersionConfig{Pinned: Version1}
	)

	BeforeEach(func() {
		algorithm = Algorithm{}
		fakeResource = new(dbfakes.FakeResource)
		fakeResource.NameReturns("some-resource")

		expectedResources = db.Resources{fakeResource}
		expectedJobIDs = NameToIDMap{"j1": 1}
	})

	DescribeTable("CreateInputConfigs",
		func(
			jobVersion *atc.VersionConfig,
			resourcePinnedVersion atc.Version,
			expectedUseEveryVersion bool,
			expectedPinnedVersion atc.Version,
		) {
			if resourcePinnedVersion != nil {
				fakeResource.CurrentPinnedVersionReturns(resourcePinnedVersion)
			}

			jobInput := atc.JobInput{
				Name: "a", Resource: fakeResource.Name(), Trigger: true, Version: jobVersion,
			}
			inputConfigs, err := algorithm.CreateInputConfigs(1, []atc.JobInput{jobInput}, expectedResources, expectedJobIDs)
			Expect(err).ToNot(HaveOccurred())
			Expect(inputConfigs).To(HaveLen(1))
			Expect(inputConfigs[0].UseEveryVersion).To(Equal(expectedUseEveryVersion))
			Expect(inputConfigs[0].PinnedVersion).To(Equal(expectedPinnedVersion))
		},
		Entry("no job version, no resource version", nil, nil, false, nil),
		Entry("no job version, resource version pinned", nil, Version1, false, Version1),
		Entry("job version latest, no resource version", Latest, nil, false, nil),
		Entry("job version latest, resource version pinned", Latest, Version1, false, Version1),
		Entry("job version every, no resource version", Every, nil, true, nil),
		Entry("job version every, resource version pinned", Every, Version1, false, Version1),
		Entry("job version pinned, no resource version", PinnedV1, nil, false, Version1),
		Entry("job version pinned, resource version pinned", PinnedV1, Version2, false, Version1),
	)

	Describe("when no matching resource exists", func() {
		It("returns the error", func() {
			jobInput := atc.JobInput{
				Name: "a", Resource: "foo", Trigger: true,
			}
			inputConfigs, err := algorithm.CreateInputConfigs(1, []atc.JobInput{jobInput}, expectedResources, expectedJobIDs)
			Expect(inputConfigs).To(BeNil())
			Expect(err).To(Equal(errors.New("input resource not found")))
		})
	})

	Describe("passed jobs", func() {
		Context("when there are no passed constraints", func() {
			It("returns an empty set of jobs", func() {
				jobInput := atc.JobInput{
					Name: "a", Resource: fakeResource.Name(), Trigger: true, Passed: []string{},
				}
				inputConfigs, err := algorithm.CreateInputConfigs(1, []atc.JobInput{jobInput}, expectedResources, expectedJobIDs)
				Expect(err).ToNot(HaveOccurred())
				Expect(inputConfigs).To(HaveLen(1))
				Expect(inputConfigs[0].Passed).To(Equal(db.JobSet{}))
			})
		})
		Context("when there are passed jobs", func() {
			It("returns a job set marking that job as passed", func() {
				expectedJobIDs = NameToIDMap{"j1": 1, "j2": 2}
				jobInput := atc.JobInput{
					Name: "a", Resource: fakeResource.Name(), Trigger: true, Passed: []string{"j1", "j2"},
				}
				inputConfigs, err := algorithm.CreateInputConfigs(1, []atc.JobInput{jobInput}, expectedResources, expectedJobIDs)
				Expect(err).ToNot(HaveOccurred())
				Expect(inputConfigs).To(HaveLen(1))
				Expect(inputConfigs[0].Passed).To(Equal(db.JobSet{1: true, 2: true}))
			})
		})
	})
})
