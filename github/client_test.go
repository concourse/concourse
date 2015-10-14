package github_test

import (
	"os"

	"github.com/concourse/atc/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("github", func() {
	var client github.Client

	BeforeEach(func() {
		client = github.NewClient(lagertest.NewTestLogger("github-client"))
	})

	Context("with a valid token", func() {
		var token string
		BeforeEach(func() {
			token = os.Getenv("GITHUB_ACCESS_TOKEN")
			if token == "" {
				Skip("Set a GITHUB_ACCESS_TOKEN envrionment variable to run these tests")
			}
		})

		It("can return a list of organizations the user belongs to", func() {
			orgs, err := client.GetOrganizations(token)
			Expect(err).NotTo(HaveOccurred())
			Expect(orgs).To(HaveLen(1))
			Expect(orgs[0]).To(Equal("ConcourseGitHubAuthTestOrg"))
		})
	})

	Context("without a valid token", func() {
		It("returns an error when trying to fetch organizations given an invalid token", func() {
			_, err := client.GetOrganizations("nope")
			Expect(err).To(HaveOccurred())
		})
	})
})
