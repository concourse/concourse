package integration_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Default Configuration", func() {

	Context("when adding a user to the main team config", func() {
		BeforeEach(func() {
			cmd.Auth.MainTeamFlags.LocalUsers = []string{"test"}
		})

		It("grants the user access to the main team", func() {
			client := login(atcURL, "test", "test")

			teams, err := client.ListTeams()
			Expect(err).NotTo(HaveOccurred())
			Expect(teams).To(HaveLen(1))
			Expect(teams[0].Name).To(Equal("main"))
		})
	})

	It("X-Frame-Options header prevents clickjacking by default", func() {
		resp, err := http.Get(atcURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Header.Get("x-frame-options")).To(Equal("deny"))
	})
})
