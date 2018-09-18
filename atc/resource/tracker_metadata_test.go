package resource_test

import (
	. "github.com/concourse/atc/resource"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TrackerMetadata", func() {
	It("Env", func() {
		Expect(TrackerMetadata{
			ExternalURL:  "https://www.example.com",
			PipelineName: "some-pipeline-name",
			ResourceName: "some-resource-name",
		}.Env()).To(Equal([]string{
			"ATC_EXTERNAL_URL=https://www.example.com",
			"RESOURCE_PIPELINE_NAME=some-pipeline-name",
			"RESOURCE_NAME=some-resource-name",
		}))
	})

	It("does not include fields that are not set", func() {
		Expect(TrackerMetadata{
			PipelineName: "some-pipeline-name",
		}.Env()).To(Equal([]string{
			"RESOURCE_PIPELINE_NAME=some-pipeline-name",
		}))
	})
})
