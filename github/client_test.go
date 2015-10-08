package github_test

import (
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

	Context("GetOrganizations", func() {
		It("returns a list of organizations given a valid access token", func() {
			orgs, err := client.GetOrganizations("37f6d08a745cd5b4a2a72733654bb16c3b3c1b24")
			Expect(err).NotTo(HaveOccurred())
			Expect(orgs).To(HaveLen(1))
			Expect(orgs[0]).To(Equal("ConcourseGitHubAuthTestOrg"))
		})

		It("returns an error given an invalid token", func() {
			_, err := client.GetOrganizations("nope")
			Expect(err).To(HaveOccurred())
		})
	})
})
