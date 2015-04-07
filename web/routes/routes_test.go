package routes_test

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Routes", func() {
	Describe("BuildPath", func() {
		It("returns the canonical path for a jobless build", func() {
			joblessBuild := db.Build{
				ID:   1,
				Name: "23",
			}

			path := routes.PathForBuild(joblessBuild)
			Ω(path).Should(Equal("/builds/1"))
		})

		It("returns the canonical path for a job-filled build", func() {
			build := db.Build{
				ID:      1,
				JobName: "hello",
				Name:    "23",
			}

			path := routes.PathForBuild(build)
			Ω(path).Should(Equal("/jobs/hello/builds/23"))
		})
	})
})
