package dbng_test

import (
	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Team Factory", func() {
	var (
		atcTeam atc.Team
	)

	BeforeEach(func() {
		atcTeam = atc.Team{
			ID:   0,
			Name: "some-team",
			BasicAuth: &atc.BasicAuth{
				BasicAuthUsername: "hello",
				BasicAuthPassword: "people",
			},
		}
	})

	Describe("CreateTeam", func() {
		BeforeEach(func() {
			team, err := teamFactory.CreateTeam(atcTeam)
			Expect(err).ToNot(HaveOccurred())
		})

	})
})
