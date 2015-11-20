package webhandler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/webhandler"
	"github.com/concourse/go-concourse/concourse"
)

var _ = Describe("URLs", func() {
	Describe("EnableVersionResource", func() {
		It("returns the correct URL", func() {
			versionedResource := atc.VersionedResource{
				ID:       123,
				Resource: "resource-name",
			}

			path, err := webhandler.PathFor(atc.EnableResourceVersion, "some-pipeline", versionedResource)
			Expect(err).NotTo(HaveOccurred())

			Expect(path).To(Equal("/api/v1/pipelines/some-pipeline/resources/resource-name/versions/123/enable"))
		})
	})

	Describe("DisableVersionResource", func() {
		It("returns the correct URL", func() {
			versionedResource := atc.VersionedResource{
				ID:       123,
				Resource: "resource-name",
			}

			path, err := webhandler.PathFor(atc.DisableResourceVersion, "some-pipeline", versionedResource)
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

				path, err := webhandler.PathFor(web.GetJob, "another-pipeline", job)
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(Equal("/pipelines/another-pipeline/jobs/some-job"))
			})
		})

		Context("with pagination data", func() {
			It("returns the correct URL", func() {
				job := atc.JobConfig{
					Name: "some-job",
				}

				path, err := webhandler.PathFor(web.GetJob, "another-pipeline", job, &concourse.Page{Since: 123, Limit: 456})
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(Equal("/pipelines/another-pipeline/jobs/some-job?since=123"))

				path, err = webhandler.PathFor(web.GetJob, "another-pipeline", job, &concourse.Page{Until: 123, Limit: 456})
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(Equal("/pipelines/another-pipeline/jobs/some-job?until=123"))
			})

		})
	})

	Describe("Resources Path", func() {
		Context("with pagination data", func() {
			It("returns the correct URL", func() {
				path, err := webhandler.PathFor(web.GetResource, "another-pipeline", "some-resource", &concourse.Page{Since: 123, Limit: 456})
				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(Equal("/pipelines/another-pipeline/resources/some-resource?since=123"))

				path, err = webhandler.PathFor(web.GetResource, "another-pipeline", "some-resource", &concourse.Page{Until: 123, Limit: 456})
				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(Equal("/pipelines/another-pipeline/resources/some-resource?until=123"))
			})
		})
	})

	Describe("OAuth Begin", func() {
		It("links to the provider with a redirect to the index", func() {
			path, err := webhandler.PathFor(auth.OAuthBegin, "some-provider", "/some/path")
			Expect(err).NotTo(HaveOccurred())

			Expect(path).To(Equal("/auth/some-provider?redirect=%2Fsome%2Fpath"))
		})
	})

	Describe("Basic Auth", func() {
		It("links to the provider with a redirect to the index", func() {
			path, err := webhandler.PathFor(web.BasicAuth, "/some/path")
			Expect(err).NotTo(HaveOccurred())

			Expect(path).To(Equal("/login/basic?redirect=%2Fsome%2Fpath"))
		})
	})
})
