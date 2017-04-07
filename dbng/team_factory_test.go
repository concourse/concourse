package dbng_test

import (
	"encoding/json"

	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Team Factory", func() {
	var (
		atcTeam atc.Team
	)

	BeforeEach(func() {
		data := []byte(`{"foo":"bar"}`)
		atcTeam = atc.Team{
			Name: "some-team",
			BasicAuth: &atc.BasicAuth{
				BasicAuthUsername: "hello",
				BasicAuthPassword: "people",
			},
			Auth: map[string]*json.RawMessage{
				"fake-provider": (*json.RawMessage)(&data),
			},
		}
	})

	Describe("CreateTeam", func() {
		var team dbng.Team
		BeforeEach(func() {
			team, err = teamFactory.CreateTeam(atcTeam)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates the correct team", func() {
			Expect(team.Name()).To(Equal(atcTeam.Name))
			Expect(team.BasicAuth().BasicAuthUsername).To(Equal(atcTeam.BasicAuth.BasicAuthUsername))
			err := bcrypt.CompareHashAndPassword([]byte(team.BasicAuth().BasicAuthPassword), []byte(atcTeam.BasicAuth.BasicAuthPassword))
			Expect(err).ToNot(HaveOccurred())
			Expect(team.Auth()).To(Equal(atcTeam.Auth))
		})
	})

	Describe("FindTeam", func() {
		var (
			team  dbng.Team
			found bool
		)

		JustBeforeEach(func() {
			team, found, err = teamFactory.FindTeam("some-team")
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the team exists", func() {
			var createdTeam dbng.Team
			BeforeEach(func() {
				createdTeam, err = teamFactory.CreateTeam(atcTeam)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finds the correct team", func() {
				Expect(team.Name()).To(Equal(atcTeam.Name))
				Expect(team.BasicAuth().BasicAuthUsername).To(Equal(atcTeam.BasicAuth.BasicAuthUsername))
				err := bcrypt.CompareHashAndPassword([]byte(team.BasicAuth().BasicAuthPassword), []byte(atcTeam.BasicAuth.BasicAuthPassword))
				Expect(err).ToNot(HaveOccurred())
				Expect(team.Auth()).To(Equal(atcTeam.Auth))
			})
		})

		Context("when the team does not exist", func() {
			It("returns not found", func() {
				Expect(team).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("FindTeams", func() {
		var (
			teams []dbng.Team
		)

		BeforeEach(func() {
			err := defaultTeam.Delete()
			Expect(err).ToNot(HaveOccurred())
			mainTeam, _, _ := teamFactory.FindTeam("main")
			err = mainTeam.Delete()
		})

		JustBeforeEach(func() {
			teams, err = teamFactory.FindTeams()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there is one team", func() {
			var createdTeam dbng.Team
			BeforeEach(func() {
				createdTeam, err = teamFactory.CreateTeam(atcTeam)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the team", func() {
				Expect(teams[0].Name()).To(Equal(atcTeam.Name))
				Expect(teams[0].BasicAuth().BasicAuthUsername).To(Equal(atcTeam.BasicAuth.BasicAuthUsername))
				err := bcrypt.CompareHashAndPassword([]byte(teams[0].BasicAuth().BasicAuthPassword), []byte(atcTeam.BasicAuth.BasicAuthPassword))
				Expect(err).ToNot(HaveOccurred())
				Expect(teams[0].Auth()).To(Equal(atcTeam.Auth))
			})
		})

		Context("when there are more than one team", func() {
			var (
				createdTeam      dbng.Team
				otherCreatedTeam dbng.Team
			)
			BeforeEach(func() {
				createdTeam, err = teamFactory.CreateTeam(atcTeam)
				Expect(err).ToNot(HaveOccurred())
				otherCreatedTeam, err = teamFactory.CreateTeam(atc.Team{
					Name: "some-other-team",
					BasicAuth: &atc.BasicAuth{
						BasicAuthUsername: "boring-user",
						BasicAuthPassword: "boring-password",
					},
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns both teams", func() {
				Expect(teams[0].Name()).To(Equal(atcTeam.Name))
				Expect(teams[0].BasicAuth().BasicAuthUsername).To(Equal(atcTeam.BasicAuth.BasicAuthUsername))
				err := bcrypt.CompareHashAndPassword([]byte(teams[0].BasicAuth().BasicAuthPassword), []byte(atcTeam.BasicAuth.BasicAuthPassword))
				Expect(err).ToNot(HaveOccurred())
				Expect(teams[0].Auth()).To(Equal(atcTeam.Auth))

				Expect(teams[1].Name()).To(Equal("some-other-team"))
				Expect(teams[1].BasicAuth().BasicAuthUsername).To(Equal("boring-user"))
				err = bcrypt.CompareHashAndPassword([]byte(teams[1].BasicAuth().BasicAuthPassword), []byte("boring-password"))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when there are no teams", func() {
			It("returns nil", func() {
				Expect(teams).To(Equal([]dbng.Team{}))
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
