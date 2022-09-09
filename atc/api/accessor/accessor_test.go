package accessor_test

import (
	"github.com/concourse/concourse/atc/atcfakes"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/ginkgo/v2"
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

		fakeDisplayUserIdGenerator *atcfakes.FakeDisplayUserIdGenerator
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

		fakeDisplayUserIdGenerator = new(atcfakes.FakeDisplayUserIdGenerator)
	})

	JustBeforeEach(func() {
		access = accessor.NewAccessor(verification, requiredRole, "sub", []string{"system"}, teams, fakeDisplayUserIdGenerator)
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
				verification.RawClaims = map[string]interface{}{
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
							"users": {"some-connector:some-user-id"},
						},
					})
				})

				It("returns true", func() {
					Expect(result).To(BeTrue())
				})
			})
		})
	})

	DescribeTable("IsAuthorized for users",
		func(requiredRole string, actualRole string, expected bool) {

			verification.HasToken = true
			verification.IsTokenValid = true
			verification.RawClaims = map[string]interface{}{
				"federated_claims": map[string]interface{}{
					"connector_id": "some-connector",
					"user_id":      "some-user-id",
				},
			}

			fakeTeam1.NameReturns("some-team")
			fakeTeam1.AdminReturns(true)
			fakeTeam1.AuthReturns(atc.TeamAuth{
				actualRole: map[string][]string{
					"users": {"some-connector:some-user-id"},
				},
			})

			access = accessor.NewAccessor(verification, requiredRole, "sub", []string{"system"}, teams, fakeDisplayUserIdGenerator)
			result := access.IsAuthorized("some-team")
			Expect(expected).Should(Equal(result))
		},

		Entry("viewer attempting viewer action", "viewer", "viewer", true),
		Entry("pipeline-operator attempting viewer action", "viewer", "pipeline-operator", true),
		Entry("member attempting viewer action", "viewer", "member", true),
		Entry("owner attempting viewer action", "viewer", "owner", true),

		Entry("viewer attempting pipeline-operator action", "pipeline-operator", "viewer", false),
		Entry("pipeline-operator attempting pipeline-operator action", "pipeline-operator", "pipeline-operator", true),
		Entry("member attempting pipeline-operator action", "pipeline-operator", "member", true),
		Entry("owner attempting pipeline-operator action", "pipeline-operator", "owner", true),

		Entry("viewer attempting member action", "member", "viewer", false),
		Entry("pipeline-operator attempting member action", "member", "pipeline-operator", false),
		Entry("member attempting member action", "member", "member", true),
		Entry("owner attempting member action", "member", "owner", true),

		Entry("viewer attempting owner action", "owner", "viewer", false),
		Entry("pipeline-operator attempting owner action", "owner", "pipeline-operator", false),
		Entry("member attempting owner action", "owner", "member", false),
		Entry("owner attempting owner action", "owner", "owner", true),
	)

	DescribeTable("IsAuthorized for groups",
		func(requiredRole string, actualRole string, expected bool) {

			verification.HasToken = true
			verification.IsTokenValid = true

			verification.RawClaims = map[string]interface{}{
				"groups": []interface{}{"some-group"},
				"federated_claims": map[string]interface{}{
					"connector_id": "some-connector",
				},
			}

			fakeTeam1.NameReturns("some-team")
			fakeTeam1.AdminReturns(true)
			fakeTeam1.AuthReturns(atc.TeamAuth{
				actualRole: map[string][]string{
					"groups": {"some-connector:some-group"},
				},
			})

			access = accessor.NewAccessor(verification, requiredRole, "sub", []string{"system"}, teams, fakeDisplayUserIdGenerator)
			result := access.IsAuthorized("some-team")
			Expect(expected).Should(Equal(result))
		},

		Entry("viewer attempting viewer action", "viewer", "viewer", true),
		Entry("pipeline-operator attempting viewer action", "viewer", "pipeline-operator", true),
		Entry("member attempting viewer action", "viewer", "member", true),
		Entry("owner attempting viewer action", "viewer", "owner", true),

		Entry("viewer attempting pipeline-operator action", "pipeline-operator", "viewer", false),
		Entry("pipeline-operator attempting pipeline-operator action", "pipeline-operator", "pipeline-operator", true),
		Entry("member attempting pipeline-operator action", "pipeline-operator", "member", true),
		Entry("owner attempting pipeline-operator action", "pipeline-operator", "owner", true),

		Entry("viewer attempting member action", "member", "viewer", false),
		Entry("pipeline-operator attempting member action", "member", "pipeline-operator", false),
		Entry("member attempting member action", "member", "member", true),
		Entry("owner attempting member action", "member", "owner", true),

		Entry("viewer attempting owner action", "owner", "viewer", false),
		Entry("pipeline-operator attempting owner action", "owner", "pipeline-operator", false),
		Entry("member attempting owner action", "owner", "member", false),
		Entry("owner attempting owner action", "owner", "owner", true),
	)

	DescribeTable("IsAuthorized for both users and groups",
		func(requiredRole string, actualUserRole, actualGroupRole string, expected bool) {

			verification.HasToken = true
			verification.IsTokenValid = true

			verification.RawClaims = map[string]interface{}{
				"groups": []interface{}{"some-group"},
				"federated_claims": map[string]interface{}{
					"connector_id": "some-connector",
					"user_id":      "some-user-id",
				},
			}

			fakeTeam1.NameReturns("some-team")
			fakeTeam1.AdminReturns(true)

			if actualUserRole == actualGroupRole {
				fakeTeam1.AuthReturns(atc.TeamAuth{
					actualUserRole: map[string][]string{
						"users":  {"some-connector:some-user-id"},
						"groups": {"some-connector:some-group"},
					},
				})
			} else {
				fakeTeam1.AuthReturns(atc.TeamAuth{
					actualUserRole: map[string][]string{
						"users": {"some-connector:some-user-id"},
					},
					actualGroupRole: map[string][]string{
						"groups": {"some-connector:some-group"},
					},
				})
			}

			access = accessor.NewAccessor(verification, requiredRole, "sub", []string{"system"}, teams, fakeDisplayUserIdGenerator)
			result := access.IsAuthorized("some-team")
			Expect(expected).Should(Equal(result))
		},

		Entry("user is member and group is viewer attempting owner action", "owner", "member", "viewer", false),
		Entry("user is viewer and group is member attempting owner action", "owner", "viewer", "member", false),
		Entry("user is member and group is viewer attempting owner action", "owner", "member", "member", false),
		Entry("user is viewer and group is member attempting owner action", "owner", "viewer", "viewer", false),
		Entry("user is member and group is viewer attempting member action", "member", "member", "viewer", true),
		Entry("user is viewer and group is member attempting member action", "member", "viewer", "member", true),
		Entry("user is member and group is viewer attempting member action", "member", "member", "member", true),
		Entry("user is viewer and group is member attempting member action", "member", "viewer", "viewer", false),
		Entry("user is member and group is viewer attempting pipeline-operator action", "pipeline-operator", "member", "viewer", true),
		Entry("user is viewer and group is member attempting pipeline-operator action", "pipeline-operator", "viewer", "member", true),
		Entry("user is member and group is viewer attempting pipeline-operator action", "pipeline-operator", "member", "member", true),
		Entry("user is viewer and group is member attempting pipeline-operator action", "pipeline-operator", "viewer", "viewer", false),
		Entry("user is member and group is viewer attempting viewer action", "viewer", "member", "viewer", true),
		Entry("user is viewer and group is member attempting viewer action", "viewer", "viewer", "member", true),
		Entry("user is member and group is viewer attempting viewer action", "viewer", "member", "member", true),
		Entry("user is viewer and group is member attempting viewer action", "viewer", "viewer", "viewer", true),
	)

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
				verification.RawClaims = map[string]interface{}{
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
							"users": {"some-connector:some-user-id"},
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
							"users": {"some-connector:some-user-id"},
						},
					})
					fakeTeam2.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": {"some-connector:some-user-id"},
						},
					})
					fakeTeam3.AuthReturns(atc.TeamAuth{
						"viewer": map[string][]string{
							"users": {"some-connector:some-user-id"},
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
				verification.RawClaims = map[string]interface{}{
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
							"users": {"some-connector:some-user-id"},
						},
					})
					fakeTeam2.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": {"some-connector:some-user-id"},
						},
					})
					fakeTeam3.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": {"some-connector:some-user-id"},
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
							"users": {"some-connector:some-user-id"},
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
							"users": {"some-connector:some-user-id"},
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
				verification.RawClaims = map[string]interface{}{
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
				verification.RawClaims = map[string]interface{}{
					"sub": "system",
				}
			})

			It("returns true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("Claims", func() {
		var result accessor.Claims

		JustBeforeEach(func() {
			result = access.Claims()
		})

		Context("when the user has no token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns empty", func() {
				Expect(result).To(Equal(accessor.Claims{}))
			})
		})

		Context("when the user has invalid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = false
			})

			It("returns empty", func() {
				Expect(result).To(Equal(accessor.Claims{}))
			})
		})

		Context("when the token has a preferred user_name", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.RawClaims = map[string]interface{}{
					"sub":                "some-sub",
					"name":               "some-name",
					"preferred_username": "some-user-name",
					"email":              "some-email",
					"federated_claims": map[string]interface{}{
						"user_id":      "some-id",
						"connector_id": "some-connector",
					},
				}
			})

			It("returns the result", func() {
				Expect(result).To(Equal(accessor.Claims{
					Sub:               "some-sub",
					UserName:          "some-name",
					Email:             "some-email",
					UserID:            "some-id",
					PreferredUsername: "some-user-name",
					Connector:         "some-connector",
				}))
			})
		})
	})

	Describe("UserInfo", func() {
		var result atc.UserInfo

		BeforeEach(func() {
			fakeDisplayUserIdGenerator.DisplayUserIdReturns("some-user-id")
		})

		JustBeforeEach(func() {
			result = access.UserInfo()
		})

		Context("when there is a valid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.RawClaims = map[string]interface{}{
					"sub":                "some-sub",
					"name":               "some-name",
					"preferred_username": "some-user-name",
					"email":              "some-email",
					"federated_claims": map[string]interface{}{
						"user_id":      "some-id",
						"connector_id": "some-connector",
					},
				}
			})

			It("returns the result", func() {
				Expect(result).To(Equal(atc.UserInfo{
					Sub:           "some-sub",
					Name:          "some-name",
					Email:         "some-email",
					UserId:        "some-id",
					UserName:      "some-user-name",
					IsAdmin:       false,
					IsSystem:      false,
					Teams:         map[string][]string{},
					Connector:     "some-connector",
					DisplayUserId: "some-user-id",
				}))
			})
		})
	})

	Describe("TeamRoles", func() {
		var result map[string][]string

		JustBeforeEach(func() {
			result = access.TeamRoles()
		})

		Context("when the user has no token", func() {
			BeforeEach(func() {
				verification.HasToken = false
			})

			It("returns empty", func() {
				Expect(result).To(Equal(map[string][]string{}))
			})
		})

		Context("when the user has invalid token", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = false
			})

			It("returns empty", func() {
				Expect(result).To(Equal(map[string][]string{}))
			})
		})

		Context("when the token has claims", func() {
			BeforeEach(func() {
				verification.HasToken = true
				verification.IsTokenValid = true
				verification.RawClaims = map[string]interface{}{
					"sub":                "some-sub",
					"name":               "some-name",
					"preferred_username": "some-user-name",
					"email":              "some-email",
					"federated_claims": map[string]interface{}{
						"connector_id": "some-connector",
						"user_id":      "some-user-id",
					},
					"groups": []interface{}{"some-group"},
				}
			})

			Context("when the user is not part of any teams", func() {
				It("returns empty", func() {
					Expect(result).To(Equal(map[string][]string{}))
				})
			})

			Context("when the user is granted a role from their user id", func() {
				BeforeEach(func() {
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": {"some-connector:some-user-id"},
						},
					})
					fakeTeam2.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": {"some-connector:some-user-id"},
						},
					})
					fakeTeam3.AuthReturns(atc.TeamAuth{
						"viewer": map[string][]string{
							"users": {"some-connector:some-user-id"},
						},
					})
				})

				It("returns result with teams", func() {
					Expect(result["some-team-1"]).To(ContainElement("owner"))
					Expect(result["some-team-2"]).To(ContainElement("member"))
					Expect(result["some-team-3"]).To(ContainElement("viewer"))
				})
			})

			Context("when the user is granted a role from their user name", func() {
				BeforeEach(func() {
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": {"some-connector:some-user-name"},
						},
					})
					fakeTeam2.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"users": {"some-connector:some-user-name"},
						},
					})
					fakeTeam3.AuthReturns(atc.TeamAuth{
						"viewer": map[string][]string{
							"users": {"some-connector:some-user-name"},
						},
					})
				})

				It("returns result with teams", func() {
					Expect(result["some-team-1"]).To(ContainElement("owner"))
					Expect(result["some-team-2"]).To(ContainElement("member"))
					Expect(result["some-team-3"]).To(ContainElement("viewer"))
				})
			})

			Context("when the user is granted a role from a group", func() {
				BeforeEach(func() {
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"groups": {"some-connector:some-group"},
						},
					})
					fakeTeam2.AuthReturns(atc.TeamAuth{
						"member": map[string][]string{
							"groups": {"some-connector:some-group"},
						},
					})
					fakeTeam3.AuthReturns(atc.TeamAuth{
						"viewer": map[string][]string{
							"groups": {"some-connector:some-group"},
						},
					})
				})

				It("returns result with teams", func() {
					Expect(result["some-team-1"]).To(ContainElement("owner"))
					Expect(result["some-team-2"]).To(ContainElement("member"))
					Expect(result["some-team-3"]).To(ContainElement("viewer"))
				})
			})

			Context("when the user is granted multiple roles on the same team", func() {
				BeforeEach(func() {
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users": {"some-connector:some-user-id"},
						},
						"member": map[string][]string{
							"groups": {"some-connector:some-group"},
						},
					})
				})

				It("adds both roles", func() {
					Expect(result["some-team-1"]).To(ContainElement("owner"))
					Expect(result["some-team-1"]).To(ContainElement("member"))
				})
			})

			Context("when the user is granted the same role multiple times", func() {
				BeforeEach(func() {
					fakeTeam1.AuthReturns(atc.TeamAuth{
						"owner": map[string][]string{
							"users":  {"some-connector:some-user-id"},
							"groups": {"some-connector:some-group"},
						},
					})
				})

				It("only adds the role once", func() {
					Expect(result["some-team-1"]).To(ContainElement("owner"))
				})
			})
		})
	})
})
