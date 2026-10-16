package healthserver_test

import (
	"github.com/concourse/concourse/v8/atc/api/healthserver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseMinWorkerThresholds", func() {
	It("parses a semicolon-delimited single env var entry", func() {
		t, err := healthserver.ParseMinWorkerThresholds([]string{"tag:gpu:1;team::3;team:infra:1"})
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Tag).To(Equal(map[string]int{"gpu": 1}))
		Expect(t.Team).To(Equal(map[string]int{"": 3, "infra": 1}))
	})

	It("returns an empty result for nil input", func() {
		t, err := healthserver.ParseMinWorkerThresholds(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Platform).To(BeNil())
		Expect(t.Team).To(BeNil())
		Expect(t.Tag).To(BeNil())
	})

	It("returns an empty result for an empty slice", func() {
		t, err := healthserver.ParseMinWorkerThresholds([]string{})
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Platform).To(BeNil())
		Expect(t.Team).To(BeNil())
		Expect(t.Tag).To(BeNil())
	})

	DescribeTable("valid entries",
		func(entries []string, expected healthserver.MinWorkerThresholds) {
			t, err := healthserver.ParseMinWorkerThresholds(entries)
			Expect(err).NotTo(HaveOccurred())
			Expect(t).To(Equal(expected))
		},
		Entry("single platform threshold",
			[]string{"platform:linux:2"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]int{"linux": 2},
			},
		),
		Entry("single team threshold",
			[]string{"team:infra:1"},
			healthserver.MinWorkerThresholds{
				Team: map[string]int{"infra": 1},
			},
		),
		Entry("single tag threshold",
			[]string{"tag:gpu:1"},
			healthserver.MinWorkerThresholds{
				Tag: map[string]int{"gpu": 1},
			},
		),
		Entry("zero count is allowed",
			[]string{"platform:linux:0"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]int{"linux": 0},
			},
		),
		Entry("empty team name targets unscoped workers",
			[]string{"team::2"},
			healthserver.MinWorkerThresholds{
				Team: map[string]int{"": 2},
			},
		),
		Entry("multiple kinds together",
			[]string{"platform:linux:2", "team:infra:1", "tag:gpu:1"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]int{"linux": 2},
				Team:     map[string]int{"infra": 1},
				Tag:      map[string]int{"gpu": 1},
			},
		),
		Entry("multiple entries of same kind",
			[]string{"platform:linux:2", "platform:windows:1"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]int{"linux": 2, "windows": 1},
			},
		),
	)

	DescribeTable("invalid entries",
		func(entry string) {
			_, err := healthserver.ParseMinWorkerThresholds([]string{entry})
			Expect(err).To(HaveOccurred())
		},
		Entry("missing count segment", "platform:linux"),
		Entry("unknown kind", "zone:us-east:2"),
		Entry("empty platform name", "platform::2"),
		Entry("empty tag name", "tag::1"),
		Entry("non-numeric count", "platform:linux:two"),
		Entry("negative count", "platform:linux:-1"),
		Entry("no separators", "linux2"),
	)

	It("rejects duplicate platform entries", func() {
		_, err := healthserver.ParseMinWorkerThresholds([]string{
			"platform:linux:2",
			"platform:linux:3",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("duplicate platform"))
	})

	It("rejects duplicate team entries", func() {
		_, err := healthserver.ParseMinWorkerThresholds([]string{
			"team:infra:1",
			"team:infra:2",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("duplicate team"))
	})

	It("rejects duplicate tag entries", func() {
		_, err := healthserver.ParseMinWorkerThresholds([]string{
			"tag:gpu:1",
			"tag:gpu:2",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("duplicate tag"))
	})

	It("includes the offending entry in the error message", func() {
		_, err := healthserver.ParseMinWorkerThresholds([]string{"badentry"})
		Expect(err.Error()).To(ContainSubstring("badentry"))
	})
})
