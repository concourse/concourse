package web_test

import (
	"fmt"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/testflight/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("the quality of being unauthenticated for public pipelines", func() {
	BeforeEach(func() {
		noAuth, _, _, err := helpers.GetAuthMethods(atcURL)
		Expect(err).ToNot(HaveOccurred())
		if noAuth {
			Skip("No auth methods enabled; skipping unauthenticated tests")
		}

		publiclyViewable, err := helpers.PipelinesPubliclyViewable()
		if !publiclyViewable {
			Skip("Pipelines are not publicly viewable; skipping unauthenticated tests for public pipelines")
		}
	})

	DescribeTable("trying to view pages unauthenticated displays the page",
		func(uri func() string) {
			url := urljoiner.Join(atcURL, uri())
			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page).Should(HaveURL(url))
		},

		Entry("index", func() string { return "/" }),

		Entry("job page (publicly viewable)", func() string {
			return fmt.Sprintf("/pipelines/%s/jobs/%s", pipelineName, publicBuild.JobName)
		}),
		Entry("build page (publicly viewable)", func() string {
			return fmt.Sprintf("/pipelines/%s/jobs/%s/builds/%s", pipelineName, publicBuild.JobName, publicBuild.Name)
		}),

		Entry("job page (private)", func() string {
			return fmt.Sprintf("/pipelines/%s/jobs/%s", pipelineName, privateBuild.JobName)
		}),
	)

	DescribeTable("trying to view pages unauthenticated displays a login message",
		func(uri func() string) {
			url := urljoiner.Join(atcURL, uri())
			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page).Should(HaveURL(url))
			Eventually(page.Find("input[value='log in to view']")).Should(BeFound())
		},

		Entry("build page (private)", func() string {
			return fmt.Sprintf("/pipelines/%s/jobs/%s/builds/%s", pipelineName, privateBuild.JobName, privateBuild.Name)
		}),
	)
})
