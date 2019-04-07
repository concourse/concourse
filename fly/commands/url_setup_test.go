package commands

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("URL parsing", func() {
	BeforeEach(func() {
		os.Setenv("TEAM", "")
		os.Setenv("PIPELINE", "")
		os.Setenv("JOB", "")
		os.Setenv("BUILD", "")
		os.Setenv("RESOURCE", "")
		os.Setenv("RESOURCE_TYPE", "")
	})

	It("set all environment variables for a pipeline", func() {
		setup := UrlSetupOptions{"https://localhost:8080/teams/main/pipelines/my_pipeline"}

		setup.SetInEnvironment()
		Expect(os.Getenv("TEAM")).To(Equal("main"))
		Expect(os.Getenv("PIPELINE")).To(Equal("my_pipeline"))
		Expect(os.Getenv("JOB")).To(Equal(""))
		Expect(os.Getenv("BUILD")).To(Equal(""))
		Expect(os.Getenv("RESOURCE")).To(Equal(""))
		Expect(os.Getenv("RESOURCE_TYPE")).To(Equal(""))
	})

	It("set all environment variables for a job", func() {
		setup := UrlSetupOptions{"https://localhost:8080/teams/main/pipelines/my_pipeline/jobs/my_job"}

		setup.SetInEnvironment()
		Expect(os.Getenv("TEAM")).To(Equal("main"))
		Expect(os.Getenv("PIPELINE")).To(Equal("my_pipeline"))
		Expect(os.Getenv("JOB")).To(Equal("my_pipeline/my_job"))
		Expect(os.Getenv("BUILD")).To(Equal(""))
		Expect(os.Getenv("RESOURCE")).To(Equal(""))
		Expect(os.Getenv("RESOURCE_TYPE")).To(Equal(""))
	})

	It("set all environment variables for a build", func() {
		setup := UrlSetupOptions{"https://localhost:8080/teams/main/pipelines/my_pipeline/jobs/my_job/builds/1"}

		setup.SetInEnvironment()
		Expect(os.Getenv("TEAM")).To(Equal("main"))
		Expect(os.Getenv("PIPELINE")).To(Equal("my_pipeline"))
		Expect(os.Getenv("JOB")).To(Equal("my_pipeline/my_job"))
		Expect(os.Getenv("BUILD")).To(Equal("1"))
		Expect(os.Getenv("RESOURCE")).To(Equal(""))
		Expect(os.Getenv("RESOURCE_TYPE")).To(Equal(""))
	})

	It("set all environment variables for a resource", func() {
		setup := UrlSetupOptions{"https://localhost:8080/teams/main/pipelines/my_pipeline/resources/my_resource"}

		setup.SetInEnvironment()
		Expect(os.Getenv("TEAM")).To(Equal("main"))
		Expect(os.Getenv("PIPELINE")).To(Equal("my_pipeline"))
		Expect(os.Getenv("JOB")).To(Equal(""))
		Expect(os.Getenv("BUILD")).To(Equal(""))
		Expect(os.Getenv("RESOURCE")).To(Equal("my_pipeline/my_resource"))
		Expect(os.Getenv("RESOURCE_TYPE")).To(Equal(""))
	})

	It("set all environment variables for a resource", func() {
		setup := UrlSetupOptions{"https://localhost:8080/teams/main/pipelines/my_pipeline/resource-types/my_resource_type"}

		setup.SetInEnvironment()
		Expect(os.Getenv("TEAM")).To(Equal("main"))
		Expect(os.Getenv("PIPELINE")).To(Equal("my_pipeline"))
		Expect(os.Getenv("JOB")).To(Equal(""))
		Expect(os.Getenv("BUILD")).To(Equal(""))
		Expect(os.Getenv("RESOURCE")).To(Equal(""))
		Expect(os.Getenv("RESOURCE_TYPE")).To(Equal("my_pipeline/my_resource_type"))
	})

	It("set all environment variables for a build by direct url", func() {
		setup := UrlSetupOptions{"https://localhost:8080/builds/1"}

		setup.SetInEnvironment()
		Expect(os.Getenv("TEAM")).To(Equal(""))
		Expect(os.Getenv("PIPELINE")).To(Equal(""))
		Expect(os.Getenv("JOB")).To(Equal(""))
		Expect(os.Getenv("BUILD")).To(Equal("1"))
		Expect(os.Getenv("RESOURCE")).To(Equal(""))
		Expect(os.Getenv("RESOURCE_TYPE")).To(Equal(""))
	})

})
