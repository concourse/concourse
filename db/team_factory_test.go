package db_test

import (
	"encoding/json"

	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
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
		var team db.Team
		BeforeEach(func() {
			var err error
			team, err = teamFactory.CreateTeam(atcTeam)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates the correct team", func() {
			Expect(team.Name()).To(Equal(atcTeam.Name))
			Expect(team.BasicAuth().BasicAuthUsername).To(Equal(atcTeam.BasicAuth.BasicAuthUsername))
			err := bcrypt.CompareHashAndPassword([]byte(team.BasicAuth().BasicAuthPassword), []byte(atcTeam.BasicAuth.BasicAuthPassword))
			Expect(err).NotTo(HaveOccurred())
			Expect(team.Auth()).To(Equal(atcTeam.Auth))

			t, found, err := teamFactory.FindTeam(atcTeam.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(t.ID()).To(Equal(team.ID()))
		})
	})

	Describe("FindTeam", func() {
		var (
			team  db.Team
			found bool
		)

		JustBeforeEach(func() {
			var err error
			team, found, err = teamFactory.FindTeam("some-team")
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the team exists", func() {
			BeforeEach(func() {
				var err error
				_, err = teamFactory.CreateTeam(atcTeam)
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

	Describe("CreateDefaultTeamIfNotExists", func() {
		It("creates the default team", func() {
			t, found, err := teamFactory.FindTeam(atc.DefaultTeamName)
			Expect(err).NotTo(HaveOccurred())
			if found {
				Expect(t.Admin()).To(BeFalse())
			}

			team, err := teamFactory.CreateDefaultTeamIfNotExists()
			Expect(err).NotTo(HaveOccurred())
			Expect(team.Admin()).To(BeTrue())

			t, found, err = teamFactory.FindTeam(atc.DefaultTeamName)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(t.ID()).To(Equal(team.ID()))
		})

		Context("when the default team already exists", func() {
			It("does not duplicate the default team", func() {
				team, err := teamFactory.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())

				team2, err := teamFactory.CreateDefaultTeamIfNotExists()
				Expect(err).NotTo(HaveOccurred())

				Expect(team.ID()).To(Equal(team2.ID()))
			})
		})
	})

	Describe("GetTeams", func() {
		var (
			teams []db.Team
		)

		BeforeEach(func() {
			err := defaultTeam.Delete()
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			teams, err = teamFactory.GetTeams()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there is one team", func() {
			BeforeEach(func() {
				var err error
				_, err = teamFactory.CreateTeam(atcTeam)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the team", func() {
				Expect(teams).To(HaveLen(1))

				Expect(teams[0].Name()).To(Equal(atcTeam.Name))
				Expect(teams[0].BasicAuth().BasicAuthUsername).To(Equal(atcTeam.BasicAuth.BasicAuthUsername))
				err := bcrypt.CompareHashAndPassword([]byte(teams[0].BasicAuth().BasicAuthPassword), []byte(atcTeam.BasicAuth.BasicAuthPassword))
				Expect(err).ToNot(HaveOccurred())
				Expect(teams[0].Auth()).To(Equal(atcTeam.Auth))
			})
		})

		Context("when there is more than one team", func() {
			BeforeEach(func() {
				var err error
				_, err = teamFactory.CreateTeam(atcTeam)
				Expect(err).ToNot(HaveOccurred())
				_, err = teamFactory.CreateTeam(atc.Team{
					Name: "some-other-team",
					BasicAuth: &atc.BasicAuth{
						BasicAuthUsername: "boring-user",
						BasicAuthPassword: "boring-password",
					},
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns both teams", func() {
				Expect(teams).To(HaveLen(2))

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
	})
})
