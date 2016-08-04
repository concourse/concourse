package web_test

import (
	"fmt"

	"code.cloudfoundry.org/gunk/urljoiner"
	"github.com/concourse/testflight/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("the quality of having no authentication", func() {
	BeforeEach(func() {
		noAuth, _, _, err := helpers.GetAuthMethods(atcURL)
		Expect(err).ToNot(HaveOccurred())
		if !noAuth {
			Skip("Auth methods enabled; skipping no authentication tests")
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
			return fmt.Sprintf("/teams/%s/pipelines/%s/jobs/%s", teamName, pipelineName, publicBuild.JobName)
		}),
		Entry("build page (publicly viewable)", func() string {
			return fmt.Sprintf("/teams/%s/pipelines/%s/jobs/%s/builds/%s", teamName, pipelineName, publicBuild.JobName, publicBuild.Name)
		}),

		Entry("job page (private)", func() string {
			return fmt.Sprintf("/teams/%s/pipelines/%s/jobs/%s", teamName, pipelineName, privateBuild.JobName)
		}),
	)

	DescribeTable("trying to view pages unauthenticated displays the page with no login prompt",
		func(uri func() string) {
			url := urljoiner.Join(atcURL, uri())
			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page).Should(HaveURL(url))
			Eventually(page.Find("input[value='log in to view']")).ShouldNot(BeFound())
		},

		Entry("build page (private)", func() string {
			return fmt.Sprintf("/teams/%s/pipelines/%s/jobs/%s/builds/%s", teamName, pipelineName, privateBuild.JobName, privateBuild.Name)
		}),
	)
})
