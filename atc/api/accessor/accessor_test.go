package accessor_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
)

var _ = Describe("Accessor", func() {
	var (
		verification accessor.Verification
		requiredRole string
		teams        []db.Team
		access       accessor.Access

		fakeTeam1 *dbfakes.FakeTeam
		fakeTeam2 *dbfakes.FakeTeam
		fakeTeam3 *dbfakes.FakeTeam
	)

	BeforeEach(func() {
		fakeTeam1 = new(dbfakes.FakeTeam)
		fakeTeam1.NameReturns("some-team-1")
		fakeTeam2 = new(dbfakes.FakeTeam)
		fakeTeam2.NameReturns("some-team-2")
		fakeTeam3 = new(dbfakes.FakeTeam)
		fakeTeam3.NameReturns("some-team-3")

		verification = accessor.Verification{}

		teams = []db.Team{fakeTeam1, fakeTeam2, fakeTeam3}
	})

	JustBeforeEach(func() {
		access = accessor.NewAccessor(verification, requiredRole, "sub", []string{"system"}, teams)
	})

	Describe("HasToken", func() {
		var result bool

		JustBeforeEach(func() {
			result = access.HasToken()
		})

		Context("when verification does not have a token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when verification has a token", func() {
			BeforeEach(func() {
				verification.HasToken = true
			})

			It("returns true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("IsAuthenticated", func() {
		var result bool

		JustBeforeEach(func() {
			result = access.IsAuthenticated()
		})

		Context("when verification does not have a token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when verification has a token", func() {
			BeforeEach(func() {
				verification.HasToken = true
			})

			Context("when verification token is not valid", func() {
				BeforeEach(func() {
					verification.IsTokenValid = false
				})

				It("returns false", func() {
					Expect(result).To(BeFalse())
				})
			})

			Context("when verification token is valid", func() {
				BeforeEach(func() {
					verification.IsTokenValid = true
				})

				It("returns true", func() {
					Expect(result).To(BeTrue())
				})
			})
		})
	})

	Describe("IsAuthorized", func() {
		var result bool

		JustBeforeEach(func() {
			result = access.IsAuthorized("some-team")
		})

		Context("when the user has no token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when the user has invalid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = false
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when the user has valid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.Claims = map[string]interface{}{
					"federated_claims": map[string]interface{}{
						"connector_id": "some-connector",
						"user_id":      "some-user-id",
					},
				}
			})

			Context("when the user is part of any admin team", func() {
				BeforeEach(func() {
					fakeTeam1.NameReturns("not-some-team")
					fakeTeam1.AdminReturns(true)
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				It("returns true", func() {
					Expect(result).To(BeTrue())
				})
			})

			Context("the team has the user configured as a 'member'", func() {

				BeforeEach(func() {
					fakeTeam1.NameReturns("some-team")
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				Context("when the action requires a 'viewer' role", func() {
					BeforeEach(func() {
						requiredRole = "viewer"
					})

					It("returns true", func() {
						Expect(result).To(BeTrue())
					})
				})

				Context("when the action requires a 'member' role", func() {
					BeforeEach(func() {
						requiredRole = "member"
					})

					It("returns true", func() {
						Expect(result).To(BeTrue())
					})
				})

				Context("when the action requires an 'owner' role", func() {
					BeforeEach(func() {
						requiredRole = "owner"
					})

					It("returns false", func() {
						Expect(result).To(BeFalse())
					})
				})
			})
		})
	})

	Describe("TeamNames", func() {
		var result []string

		JustBeforeEach(func() {
			result = access.TeamNames()
		})

		Context("when the user has no token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns nothing", func() {
				Expect(result).To(BeEmpty())
			})
		})

		Context("when the user has invalid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = false
			})

			It("returns nothing", func() {
				Expect(result).To(BeEmpty())
			})
		})

		Context("when the user has valid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.Claims = map[string]interface{}{
					"federated_claims": map[string]interface{}{
						"connector_id": "some-connector",
						"user_id":      "some-user-id",
					},
				}
			})

			Context("when the user is part of any admin team", func() {
				BeforeEach(func() {
					fakeTeam1.AdminReturns(true)
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				It("returns all teams", func() {
					Expect(result).To(ConsistOf(
						"some-team-1",
						"some-team-2",
						"some-team-3",
					))
				})
			})

			Context("the team has the user configured", func() {

				BeforeEach(func() {
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
					fakeTeam2.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
					fakeTeam3.AuthReturns(atc.TeamAuth{
						"viewer": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				Context("when the action requires a 'viewer' role", func() {
					BeforeEach(func() {
						requiredRole = "viewer"
					})

					It("returns all teams", func() {
						Expect(result).To(ConsistOf(
							"some-team-1",
							"some-team-2",
							"some-team-3",
						))
					})
				})

				Context("when the action requires a 'member' role", func() {
					BeforeEach(func() {
						requiredRole = "member"
					})

					It("returns all teams", func() {
						Expect(result).To(ConsistOf(
							"some-team-1",
							"some-team-2",
						))
					})
				})

				Context("when the action requires an 'owner' role", func() {
					BeforeEach(func() {
						requiredRole = "owner"
					})

					It("returns all teams", func() {
						Expect(result).To(ConsistOf(
							"some-team-1",
						))
					})
				})
			})
		})
	})

	Describe("IsAdmin", func() {
		var result bool

		JustBeforeEach(func() {
			result = access.IsAdmin()
		})

		Context("when the user has no token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when the user has invalid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = false
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when the user has valid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.Claims = map[string]interface{}{
					"federated_claims": map[string]interface{}{
						"connector_id": "some-connector",
						"user_id":      "some-user-id",
					},
				}
			})

			Context("when the user is a not on an admin team", func() {
				BeforeEach(func() {
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"viewer": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
					fakeTeam2.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
					fakeTeam3.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				It("returns false", func() {
					Expect(result).To(BeFalse())
				})
			})

			Context("when the user is a 'viewer' on an admin team", func() {
				BeforeEach(func() {
					fakeTeam1.AdminReturns(true)
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"viewer": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				It("returns false", func() {
					Expect(result).To(BeFalse())
				})
			})

			Context("when the user is a 'member' on an admin team", func() {
				BeforeEach(func() {
					fakeTeam1.AdminReturns(true)
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				It("returns false", func() {
					Expect(result).To(BeFalse())
				})
			})

			Context("when the user is a 'owner' on an admin team", func() {
				BeforeEach(func() {
					fakeTeam1.AdminReturns(true)
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				It("returns true", func() {
					Expect(result).To(BeTrue())
				})
			})
		})
	})

	Describe("IsSystem", func() {
		var result bool

		JustBeforeEach(func() {
			result = access.IsSystem()
		})

		Context("when the user has no token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when the user has invalid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = false
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when the token does not have the system sub", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.Claims = map[string]interface{}{
					"sub": "not-system",
				}
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})
		})

		Context("when the token does have the system sub", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.Claims = map[string]interface{}{
					"sub": "system",
				}
			})

			It("returns true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("UserName", func() {
		var result string

		JustBeforeEach(func() {
			result = access.UserName()
		})

		Context("when the user has no token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns empty", func() {
				Expect(result).To(BeEmpty())
			})
		})

		Context("when the user has invalid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = false
			})

			It("returns empty", func() {
				Expect(result).To(BeEmpty())
			})
		})

		Context("when the token has a federated user_name", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.Claims = map[string]interface{}{
					"federated_claims": map[string]interface{}{
						"user_name": "some-user-name",
					},
				}
			})

			It("returns the result", func() {
				Expect(result).To(Equal("some-user-name"))
			})
		})
	})

	Describe("UserInfo", func() {
		var result accessor.UserInfo

		JustBeforeEach(func() {
			result = access.UserInfo()
		})

		Context("when the user has no token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns empty", func() {
				Expect(result).To(Equal(accessor.UserInfo{
					Teams: map[string][]string{},
				}))
			})
		})

		Context("when the user has invalid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = false
			})

			It("returns empty", func() {
				Expect(result).To(Equal(accessor.UserInfo{
					Teams: map[string][]string{},
				}))
			})
		})

		Context("when the token has claims", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.Claims = map[string]interface{}{
					"sub":   "some-sub",
					"name":  "some-name",
					"email": "some-email",
					"federated_claims": map[string]interface{}{
						"connector_id": "some-connector",
						"user_id":      "some-user-id",
						"user_name":    "some-user-name",
					},
				}
			})

			Context("when the user is not part of any teams", func() {
				It("returns result without teams", func() {
					Expect(result).To(Equal(accessor.UserInfo{
						Sub:      "some-sub",
						Name:     "some-name",
						Email:    "some-email",
						UserID:   "some-user-id",
						UserName: "some-user-name",
						IsAdmin:  false,
						IsSystem: false,
						Teams:    map[string][]string{},
					}))
				})
			})

			Context("when the user is part of teams", func() {
				BeforeEach(func() {
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
					fakeTeam2.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
					fakeTeam3.AuthReturns(atc.TeamAuth{
						"viewer": map[string][]string{
							"users": []string{"some-connector:some-user-id"},
						},
					})
				})

				It("returns result with teams", func() {
					Expect(result).To(Equal(accessor.UserInfo{
						Sub:      "some-sub",
						Name:     "some-name",
						Email:    "some-email",
						UserID:   "some-user-id",
						UserName: "some-user-name",
						IsAdmin:  false,
						IsSystem: false,
						Teams: map[string][]string{
							"some-team-1": []string{"owner"},
							"some-team-2": []string{"member"},
							"some-team-3": []string{"viewer"},
						},
					}))
				})
			})
		})
	})
})
