package accessor_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/api/accessor"
	jwt "github.com/dgrijalva/jwt-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Accessor", func() {
	var (
		req             *http.Request
		key             *rsa.PrivateKey
		accessorFactory accessor.AccessFactory
		claims          *jwt.MapClaims
		access          accessor.Access
	)

	BeforeEach(func() {
		var err error
		reader := rand.Reader
		bitSize := 2048

		req, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())

		key, err = rsa.GenerateKey(reader, bitSize)
		Expect(err).NotTo(HaveOccurred())

		publicKey := &key.PublicKey
		accessorFactory = accessor.NewAccessFactory(publicKey)

	})

	Describe("Is Admin", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, "some-action")
		})

		Context("when request has admin claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"is_admin": true}
			})
			It("returns true", func() {
				Expect(access.IsAdmin()).To(BeTrue())
			})
		})
		Context("when request has admin claim set to empty", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"is_admin": ""}
			})
			It("returns false", func() {
				Expect(access.IsAdmin()).To(BeFalse())
			})
		})
		Context("when request has admin claim set to nil", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"is_admin": nil}
			})
			It("returns false", func() {
				Expect(access.IsAdmin()).To(BeFalse())
			})
		})
		Context("when request has admin claim set to false", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"is_admin": false}
			})
			It("returns false", func() {
				Expect(access.IsAdmin()).To(BeFalse())
			})
		})
		Context("when request does not have admin claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{}
			})
			It("returns false", func() {
				Expect(access.IsAdmin()).To(BeFalse())
			})
		})
	})

	Describe("Is System", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, "some-action")
		})

		Context("when request has system claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"system": true}
			})
			It("returns true", func() {
				Expect(access.IsSystem()).To(BeTrue())
			})
		})

		Context("when request has system claim set to empty", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"system": ""}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})

		Context("when request has system claim set to nil", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"system": nil}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})

		Context("when request has system claim set to false", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"system": false}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})

		Context("when request does not have system claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})
	})

	Describe("has token", func() {

		JustBeforeEach(func() {
			access = accessorFactory.Create(req, "some-action")
		})

		Context("when request has token", func() {
			BeforeEach(func() {
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				tokenString, err := token.SignedString(key)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			})

			It("returns true", func() {
				Expect(access.HasToken()).To(BeTrue())
			})
		})

		Context("when request has garbage token", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", "29384q29jdhkwjdhs"))
			})

			It("returns true", func() {
				Expect(access.HasToken()).To(BeTrue())
			})
		})

		Context("when request has empty token", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", ""))
			})

			It("returns true", func() {
				Expect(access.HasToken()).To(BeTrue())
			})
		})

		Context("when request has no token", func() {
			BeforeEach(func() {
				req.Header.Del("Authorization")
			})

			It("returns false", func() {
				Expect(access.HasToken()).To(BeFalse())
			})
		})
	})

	Describe("Is authenticated", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, "some-action")
		})
		Context("when valid token is set", func() {
			It("returns true", func() {
				Expect(access.IsAuthenticated()).To(BeTrue())
			})
		})
	})

	Describe("Is Authorized owner action", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, atc.SetTeam)
		})

		Context("when request has team name claim set for some-team as owner", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": map[string][]string{"some-team": {"owner"}}}
			})
			It("returns true", func() {
				Expect(access.IsAuthorized("some-team")).To(BeTrue())
			})
		})

		Context("when request does not have a team role set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{"some-team"}}
			})
			It("returns true and defaults to role 'owner'", func() {
				Expect(access.IsAuthorized("some-team")).To(BeTrue())
			})
		})

		Context("when request has team name claim set to empty", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{""}}
			})
			It("returns false", func() {
				Expect(access.IsAuthorized("some-team")).To(BeFalse())
			})
		})

		Context("when request has team name claim set to nil", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": nil}
			})
			It("returns false", func() {
				Expect(access.IsAuthorized("some-team")).To(BeFalse())
			})
		})

		Context("when request has team name claim set to other-team:owner", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": map[string][]string{"other-team": {"owner"}}}
			})
			It("returns false", func() {
				Expect(access.IsAuthorized("some-team")).To(BeFalse())
			})
		})

		Context("when request does not have team name claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{}
			})
			It("returns false", func() {
				Expect(access.IsAuthorized("some-team")).To(BeFalse())
			})
		})
	})

	Describe("Get CSRF Token", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, "some-action")
		})

		Context("when request has csrfToken claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"csrf": "fake-token"}
			})
			It("returns true", func() {

				Expect(access.CSRFToken()).To(Equal("fake-token"))
			})
		})
		Context("when request has csrfToken claim set to empty", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"csrf": ""}
			})
			It("returns false", func() {
				Expect(access.CSRFToken()).To(BeEmpty())
			})
		})
		Context("when request has csrfToken claim set to nil", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"csrf": nil}
			})
			It("returns false", func() {
				Expect(access.CSRFToken()).To(BeEmpty())
			})
		})

		Context("when request does not have csrfToken claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{}
			})
			It("returns false", func() {
				Expect(access.CSRFToken()).To(BeEmpty())
			})
		})
	})

	Describe("Get Team Names", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, "some-action")
		})

		Context("when request has teams claim set to nil", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": nil}
			})
			It("returns empty list", func() {
				Expect(access.TeamNames()).To(BeEmpty())
			})
		})
		Context("when request does not have teams claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{}
			})
			It("returns empty list", func() {
				Expect(len(access.TeamNames())).To(BeZero())
			})
		})
		Context("when request has teams claims set in the old array format", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{"fake-team-name"}}
			})
			It("returns list of teams", func() {
				Expect(access.TeamNames()).To(Equal([]string{"fake-team-name"}))
			})
		})
		Context("when request has teams claims set to empty in the old array format", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{""}}
			})
			It("returns empty list", func() {
				Expect(access.TeamNames()).To(Equal([]string{""}))
			})
		})
		Context("when request has teams with multiple roles", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": map[string][]string{
					"team-1": {"owner", "member"},
					"team-2": {"viewer"},
				}}
			})
			It("returns empty list", func() {
				Expect(access.TeamNames()).To(ConsistOf("team-1", "team-2"))
			})
		})
	})

	Describe("Get User Name", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, "some-action")
		})

		Context("when request has user_name claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"user_name": "fake-user-name"}
			})
			It("returns the user_name value", func() {

				Expect(access.UserName()).To(Equal("fake-user-name"))
			})
		})
		Context("when request has user_name claim set to empty", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"user_name": ""}
			})
			It("returns an empty string", func() {
				Expect(access.UserName()).To(BeEmpty())
			})
		})
		Context("when request has user_name claim set to nil", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"user_name": nil}
			})
			It("returns an empty string", func() {
				Expect(access.UserName()).To(BeEmpty())
			})
		})

		Context("when request does not have user_name claim set as true", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{}
			})
			It("returns an empty string", func() {
				Expect(access.UserName()).To(BeEmpty())
			})
		})
		Context("when request has the system claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"system": true}
			})
			It("returns as a system call", func() {
				Expect(access.UserName()).To(Equal("system"))
			})
		})
		Context("when request has the system claim set as false", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"system": false}
			})
			It("returns as a system call", func() {
				Expect(access.UserName()).To(BeEmpty())
			})
		})
	})

	DescribeTable("role actions",
		func(action, role string, authorized bool) {
			claims := &jwt.MapClaims{"teams": map[string][]string{"some-team": {role}}}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access := accessorFactory.Create(req, action)

			Expect(access.IsAuthorized("some-team")).To(Equal(authorized))
		},
		Entry("owner :: table has no entry", "some-role", "owner", false),
		Entry("member :: table has no entry", "some-role", "member", false),
		Entry("pipeline-operator :: table has no entry", "some-role", "pipeline-operator", false),
		Entry("viewer :: table has no entry", "some-role", "viewer", false),

		Entry("owner :: "+atc.SaveConfig, atc.SaveConfig, "owner", true),
		Entry("member :: "+atc.SaveConfig, atc.SaveConfig, "member", true),
		Entry("pipeline-operator :: "+atc.SaveConfig, atc.SaveConfig, "pipeline-operator", false),
		Entry("viewer :: "+atc.SaveConfig, atc.SaveConfig, "viewer", false),

		Entry("owner :: "+atc.GetConfig, atc.GetConfig, "owner", true),
		Entry("member :: "+atc.GetConfig, atc.GetConfig, "member", true),
		Entry("pipeline-operator :: "+atc.GetConfig, atc.GetConfig, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetConfig, atc.GetConfig, "viewer", true),

		Entry("owner :: "+atc.GetCC, atc.GetCC, "owner", true),
		Entry("member :: "+atc.GetCC, atc.GetCC, "member", true),
		Entry("pipeline-operator :: "+atc.GetCC, atc.GetCC, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetCC, atc.GetCC, "viewer", true),

		Entry("owner :: "+atc.GetBuild, atc.GetBuild, "owner", true),
		Entry("member :: "+atc.GetBuild, atc.GetBuild, "member", true),
		Entry("pipeline-operator :: "+atc.GetBuild, atc.GetBuild, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetBuild, atc.GetBuild, "viewer", true),

		Entry("owner :: "+atc.GetBuildPlan, atc.GetBuildPlan, "owner", true),
		Entry("member :: "+atc.GetBuildPlan, atc.GetBuildPlan, "member", true),
		Entry("pipeline-operator :: "+atc.GetBuildPlan, atc.GetBuildPlan, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetBuildPlan, atc.GetBuildPlan, "viewer", true),

		Entry("owner :: "+atc.CreateBuild, atc.CreateBuild, "owner", true),
		Entry("member :: "+atc.CreateBuild, atc.CreateBuild, "member", true),
		Entry("pipeline-operator :: "+atc.CreateBuild, atc.CreateBuild, "pipeline-operator", false),
		Entry("viewer :: "+atc.CreateBuild, atc.CreateBuild, "viewer", false),

		Entry("owner :: "+atc.ListBuilds, atc.ListBuilds, "owner", true),
		Entry("member :: "+atc.ListBuilds, atc.ListBuilds, "member", true),
		Entry("pipeline-operator :: "+atc.ListBuilds, atc.ListBuilds, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListBuilds, atc.ListBuilds, "viewer", true),

		Entry("owner :: "+atc.BuildEvents, atc.BuildEvents, "owner", true),
		Entry("member :: "+atc.BuildEvents, atc.BuildEvents, "member", true),
		Entry("pipeline-operator :: "+atc.BuildEvents, atc.BuildEvents, "pipeline-operator", true),
		Entry("viewer :: "+atc.BuildEvents, atc.BuildEvents, "viewer", true),

		Entry("owner :: "+atc.BuildResources, atc.BuildResources, "owner", true),
		Entry("member :: "+atc.BuildResources, atc.BuildResources, "member", true),
		Entry("pipeline-operator :: "+atc.BuildResources, atc.BuildResources, "pipeline-operator", true),
		Entry("viewer :: "+atc.BuildResources, atc.BuildResources, "viewer", true),

		Entry("owner :: "+atc.AbortBuild, atc.AbortBuild, "owner", true),
		Entry("member :: "+atc.AbortBuild, atc.AbortBuild, "member", true),
		Entry("pipeline-operator :: "+atc.AbortBuild, atc.AbortBuild, "pipeline-operator", true),
		Entry("viewer :: "+atc.AbortBuild, atc.AbortBuild, "viewer", false),

		Entry("owner :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "owner", true),
		Entry("member :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "member", true),
		Entry("pipeline-operator :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "viewer", true),

		Entry("owner :: "+atc.GetJob, atc.GetJob, "owner", true),
		Entry("member :: "+atc.GetJob, atc.GetJob, "member", true),
		Entry("pipeline-operator :: "+atc.GetJob, atc.GetJob, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetJob, atc.GetJob, "viewer", true),

		Entry("owner :: "+atc.CreateJobBuild, atc.CreateJobBuild, "owner", true),
		Entry("member :: "+atc.CreateJobBuild, atc.CreateJobBuild, "member", true),
		Entry("pipeline-operator :: "+atc.CreateJobBuild, atc.CreateJobBuild, "pipeline-operator", true),
		Entry("viewer :: "+atc.CreateJobBuild, atc.CreateJobBuild, "viewer", false),

		Entry("owner :: "+atc.ListAllJobs, atc.ListAllJobs, "owner", true),
		Entry("member :: "+atc.ListAllJobs, atc.ListAllJobs, "member", true),
		Entry("pipeline-operator :: "+atc.ListAllJobs, atc.ListAllJobs, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListAllJobs, atc.ListAllJobs, "viewer", true),

		Entry("owner :: "+atc.ListJobs, atc.ListJobs, "owner", true),
		Entry("member :: "+atc.ListJobs, atc.ListJobs, "member", true),
		Entry("pipeline-operator :: "+atc.ListJobs, atc.ListJobs, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListJobs, atc.ListJobs, "viewer", true),

		Entry("owner :: "+atc.ListJobBuilds, atc.ListJobBuilds, "owner", true),
		Entry("member :: "+atc.ListJobBuilds, atc.ListJobBuilds, "member", true),
		Entry("pipeline-operator :: "+atc.ListJobBuilds, atc.ListJobBuilds, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListJobBuilds, atc.ListJobBuilds, "viewer", true),

		Entry("owner :: "+atc.ListJobInputs, atc.ListJobInputs, "owner", true),
		Entry("member :: "+atc.ListJobInputs, atc.ListJobInputs, "member", true),
		Entry("pipeline-operator :: "+atc.ListJobInputs, atc.ListJobInputs, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListJobInputs, atc.ListJobInputs, "viewer", true),

		Entry("owner :: "+atc.GetJobBuild, atc.GetJobBuild, "owner", true),
		Entry("member :: "+atc.GetJobBuild, atc.GetJobBuild, "member", true),
		Entry("pipeline-operator :: "+atc.GetJobBuild, atc.GetJobBuild, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetJobBuild, atc.GetJobBuild, "viewer", true),

		Entry("owner :: "+atc.PauseJob, atc.PauseJob, "owner", true),
		Entry("member :: "+atc.PauseJob, atc.PauseJob, "member", true),
		Entry("pipeline-operator :: "+atc.PauseJob, atc.PauseJob, "pipeline-operator", true),
		Entry("viewer :: "+atc.PauseJob, atc.PauseJob, "viewer", false),

		Entry("owner :: "+atc.UnpauseJob, atc.UnpauseJob, "owner", true),
		Entry("member :: "+atc.UnpauseJob, atc.UnpauseJob, "member", true),
		Entry("pipeline-operator :: "+atc.UnpauseJob, atc.UnpauseJob, "pipeline-operator", true),
		Entry("viewer :: "+atc.UnpauseJob, atc.UnpauseJob, "viewer", false),

		Entry("owner :: "+atc.GetVersionsDB, atc.GetVersionsDB, "owner", true),
		Entry("member :: "+atc.GetVersionsDB, atc.GetVersionsDB, "member", true),
		Entry("pipeline-operator :: "+atc.GetVersionsDB, atc.GetVersionsDB, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetVersionsDB, atc.GetVersionsDB, "viewer", true),

		Entry("owner :: "+atc.JobBadge, atc.JobBadge, "owner", true),
		Entry("member :: "+atc.JobBadge, atc.JobBadge, "member", true),
		Entry("pipeline-operator :: "+atc.JobBadge, atc.JobBadge, "pipeline-operator", true),
		Entry("viewer :: "+atc.JobBadge, atc.JobBadge, "viewer", true),

		Entry("owner :: "+atc.MainJobBadge, atc.MainJobBadge, "owner", true),
		Entry("member :: "+atc.MainJobBadge, atc.MainJobBadge, "member", true),
		Entry("pipeline-operator :: "+atc.MainJobBadge, atc.MainJobBadge, "pipeline-operator", true),
		Entry("viewer :: "+atc.MainJobBadge, atc.MainJobBadge, "viewer", true),

		Entry("owner :: "+atc.ClearTaskCache, atc.ClearTaskCache, "owner", true),
		Entry("member :: "+atc.ClearTaskCache, atc.ClearTaskCache, "member", true),
		Entry("pipeline-operator :: "+atc.ClearTaskCache, atc.ClearTaskCache, "pipeline-operator", true),
		Entry("viewer :: "+atc.ClearTaskCache, atc.ClearTaskCache, "viewer", false),

		Entry("owner :: "+atc.ListAllResources, atc.ListAllResources, "owner", true),
		Entry("member :: "+atc.ListAllResources, atc.ListAllResources, "member", true),
		Entry("pipeline-operator :: "+atc.ListAllResources, atc.ListAllResources, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListAllResources, atc.ListAllResources, "viewer", true),

		Entry("owner :: "+atc.ListResources, atc.ListResources, "owner", true),
		Entry("member :: "+atc.ListResources, atc.ListResources, "member", true),
		Entry("pipeline-operator :: "+atc.ListResources, atc.ListResources, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListResources, atc.ListResources, "viewer", true),

		Entry("owner :: "+atc.ListResourceTypes, atc.ListResourceTypes, "owner", true),
		Entry("member :: "+atc.ListResourceTypes, atc.ListResourceTypes, "member", true),
		Entry("pipeline-operator :: "+atc.ListResourceTypes, atc.ListResourceTypes, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListResourceTypes, atc.ListResourceTypes, "viewer", true),

		Entry("owner :: "+atc.GetResource, atc.GetResource, "owner", true),
		Entry("member :: "+atc.GetResource, atc.GetResource, "member", true),
		Entry("pipeline-operator :: "+atc.GetResource, atc.GetResource, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetResource, atc.GetResource, "viewer", true),

		Entry("owner :: "+atc.CheckResource, atc.CheckResource, "owner", true),
		Entry("member :: "+atc.CheckResource, atc.CheckResource, "member", true),
		Entry("pipeline-operator :: "+atc.CheckResource, atc.CheckResource, "pipeline-operator", true),
		Entry("viewer :: "+atc.CheckResource, atc.CheckResource, "viewer", false),

		Entry("owner :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "owner", true),
		Entry("member :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "member", true),
		Entry("pipeline-operator :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "pipeline-operator", true),
		Entry("viewer :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "viewer", false),

		Entry("owner :: "+atc.CheckResourceType, atc.CheckResourceType, "owner", true),
		Entry("member :: "+atc.CheckResourceType, atc.CheckResourceType, "member", true),
		Entry("pipeline-operator :: "+atc.CheckResourceType, atc.CheckResourceType, "pipeline-operator", true),
		Entry("viewer :: "+atc.CheckResourceType, atc.CheckResourceType, "viewer", false),

		Entry("owner :: "+atc.ListResourceVersions, atc.ListResourceVersions, "owner", true),
		Entry("member :: "+atc.ListResourceVersions, atc.ListResourceVersions, "member", true),
		Entry("pipeline-operator :: "+atc.ListResourceVersions, atc.ListResourceVersions, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListResourceVersions, atc.ListResourceVersions, "viewer", true),

		Entry("owner :: "+atc.GetResourceVersion, atc.GetResourceVersion, "owner", true),
		Entry("member :: "+atc.GetResourceVersion, atc.GetResourceVersion, "member", true),
		Entry("pipeline-operator :: "+atc.GetResourceVersion, atc.GetResourceVersion, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetResourceVersion, atc.GetResourceVersion, "viewer", true),

		Entry("owner :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "owner", true),
		Entry("member :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "member", true),
		Entry("pipeline-operator :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "pipeline-operator", true),
		Entry("viewer :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "viewer", false),

		Entry("owner :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "owner", true),
		Entry("member :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "member", true),
		Entry("pipeline-operator :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "pipeline-operator", true),
		Entry("viewer :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "viewer", false),

		Entry("owner :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "owner", true),
		Entry("member :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "member", true),
		Entry("pipeline-operator :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "viewer", true),

		Entry("owner :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "owner", true),
		Entry("member :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "member", true),
		Entry("pipeline-operator :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "viewer", true),

		Entry("owner :: "+atc.GetResourceCausality, atc.GetResourceCausality, "owner", true),
		Entry("member :: "+atc.GetResourceCausality, atc.GetResourceCausality, "member", true),
		Entry("pipeline-operator :: "+atc.GetResourceCausality, atc.GetResourceCausality, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetResourceCausality, atc.GetResourceCausality, "viewer", true),

		Entry("owner :: "+atc.ListAllPipelines, atc.ListAllPipelines, "owner", true),
		Entry("member :: "+atc.ListAllPipelines, atc.ListAllPipelines, "member", true),
		Entry("pipeline-operator :: "+atc.ListAllPipelines, atc.ListAllPipelines, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListAllPipelines, atc.ListAllPipelines, "viewer", true),

		Entry("owner :: "+atc.ListPipelines, atc.ListPipelines, "owner", true),
		Entry("member :: "+atc.ListPipelines, atc.ListPipelines, "member", true),
		Entry("pipeline-operator :: "+atc.ListPipelines, atc.ListPipelines, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListPipelines, atc.ListPipelines, "viewer", true),

		Entry("owner :: "+atc.GetPipeline, atc.GetPipeline, "owner", true),
		Entry("member :: "+atc.GetPipeline, atc.GetPipeline, "member", true),
		Entry("pipeline-operator :: "+atc.GetPipeline, atc.GetPipeline, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetPipeline, atc.GetPipeline, "viewer", true),

		Entry("owner :: "+atc.DeletePipeline, atc.DeletePipeline, "owner", true),
		Entry("member :: "+atc.DeletePipeline, atc.DeletePipeline, "member", true),
		Entry("pipeline-operator :: "+atc.DeletePipeline, atc.DeletePipeline, "pipeline-operator", false),
		Entry("viewer :: "+atc.DeletePipeline, atc.DeletePipeline, "viewer", false),

		Entry("owner :: "+atc.OrderPipelines, atc.OrderPipelines, "owner", true),
		Entry("member :: "+atc.OrderPipelines, atc.OrderPipelines, "member", true),
		Entry("pipeline-operator :: "+atc.OrderPipelines, atc.OrderPipelines, "pipeline-operator", false),
		Entry("viewer :: "+atc.OrderPipelines, atc.OrderPipelines, "viewer", false),

		Entry("owner :: "+atc.PausePipeline, atc.PausePipeline, "owner", true),
		Entry("member :: "+atc.PausePipeline, atc.PausePipeline, "member", true),
		Entry("pipeline-operator :: "+atc.PausePipeline, atc.PausePipeline, "pipeline-operator", true),
		Entry("viewer :: "+atc.PausePipeline, atc.PausePipeline, "viewer", false),

		Entry("owner :: "+atc.UnpausePipeline, atc.UnpausePipeline, "owner", true),
		Entry("member :: "+atc.UnpausePipeline, atc.UnpausePipeline, "member", true),
		Entry("pipeline-operator :: "+atc.UnpausePipeline, atc.UnpausePipeline, "pipeline-operator", true),
		Entry("viewer :: "+atc.UnpausePipeline, atc.UnpausePipeline, "viewer", false),

		Entry("owner :: "+atc.ExposePipeline, atc.ExposePipeline, "owner", true),
		Entry("member :: "+atc.ExposePipeline, atc.ExposePipeline, "member", true),
		Entry("pipeline-operator :: "+atc.ExposePipeline, atc.ExposePipeline, "pipeline-operator", false),
		Entry("viewer :: "+atc.ExposePipeline, atc.ExposePipeline, "viewer", false),

		Entry("owner :: "+atc.HidePipeline, atc.HidePipeline, "owner", true),
		Entry("member :: "+atc.HidePipeline, atc.HidePipeline, "member", true),
		Entry("pipeline-operator :: "+atc.HidePipeline, atc.HidePipeline, "pipeline-operator", false),
		Entry("viewer :: "+atc.HidePipeline, atc.HidePipeline, "viewer", false),

		Entry("owner :: "+atc.RenamePipeline, atc.RenamePipeline, "owner", true),
		Entry("member :: "+atc.RenamePipeline, atc.RenamePipeline, "member", true),
		Entry("pipeline-operator :: "+atc.RenamePipeline, atc.RenamePipeline, "pipeline-operator", false),
		Entry("viewer :: "+atc.RenamePipeline, atc.RenamePipeline, "viewer", false),

		Entry("owner :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "owner", true),
		Entry("member :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "member", true),
		Entry("pipeline-operator :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "viewer", true),

		Entry("owner :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "owner", true),
		Entry("member :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "member", true),
		Entry("pipeline-operator :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "pipeline-operator", false),
		Entry("viewer :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "viewer", false),

		Entry("owner :: "+atc.PipelineBadge, atc.PipelineBadge, "owner", true),
		Entry("member :: "+atc.PipelineBadge, atc.PipelineBadge, "member", true),
		Entry("pipeline-operator :: "+atc.PipelineBadge, atc.PipelineBadge, "pipeline-operator", true),
		Entry("viewer :: "+atc.PipelineBadge, atc.PipelineBadge, "viewer", true),

		Entry("owner :: "+atc.RegisterWorker, atc.RegisterWorker, "owner", true),
		Entry("member :: "+atc.RegisterWorker, atc.RegisterWorker, "member", true),
		Entry("pipeline-operator :: "+atc.RegisterWorker, atc.RegisterWorker, "pipeline-operator", false),
		Entry("viewer :: "+atc.RegisterWorker, atc.RegisterWorker, "viewer", false),

		Entry("owner :: "+atc.LandWorker, atc.LandWorker, "owner", true),
		Entry("member :: "+atc.LandWorker, atc.LandWorker, "member", true),
		Entry("pipeline-operator :: "+atc.LandWorker, atc.LandWorker, "pipeline-operator", false),
		Entry("viewer :: "+atc.LandWorker, atc.LandWorker, "viewer", false),

		Entry("owner :: "+atc.RetireWorker, atc.RetireWorker, "owner", true),
		Entry("member :: "+atc.RetireWorker, atc.RetireWorker, "member", true),
		Entry("pipeline-operator :: "+atc.RetireWorker, atc.RetireWorker, "pipeline-operator", false),
		Entry("viewer :: "+atc.RetireWorker, atc.RetireWorker, "viewer", false),

		Entry("owner :: "+atc.PruneWorker, atc.PruneWorker, "owner", true),
		Entry("member :: "+atc.PruneWorker, atc.PruneWorker, "member", true),
		Entry("pipeline-operator :: "+atc.PruneWorker, atc.PruneWorker, "pipeline-operator", false),
		Entry("viewer :: "+atc.PruneWorker, atc.PruneWorker, "viewer", false),

		Entry("owner :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "owner", true),
		Entry("member :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "member", true),
		Entry("pipeline-operator :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "pipeline-operator", false),
		Entry("viewer :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "viewer", false),

		Entry("owner :: "+atc.ListWorkers, atc.ListWorkers, "owner", true),
		Entry("member :: "+atc.ListWorkers, atc.ListWorkers, "member", true),
		Entry("pipeline-operator :: "+atc.ListWorkers, atc.ListWorkers, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListWorkers, atc.ListWorkers, "viewer", true),

		Entry("owner :: "+atc.DeleteWorker, atc.DeleteWorker, "owner", true),
		Entry("member :: "+atc.DeleteWorker, atc.DeleteWorker, "member", true),
		Entry("pipeline-operator :: "+atc.DeleteWorker, atc.DeleteWorker, "pipeline-operator", false),
		Entry("viewer :: "+atc.DeleteWorker, atc.DeleteWorker, "viewer", false),

		Entry("owner :: "+atc.SetLogLevel, atc.SetLogLevel, "owner", true),
		Entry("member :: "+atc.SetLogLevel, atc.SetLogLevel, "member", true),
		Entry("pipeline-operator :: "+atc.SetLogLevel, atc.SetLogLevel, "pipeline-operator", false),
		Entry("viewer :: "+atc.SetLogLevel, atc.SetLogLevel, "viewer", false),

		Entry("owner :: "+atc.GetLogLevel, atc.GetLogLevel, "owner", true),
		Entry("member :: "+atc.GetLogLevel, atc.GetLogLevel, "member", true),
		Entry("pipeline-operator :: "+atc.GetLogLevel, atc.GetLogLevel, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetLogLevel, atc.GetLogLevel, "viewer", true),

		Entry("owner :: "+atc.DownloadCLI, atc.DownloadCLI, "owner", true),
		Entry("member :: "+atc.DownloadCLI, atc.DownloadCLI, "member", true),
		Entry("pipeline-operator :: "+atc.DownloadCLI, atc.DownloadCLI, "pipeline-operator", true),
		Entry("viewer :: "+atc.DownloadCLI, atc.DownloadCLI, "viewer", true),

		Entry("owner :: "+atc.GetInfo, atc.GetInfo, "owner", true),
		Entry("member :: "+atc.GetInfo, atc.GetInfo, "member", true),
		Entry("pipeline-operator :: "+atc.GetInfo, atc.GetInfo, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetInfo, atc.GetInfo, "viewer", true),

		Entry("owner :: "+atc.GetInfoCreds, atc.GetInfoCreds, "owner", true),
		Entry("member :: "+atc.GetInfoCreds, atc.GetInfoCreds, "member", true),
		Entry("pipeline-operator :: "+atc.GetInfoCreds, atc.GetInfoCreds, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetInfoCreds, atc.GetInfoCreds, "viewer", true),

		Entry("owner :: "+atc.ListContainers, atc.ListContainers, "owner", true),
		Entry("member :: "+atc.ListContainers, atc.ListContainers, "member", true),
		Entry("pipeline-operator :: "+atc.ListContainers, atc.ListContainers, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListContainers, atc.ListContainers, "viewer", true),

		Entry("owner :: "+atc.GetContainer, atc.GetContainer, "owner", true),
		Entry("member :: "+atc.GetContainer, atc.GetContainer, "member", true),
		Entry("pipeline-operator :: "+atc.GetContainer, atc.GetContainer, "pipeline-operator", true),
		Entry("viewer :: "+atc.GetContainer, atc.GetContainer, "viewer", true),

		Entry("owner :: "+atc.HijackContainer, atc.HijackContainer, "owner", true),
		Entry("member :: "+atc.HijackContainer, atc.HijackContainer, "member", true),
		Entry("pipeline-operator :: "+atc.HijackContainer, atc.HijackContainer, "pipeline-operator", false),
		Entry("viewer :: "+atc.HijackContainer, atc.HijackContainer, "viewer", false),

		Entry("owner :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "owner", true),
		Entry("member :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "member", true),
		Entry("pipeline-operator :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "viewer", true),

		Entry("owner :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "owner", true),
		Entry("member :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "member", true),
		Entry("pipeline-operator :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "pipeline-operator", false),
		Entry("viewer :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "viewer", false),

		Entry("owner :: "+atc.ListVolumes, atc.ListVolumes, "owner", true),
		Entry("member :: "+atc.ListVolumes, atc.ListVolumes, "member", true),
		Entry("pipeline-operator :: "+atc.ListVolumes, atc.ListVolumes, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListVolumes, atc.ListVolumes, "viewer", true),

		Entry("owner :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "owner", true),
		Entry("member :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "member", true),
		Entry("pipeline-operator :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "viewer", true),

		Entry("owner :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "owner", true),
		Entry("member :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "member", true),
		Entry("pipeline-operator :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "pipeline-operator", false),
		Entry("viewer :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "viewer", false),

		Entry("owner :: "+atc.ListTeams, atc.ListTeams, "owner", true),
		Entry("member :: "+atc.ListTeams, atc.ListTeams, "member", true),
		Entry("pipeline-operator :: "+atc.ListTeams, atc.ListTeams, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListTeams, atc.ListTeams, "viewer", true),

		Entry("owner :: "+atc.GetTeam, atc.GetTeam, "owner", true),
		Entry("member :: "+atc.GetTeam, atc.GetTeam, "member", true),
		Entry("viewer :: "+atc.GetTeam, atc.GetTeam, "viewer", true),

		Entry("owner :: "+atc.SetTeam, atc.SetTeam, "owner", true),
		Entry("member :: "+atc.SetTeam, atc.SetTeam, "member", false),
		Entry("pipeline-operator :: "+atc.SetTeam, atc.SetTeam, "pipeline-operator", false),
		Entry("viewer :: "+atc.SetTeam, atc.SetTeam, "viewer", false),

		Entry("owner :: "+atc.RenameTeam, atc.RenameTeam, "owner", true),
		Entry("member :: "+atc.RenameTeam, atc.RenameTeam, "member", false),
		Entry("pipeline-operator :: "+atc.RenameTeam, atc.RenameTeam, "pipeline-operator", false),
		Entry("viewer :: "+atc.RenameTeam, atc.RenameTeam, "viewer", false),

		Entry("owner :: "+atc.DestroyTeam, atc.DestroyTeam, "owner", true),
		Entry("member :: "+atc.DestroyTeam, atc.DestroyTeam, "member", false),
		Entry("pipeline-operator :: "+atc.DestroyTeam, atc.DestroyTeam, "pipeline-operator", false),
		Entry("viewer :: "+atc.DestroyTeam, atc.DestroyTeam, "viewer", false),

		Entry("owner :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "owner", true),
		Entry("member :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "member", true),
		Entry("pipeline-operator :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "viewer", true),

		Entry("owner :: "+atc.CreateArtifact, atc.CreateArtifact, "owner", true),
		Entry("member :: "+atc.CreateArtifact, atc.CreateArtifact, "member", true),
		Entry("pipeline-operator :: "+atc.CreateArtifact, atc.CreateArtifact, "pipeline-operator", false),
		Entry("viewer :: "+atc.CreateArtifact, atc.CreateArtifact, "viewer", false),

		Entry("owner :: "+atc.GetArtifact, atc.GetArtifact, "owner", true),
		Entry("member :: "+atc.GetArtifact, atc.GetArtifact, "member", true),
		Entry("pipeline-operator :: "+atc.GetArtifact, atc.GetArtifact, "pipeline-operator", false),
		Entry("viewer :: "+atc.GetArtifact, atc.GetArtifact, "viewer", false),

		Entry("owner :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "owner", true),
		Entry("member :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "member", true),
		Entry("pipeline-operator :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "pipeline-operator", true),
		Entry("viewer :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "viewer", true),
	)
})
