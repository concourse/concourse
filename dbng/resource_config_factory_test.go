package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigFactory", func() {
	DescribeTable("CleanConfigUsesForFinishedBuilds",
		func(i bool, diff int) {
			b, err := defaultPipeline.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			err = b.SetInterceptible(i)
			Expect(err).NotTo(HaveOccurred())

			_, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, dbng.ForBuild{BuildID: b.ID()}, "some-base-resource-type", atc.Source{}, atc.VersionedResourceTypes{})
			Expect(err).NotTo(HaveOccurred())

			var (
				rcuCountBefore int
				rcuCountAfter  int
			)

			dbConn.QueryRow("select count(*) from resource_config_uses").Scan(&rcuCountBefore)

			resourceConfigFactory.CleanConfigUsesForFinishedBuilds()
			Expect(err).NotTo(HaveOccurred())

			dbConn.QueryRow("select count(*) from resource_config_uses").Scan(&rcuCountAfter)

			Expect(rcuCountBefore - rcuCountAfter).To(Equal(diff))
		},
		Entry("non-interceptible builds are deleted", false, 1),
		Entry("interceptible builds are not deleted", true, 0),
	)
})
