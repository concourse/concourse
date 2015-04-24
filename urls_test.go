package web_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/routes"
)

func TestURLs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "URLs Suite")
}

var _ = Describe("URLs", func() {
	Describe("EnableVersionResource", func() {
		It("returns the correct URL", func() {
			versionedResource := db.SavedVersionedResource{
				ID: 123,
				VersionedResource: db.VersionedResource{
					Resource: "resource-name",
				},
			}

			path, err := web.PathFor(atc.EnableResourceVersion, "some-pipeline", versionedResource)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(path).Should(Equal("/api/v1/pipelines/some-pipeline/resources/resource-name/versions/123/enable"))
		})
	})

	Describe("DisableVersionResource", func() {
		It("returns the correct URL", func() {
			versionedResource := db.SavedVersionedResource{
				ID: 123,
				VersionedResource: db.VersionedResource{
					Resource: "resource-name",
				},
			}

			path, err := web.PathFor(atc.DisableResourceVersion, "some-pipeline", versionedResource)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(path).Should(Equal("/api/v1/pipelines/some-pipeline/resources/resource-name/versions/123/disable"))
		})
	})

	Describe("Jobs Patch", func() {
		It("returns the correct URL", func() {
			job := atc.JobConfig{
				Name: "some-job",
			}

			path, err := web.PathFor(routes.GetJob, "another-pipeline", job)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(path).Should(Equal("/pipelines/another-pipeline/jobs/some-job"))
		})
	})
})
