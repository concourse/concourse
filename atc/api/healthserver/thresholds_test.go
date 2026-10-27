package healthserver_test

import (
	"github.com/concourse/concourse/v8/atc/api/healthserver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseMinWorkerThresholds", func() {
	It("parses a comma-delimited single env var entry", func() {
		t, err := healthserver.ParseMinWorkerThresholds([]string{"tag:gpu:1:critical", "team::3:critical", "team:infra:1:critical"})
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Tag).To(Equal(map[string]healthserver.ThresholdEntry{
			"gpu": {Min: 1, Severity: healthserver.ThresholdSeverityCritical},
		}))
		Expect(t.Team).To(Equal(map[string]healthserver.ThresholdEntry{
			"":      {Min: 3, Severity: healthserver.ThresholdSeverityCritical},
			"infra": {Min: 1, Severity: healthserver.ThresholdSeverityCritical},
		}))
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
		Entry("linux platform threshold",
			[]string{"platform:linux:2:critical"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"linux": {Min: 2, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("windows platform threshold",
			[]string{"platform:windows:1:critical"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"windows": {Min: 1, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("darwin platform threshold",
			[]string{"platform:darwin:1:critical"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"darwin": {Min: 1, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("explicit critical severity",
			[]string{"platform:linux:2:critical"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"linux": {Min: 2, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("warn severity",
			[]string{"team:infra:1:warn"},
			healthserver.MinWorkerThresholds{
				Team: map[string]healthserver.ThresholdEntry{"infra": {Min: 1, Severity: healthserver.ThresholdSeverityWarn}},
			},
		),
		Entry("info severity",
			[]string{"tag:gpu:0:info"},
			healthserver.MinWorkerThresholds{
				Tag: map[string]healthserver.ThresholdEntry{"gpu": {Min: 0, Severity: healthserver.ThresholdSeverityInfo}},
			},
		),
		Entry("single team threshold",
			[]string{"team:infra:1:critical"},
			healthserver.MinWorkerThresholds{
				Team: map[string]healthserver.ThresholdEntry{"infra": {Min: 1, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("single tag threshold",
			[]string{"tag:gpu:1:critical"},
			healthserver.MinWorkerThresholds{
				Tag: map[string]healthserver.ThresholdEntry{"gpu": {Min: 1, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("zero count is allowed",
			[]string{"platform:linux:0:critical"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"linux": {Min: 0, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("empty team name targets unscoped workers",
			[]string{"team::2:critical"},
			healthserver.MinWorkerThresholds{
				Team: map[string]healthserver.ThresholdEntry{"": {Min: 2, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("multiple kinds together",
			[]string{"platform:linux:2:critical", "team:infra:1:critical", "tag:gpu:1:critical"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"linux": {Min: 2, Severity: healthserver.ThresholdSeverityCritical}},
				Team:     map[string]healthserver.ThresholdEntry{"infra": {Min: 1, Severity: healthserver.ThresholdSeverityCritical}},
				Tag:      map[string]healthserver.ThresholdEntry{"gpu": {Min: 1, Severity: healthserver.ThresholdSeverityCritical}},
			},
		),
		Entry("mixed severities",
			[]string{"platform:linux:2:critical", "tag:gpu:1:warn", "tag:cpu:0:info"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{"linux": {Min: 2, Severity: healthserver.ThresholdSeverityCritical}},
				Tag: map[string]healthserver.ThresholdEntry{
					"gpu": {Min: 1, Severity: healthserver.ThresholdSeverityWarn},
					"cpu": {Min: 0, Severity: healthserver.ThresholdSeverityInfo},
				},
			},
		),
		Entry("multiple entries of same kind",
			[]string{"platform:linux:2:critical", "platform:windows:1:warn"},
			healthserver.MinWorkerThresholds{
				Platform: map[string]healthserver.ThresholdEntry{
					"linux":   {Min: 2, Severity: healthserver.ThresholdSeverityCritical},
					"windows": {Min: 1, Severity: healthserver.ThresholdSeverityWarn},
				},
			},
		),
	)

	DescribeTable("invalid entries",
		func(entry string) {
			_, err := healthserver.ParseMinWorkerThresholds([]string{entry})
			Expect(err).To(HaveOccurred())
		},
		Entry("missing severity", "platform:linux:2"),
		Entry("missing count segment", "platform:linux"),
		Entry("unknown kind", "zone:us-east:2:critical"),
		Entry("invalid platform name", "platform:freebsd:1:critical"),
		Entry("empty platform name", "platform::2:critical"),
		Entry("empty tag name", "tag::1:critical"),
		Entry("non-numeric count", "platform:linux:two:critical"),
		Entry("negative count", "platform:linux:-1:critical"),
		Entry("no separators", "linux2"),
		Entry("invalid severity", "platform:linux:2:unknown"),
	)

	It("rejects duplicate platform entries", func() {
		_, err := healthserver.ParseMinWorkerThresholds([]string{
			"platform:linux:2:critical",
			"platform:linux:3:warn",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("duplicate platform"))
	})

	It("rejects duplicate team entries", func() {
		_, err := healthserver.ParseMinWorkerThresholds([]string{
			"team:infra:1:critical",
			"team:infra:2:warn",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("duplicate team"))
	})

	It("rejects duplicate tag entries", func() {
		_, err := healthserver.ParseMinWorkerThresholds([]string{
			"tag:gpu:1:critical",
			"tag:gpu:2:warn",
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("duplicate tag"))
	})

	It("includes the offending entry in the error message", func() {
		_, err := healthserver.ParseMinWorkerThresholds([]string{"badentry"})
		Expect(err.Error()).To(ContainSubstring("badentry"))
	})
})
