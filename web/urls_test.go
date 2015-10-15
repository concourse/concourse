package web_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/pagination"
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
			Expect(err).NotTo(HaveOccurred())

			Expect(path).To(Equal("/api/v1/pipelines/some-pipeline/resources/resource-name/versions/123/enable"))
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
			Expect(err).NotTo(HaveOccurred())

			Expect(path).To(Equal("/api/v1/pipelines/some-pipeline/resources/resource-name/versions/123/disable"))
		})
	})

	Describe("Jobs Patch", func() {
		Context("without pagination data", func() {
			It("returns the correct URL", func() {
				job := atc.JobConfig{
					Name: "some-job",
				}

				path, err := web.PathFor(routes.GetJob, "another-pipeline", job)
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(Equal("/pipelines/another-pipeline/jobs/some-job"))
			})
		})

		Context("with pagination data", func() {
			It("returns the correct URL", func() {
				paginationData := pagination.NewPaginationData(false, false, 0, 29, 21)
				job := atc.JobConfig{
					Name: "some-job",
				}

				path, err := web.PathFor(routes.GetJob, "another-pipeline", job, paginationData, false)
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(Equal("/pipelines/another-pipeline/jobs/some-job?startingID=20&resultsGreaterThanStartingID=false"))

				path, err = web.PathFor(routes.GetJob, "another-pipeline", job, paginationData, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(Equal("/pipelines/another-pipeline/jobs/some-job?startingID=30&resultsGreaterThanStartingID=true"))
			})

		})
	})

	Describe("Resources Path", func() {
		Context("older links", func() {
			It("can generate them", func() {
				paginationData := pagination.NewPaginationData(false, false, 0, 29, 21)

				path, err := web.PathFor(routes.GetResource, "another-pipeline", "some-resource", paginationData, false)
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(Equal("/pipelines/another-pipeline/resources/some-resource?id=20&newer=false"))
			})
		})

		Context("newer links", func() {
			It("can generate them", func() {
				paginationData := pagination.NewPaginationData(false, false, 0, 29, 21)

				path, err := web.PathFor(routes.GetResource, "another-pipeline", "some-resource", paginationData, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(Equal("/pipelines/another-pipeline/resources/some-resource?id=30&newer=true"))
			})
		})
	})

	Describe("OAuth Begin", func() {
		It("links to the provider with a redirect to the index", func() {
			path, err := web.PathFor(auth.OAuthBegin, "some-provider")
			Expect(err).NotTo(HaveOccurred())

			Expect(path).To(Equal("/auth/some-provider?redirect=%2F"))
		})
	})
})
