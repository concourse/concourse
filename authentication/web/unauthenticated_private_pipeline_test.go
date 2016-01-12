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

var _ = Describe("the quality of being unauthenticated for private pipelines", func() {
	var loginURL = urljoiner.Join(atcURL, "login")

	BeforeEach(func() {
		noAuth, _, _, err := helpers.GetAuthMethods(atcURL)
		Expect(err).ToNot(HaveOccurred())
		if noAuth {
			Skip("No auth methods enabled; skipping unauthenticated tests")
		}

		publiclyViewable, err := helpers.PipelinesPubliclyViewable()
		if publiclyViewable {
			Skip("Pipelines are publicly viewable; skipping unauthenticated tests for private pipelines")
		}
	})

	DescribeTable("trying to view pages unauthenticated prompts for login",
		func(uri func() string) {
			url := urljoiner.Join(atcURL, uri())
			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page).Should(HaveURL(loginURL))
			Eventually(page.Find(".login-box")).Should(MatchText("Log in with"))
		},

		Entry("index", func() string { return "/" }),

		Entry("job page (publicly viewable)", func() string { return fmt.Sprintf("/pipelines/%s/jobs/%s", pipelineName, publicBuild.JobName) }),
		Entry("build page (publicly viewable)", func() string {
			return fmt.Sprintf("/pipelines/%s/jobs/%s/builds/%s", pipelineName, publicBuild.JobName, publicBuild.Name)
		}),

		Entry("job page (private)", func() string { return fmt.Sprintf("/pipelines/%s/jobs/%s", pipelineName, privateBuild.JobName) }),
		Entry("build page (private)", func() string {
			return fmt.Sprintf("/pipelines/%s/jobs/%s/builds/%s", pipelineName, privateBuild.JobName, privateBuild.Name)
		}),
	)
})
