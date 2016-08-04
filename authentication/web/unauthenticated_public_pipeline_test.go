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
		func(options func() (string, []string)) {
			uri, clicks := options()
			url := urljoiner.Join(atcURL, uri)
			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page).Should(HaveURL(url))

			if len(clicks) > 0 {
				By("displays login prompt when clicking on link/button")
				for _, selector := range clicks {
					Eventually(page.Find(selector)).Should(BeEnabled())
					Expect(page.Find(selector).Click()).To(Succeed())
					Eventually(page).ShouldNot(HaveURL(url))
					Expect(page.Title()).To(ContainSubstring("Log In"))

					//Return back to previous page
					Expect(page.Navigate(url)).To(Succeed())
				}
			}
		},

		Entry("index", func() (string, []string) { return "/", []string{} }),

		Entry("job page (publicly viewable)", func() (string, []string) {
			return fmt.Sprintf("/teams/%s/pipelines/%s/jobs/%s", teamName, pipelineName, publicBuild.JobName),
				[]string{"#page-header .build-header button.btn-pause"}
		}),

		Entry("build page (publicly viewable)", func() (string, []string) {
			return fmt.Sprintf("/teams/%s/pipelines/%s/jobs/%s/builds/%s", teamName, pipelineName, publicBuild.JobName, publicBuild.Name),
				[]string{
					"#page-header .build-header button.build-action",   //New Build
					"#page-header .build-header .build-action-abort i", //Abort Build
				}
		}),

		Entry("job page (private)", func() (string, []string) {
			return fmt.Sprintf("/teams/%s/pipelines/%s/jobs/%s", pipelineName, privateBuild.JobName), []string{}
		}),

		Entry("resource page", func() (string, []string) {
			return fmt.Sprintf("/teams/%s/pipelines/%s/resources/%s", pipelineName, brokenResource.Name),
				[]string{".build-step .header span.btn-pause"}
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
			return fmt.Sprintf("/teams/%s/pipelines/%s/jobs/%s/builds/%s", pipelineName, privateBuild.JobName, privateBuild.Name)
		}),
	)
})
