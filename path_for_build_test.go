package web_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/web"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Routes", func() {
	Describe("PathForBuild", func() {
		It("returns the canonical path for a jobless build", func() {
			joblessBuild := atc.Build{
				ID:           1,
				TeamName:     "some-team",
				Name:         "23",
				PipelineName: "a-pipeline",
			}

			path := web.PathForBuild(joblessBuild)
			Expect(path).To(Equal("/builds/1"))
		})

		It("returns the canonical path for a job-filled build", func() {
			build := atc.Build{
				ID:           1,
				TeamName:     "some-team",
				JobName:      "hello",
				Name:         "23",
				PipelineName: "a-pipeline",
			}

			path := web.PathForBuild(build)
			Expect(path).To(Equal("/teams/some-team/pipelines/a-pipeline/jobs/hello/builds/23"))
		})
	})
})
