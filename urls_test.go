package web_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
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

			path, err := web.PathFor(atc.EnableResourceVersion, versionedResource)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(path).Should(Equal("/api/v1/resources/resource-name/versions/123/enable"))
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

			path, err := web.PathFor(atc.DisableResourceVersion, versionedResource)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(path).Should(Equal("/api/v1/resources/resource-name/versions/123/disable"))
		})
	})
})
