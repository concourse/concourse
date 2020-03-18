package accessor_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/dgrijalva/jwt-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
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

	Describe("Is system", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, "some-action")
		})

		Context("when request has system claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{accessor.System: true}
			})
			It("returns true", func() {
				Expect(access.IsSystem()).To(BeTrue())
			})
		})

		Context("when request has system claim set to empty", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{accessor.System: ""}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})

		Context("when request has system claim set to nil", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{accessor.System: nil}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})

		Context("when request has system claim set to false", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{accessor.System: false}
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
				claims = &jwt.MapClaims{"teams": map[string][]string{"some-team": {accessor.OwnerRole}}}
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
				claims = &jwt.MapClaims{"teams": map[string][]string{"other-team": {accessor.OwnerRole}}}
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
					"team-1": {accessor.OwnerRole, accessor.MemberRole},
					"team-2": {accessor.ViewerRole},
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
				claims = &jwt.MapClaims{accessor.System: true}
			})
			It("returns as a system call", func() {
				Expect(access.UserName()).To(Equal(accessor.System))
			})
		})
		Context("when request has the system claim set as false", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{accessor.System: false}
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
		Entry("owner :: table has no entry", "some-role", accessor.OwnerRole, false),
		Entry("member :: table has no entry", "some-role", accessor.MemberRole, false),
		Entry("pipeline-operator :: table has no entry", "some-role", accessor.OperatorRole, false),
		Entry("viewer :: table has no entry", "some-role", accessor.ViewerRole, false),

		Entry("owner :: "+atc.SaveConfig, atc.SaveConfig, accessor.OwnerRole, true),
		Entry("member :: "+atc.SaveConfig, atc.SaveConfig, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.SaveConfig, atc.SaveConfig, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.SaveConfig, atc.SaveConfig, accessor.ViewerRole, false),

		Entry("owner :: "+atc.GetConfig, atc.GetConfig, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetConfig, atc.GetConfig, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetConfig, atc.GetConfig, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetConfig, atc.GetConfig, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetCC, atc.GetCC, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetCC, atc.GetCC, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetCC, atc.GetCC, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetCC, atc.GetCC, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetBuild, atc.GetBuild, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetBuild, atc.GetBuild, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetBuild, atc.GetBuild, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetBuild, atc.GetBuild, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetBuildPlan, atc.GetBuildPlan, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetBuildPlan, atc.GetBuildPlan, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetBuildPlan, atc.GetBuildPlan, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetBuildPlan, atc.GetBuildPlan, accessor.ViewerRole, true),

		Entry("owner :: "+atc.CreateBuild, atc.CreateBuild, accessor.OwnerRole, true),
		Entry("member :: "+atc.CreateBuild, atc.CreateBuild, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.CreateBuild, atc.CreateBuild, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.CreateBuild, atc.CreateBuild, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListBuilds, atc.ListBuilds, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListBuilds, atc.ListBuilds, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListBuilds, atc.ListBuilds, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListBuilds, atc.ListBuilds, accessor.ViewerRole, true),

		Entry("owner :: "+atc.BuildEvents, atc.BuildEvents, accessor.OwnerRole, true),
		Entry("member :: "+atc.BuildEvents, atc.BuildEvents, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.BuildEvents, atc.BuildEvents, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.BuildEvents, atc.BuildEvents, accessor.ViewerRole, true),

		Entry("owner :: "+atc.BuildResources, atc.BuildResources, accessor.OwnerRole, true),
		Entry("member :: "+atc.BuildResources, atc.BuildResources, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.BuildResources, atc.BuildResources, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.BuildResources, atc.BuildResources, accessor.ViewerRole, true),

		Entry("owner :: "+atc.AbortBuild, atc.AbortBuild, accessor.OwnerRole, true),
		Entry("member :: "+atc.AbortBuild, atc.AbortBuild, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.AbortBuild, atc.AbortBuild, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.AbortBuild, atc.AbortBuild, accessor.ViewerRole, false),

		Entry("owner :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetJob, atc.GetJob, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetJob, atc.GetJob, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetJob, atc.GetJob, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetJob, atc.GetJob, accessor.ViewerRole, true),

		Entry("owner :: "+atc.CreateJobBuild, atc.CreateJobBuild, accessor.OwnerRole, true),
		Entry("member :: "+atc.CreateJobBuild, atc.CreateJobBuild, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.CreateJobBuild, atc.CreateJobBuild, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.CreateJobBuild, atc.CreateJobBuild, accessor.ViewerRole, false),

		Entry("owner :: "+atc.RerunJobBuild, atc.RerunJobBuild, accessor.OwnerRole, true),
		Entry("member :: "+atc.RerunJobBuild, atc.RerunJobBuild, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.RerunJobBuild, atc.RerunJobBuild, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.RerunJobBuild, atc.RerunJobBuild, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListAllJobs, atc.ListAllJobs, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListAllJobs, atc.ListAllJobs, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListAllJobs, atc.ListAllJobs, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListAllJobs, atc.ListAllJobs, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListJobs, atc.ListJobs, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListJobs, atc.ListJobs, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListJobs, atc.ListJobs, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListJobs, atc.ListJobs, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListJobBuilds, atc.ListJobBuilds, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListJobBuilds, atc.ListJobBuilds, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListJobBuilds, atc.ListJobBuilds, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListJobBuilds, atc.ListJobBuilds, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListJobInputs, atc.ListJobInputs, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListJobInputs, atc.ListJobInputs, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListJobInputs, atc.ListJobInputs, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListJobInputs, atc.ListJobInputs, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetJobBuild, atc.GetJobBuild, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetJobBuild, atc.GetJobBuild, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetJobBuild, atc.GetJobBuild, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetJobBuild, atc.GetJobBuild, accessor.ViewerRole, true),

		Entry("owner :: "+atc.PauseJob, atc.PauseJob, accessor.OwnerRole, true),
		Entry("member :: "+atc.PauseJob, atc.PauseJob, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.PauseJob, atc.PauseJob, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.PauseJob, atc.PauseJob, accessor.ViewerRole, false),

		Entry("owner :: "+atc.UnpauseJob, atc.UnpauseJob, accessor.OwnerRole, true),
		Entry("member :: "+atc.UnpauseJob, atc.UnpauseJob, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.UnpauseJob, atc.UnpauseJob, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.UnpauseJob, atc.UnpauseJob, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ScheduleJob, atc.ScheduleJob, accessor.OwnerRole, true),
		Entry("member :: "+atc.ScheduleJob, atc.ScheduleJob, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ScheduleJob, atc.ScheduleJob, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ScheduleJob, atc.ScheduleJob, accessor.ViewerRole, false),

		Entry("owner :: "+atc.GetVersionsDB, atc.GetVersionsDB, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetVersionsDB, atc.GetVersionsDB, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetVersionsDB, atc.GetVersionsDB, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetVersionsDB, atc.GetVersionsDB, accessor.ViewerRole, true),

		Entry("owner :: "+atc.JobBadge, atc.JobBadge, accessor.OwnerRole, true),
		Entry("member :: "+atc.JobBadge, atc.JobBadge, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.JobBadge, atc.JobBadge, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.JobBadge, atc.JobBadge, accessor.ViewerRole, true),

		Entry("owner :: "+atc.MainJobBadge, atc.MainJobBadge, accessor.OwnerRole, true),
		Entry("member :: "+atc.MainJobBadge, atc.MainJobBadge, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.MainJobBadge, atc.MainJobBadge, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.MainJobBadge, atc.MainJobBadge, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ClearTaskCache, atc.ClearTaskCache, accessor.OwnerRole, true),
		Entry("member :: "+atc.ClearTaskCache, atc.ClearTaskCache, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ClearTaskCache, atc.ClearTaskCache, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ClearTaskCache, atc.ClearTaskCache, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListAllResources, atc.ListAllResources, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListAllResources, atc.ListAllResources, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListAllResources, atc.ListAllResources, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListAllResources, atc.ListAllResources, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListResources, atc.ListResources, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListResources, atc.ListResources, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListResources, atc.ListResources, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListResources, atc.ListResources, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListResourceTypes, atc.ListResourceTypes, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListResourceTypes, atc.ListResourceTypes, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListResourceTypes, atc.ListResourceTypes, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListResourceTypes, atc.ListResourceTypes, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetResource, atc.GetResource, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetResource, atc.GetResource, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetResource, atc.GetResource, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetResource, atc.GetResource, accessor.ViewerRole, true),

		Entry("owner :: "+atc.CheckResource, atc.CheckResource, accessor.OwnerRole, true),
		Entry("member :: "+atc.CheckResource, atc.CheckResource, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.CheckResource, atc.CheckResource, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.CheckResource, atc.CheckResource, accessor.ViewerRole, false),

		Entry("owner :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, accessor.OwnerRole, true),
		Entry("member :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, accessor.ViewerRole, false),

		Entry("owner :: "+atc.CheckResourceType, atc.CheckResourceType, accessor.OwnerRole, true),
		Entry("member :: "+atc.CheckResourceType, atc.CheckResourceType, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.CheckResourceType, atc.CheckResourceType, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.CheckResourceType, atc.CheckResourceType, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListResourceVersions, atc.ListResourceVersions, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListResourceVersions, atc.ListResourceVersions, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListResourceVersions, atc.ListResourceVersions, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListResourceVersions, atc.ListResourceVersions, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetResourceVersion, atc.GetResourceVersion, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetResourceVersion, atc.GetResourceVersion, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetResourceVersion, atc.GetResourceVersion, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetResourceVersion, atc.GetResourceVersion, accessor.ViewerRole, true),

		Entry("owner :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, accessor.OwnerRole, true),
		Entry("member :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, accessor.ViewerRole, false),

		Entry("owner :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, accessor.OwnerRole, true),
		Entry("member :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetResourceCausality, atc.GetResourceCausality, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetResourceCausality, atc.GetResourceCausality, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetResourceCausality, atc.GetResourceCausality, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetResourceCausality, atc.GetResourceCausality, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListAllPipelines, atc.ListAllPipelines, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListAllPipelines, atc.ListAllPipelines, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListAllPipelines, atc.ListAllPipelines, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListAllPipelines, atc.ListAllPipelines, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListPipelines, atc.ListPipelines, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListPipelines, atc.ListPipelines, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListPipelines, atc.ListPipelines, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListPipelines, atc.ListPipelines, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetPipeline, atc.GetPipeline, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetPipeline, atc.GetPipeline, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetPipeline, atc.GetPipeline, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetPipeline, atc.GetPipeline, accessor.ViewerRole, true),

		Entry("owner :: "+atc.DeletePipeline, atc.DeletePipeline, accessor.OwnerRole, true),
		Entry("member :: "+atc.DeletePipeline, atc.DeletePipeline, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.DeletePipeline, atc.DeletePipeline, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.DeletePipeline, atc.DeletePipeline, accessor.ViewerRole, false),

		Entry("owner :: "+atc.OrderPipelines, atc.OrderPipelines, accessor.OwnerRole, true),
		Entry("member :: "+atc.OrderPipelines, atc.OrderPipelines, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.OrderPipelines, atc.OrderPipelines, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.OrderPipelines, atc.OrderPipelines, accessor.ViewerRole, false),

		Entry("owner :: "+atc.PausePipeline, atc.PausePipeline, accessor.OwnerRole, true),
		Entry("member :: "+atc.PausePipeline, atc.PausePipeline, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.PausePipeline, atc.PausePipeline, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.PausePipeline, atc.PausePipeline, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ArchivePipeline, atc.ArchivePipeline, accessor.OwnerRole, true),
		Entry("member :: "+atc.ArchivePipeline, atc.ArchivePipeline, accessor.MemberRole, false),
		Entry("pipeline-operator :: "+atc.ArchivePipeline, atc.ArchivePipeline, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.ArchivePipeline, atc.ArchivePipeline, accessor.ViewerRole, false),

		Entry("owner :: "+atc.UnpausePipeline, atc.UnpausePipeline, accessor.OwnerRole, true),
		Entry("member :: "+atc.UnpausePipeline, atc.UnpausePipeline, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.UnpausePipeline, atc.UnpausePipeline, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.UnpausePipeline, atc.UnpausePipeline, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ExposePipeline, atc.ExposePipeline, accessor.OwnerRole, true),
		Entry("member :: "+atc.ExposePipeline, atc.ExposePipeline, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ExposePipeline, atc.ExposePipeline, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.ExposePipeline, atc.ExposePipeline, accessor.ViewerRole, false),

		Entry("owner :: "+atc.HidePipeline, atc.HidePipeline, accessor.OwnerRole, true),
		Entry("member :: "+atc.HidePipeline, atc.HidePipeline, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.HidePipeline, atc.HidePipeline, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.HidePipeline, atc.HidePipeline, accessor.ViewerRole, false),

		Entry("owner :: "+atc.RenamePipeline, atc.RenamePipeline, accessor.OwnerRole, true),
		Entry("member :: "+atc.RenamePipeline, atc.RenamePipeline, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.RenamePipeline, atc.RenamePipeline, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.RenamePipeline, atc.RenamePipeline, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, accessor.ViewerRole, true),

		Entry("owner :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, accessor.OwnerRole, true),
		Entry("member :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, accessor.ViewerRole, false),

		Entry("owner :: "+atc.PipelineBadge, atc.PipelineBadge, accessor.OwnerRole, true),
		Entry("member :: "+atc.PipelineBadge, atc.PipelineBadge, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.PipelineBadge, atc.PipelineBadge, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.PipelineBadge, atc.PipelineBadge, accessor.ViewerRole, true),

		Entry("owner :: "+atc.RegisterWorker, atc.RegisterWorker, accessor.OwnerRole, true),
		Entry("member :: "+atc.RegisterWorker, atc.RegisterWorker, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.RegisterWorker, atc.RegisterWorker, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.RegisterWorker, atc.RegisterWorker, accessor.ViewerRole, false),

		Entry("owner :: "+atc.LandWorker, atc.LandWorker, accessor.OwnerRole, true),
		Entry("member :: "+atc.LandWorker, atc.LandWorker, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.LandWorker, atc.LandWorker, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.LandWorker, atc.LandWorker, accessor.ViewerRole, false),

		Entry("owner :: "+atc.RetireWorker, atc.RetireWorker, accessor.OwnerRole, true),
		Entry("member :: "+atc.RetireWorker, atc.RetireWorker, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.RetireWorker, atc.RetireWorker, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.RetireWorker, atc.RetireWorker, accessor.ViewerRole, false),

		Entry("owner :: "+atc.PruneWorker, atc.PruneWorker, accessor.OwnerRole, true),
		Entry("member :: "+atc.PruneWorker, atc.PruneWorker, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.PruneWorker, atc.PruneWorker, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.PruneWorker, atc.PruneWorker, accessor.ViewerRole, false),

		Entry("owner :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, accessor.OwnerRole, true),
		Entry("member :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListWorkers, atc.ListWorkers, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListWorkers, atc.ListWorkers, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListWorkers, atc.ListWorkers, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListWorkers, atc.ListWorkers, accessor.ViewerRole, true),

		Entry("owner :: "+atc.DeleteWorker, atc.DeleteWorker, accessor.OwnerRole, true),
		Entry("member :: "+atc.DeleteWorker, atc.DeleteWorker, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.DeleteWorker, atc.DeleteWorker, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.DeleteWorker, atc.DeleteWorker, accessor.ViewerRole, false),

		Entry("owner :: "+atc.SetLogLevel, atc.SetLogLevel, accessor.OwnerRole, true),
		Entry("member :: "+atc.SetLogLevel, atc.SetLogLevel, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.SetLogLevel, atc.SetLogLevel, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.SetLogLevel, atc.SetLogLevel, accessor.ViewerRole, false),

		Entry("owner :: "+atc.GetLogLevel, atc.GetLogLevel, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetLogLevel, atc.GetLogLevel, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetLogLevel, atc.GetLogLevel, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetLogLevel, atc.GetLogLevel, accessor.ViewerRole, true),

		Entry("owner :: "+atc.DownloadCLI, atc.DownloadCLI, accessor.OwnerRole, true),
		Entry("member :: "+atc.DownloadCLI, atc.DownloadCLI, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.DownloadCLI, atc.DownloadCLI, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.DownloadCLI, atc.DownloadCLI, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetInfo, atc.GetInfo, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetInfo, atc.GetInfo, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetInfo, atc.GetInfo, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetInfo, atc.GetInfo, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetInfoCreds, atc.GetInfoCreds, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetInfoCreds, atc.GetInfoCreds, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetInfoCreds, atc.GetInfoCreds, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetInfoCreds, atc.GetInfoCreds, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListContainers, atc.ListContainers, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListContainers, atc.ListContainers, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListContainers, atc.ListContainers, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListContainers, atc.ListContainers, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetContainer, atc.GetContainer, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetContainer, atc.GetContainer, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetContainer, atc.GetContainer, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetContainer, atc.GetContainer, accessor.ViewerRole, true),

		Entry("owner :: "+atc.HijackContainer, atc.HijackContainer, accessor.OwnerRole, true),
		Entry("member :: "+atc.HijackContainer, atc.HijackContainer, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.HijackContainer, atc.HijackContainer, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.HijackContainer, atc.HijackContainer, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, accessor.OwnerRole, true),
		Entry("member :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListVolumes, atc.ListVolumes, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListVolumes, atc.ListVolumes, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListVolumes, atc.ListVolumes, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListVolumes, atc.ListVolumes, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, accessor.ViewerRole, true),

		Entry("owner :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, accessor.OwnerRole, true),
		Entry("member :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListTeams, atc.ListTeams, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListTeams, atc.ListTeams, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListTeams, atc.ListTeams, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListTeams, atc.ListTeams, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetTeam, atc.GetTeam, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetTeam, atc.GetTeam, accessor.MemberRole, true),
		Entry("viewer :: "+atc.GetTeam, atc.GetTeam, accessor.ViewerRole, true),

		Entry("owner :: "+atc.SetTeam, atc.SetTeam, accessor.OwnerRole, true),
		Entry("member :: "+atc.SetTeam, atc.SetTeam, accessor.MemberRole, false),
		Entry("pipeline-operator :: "+atc.SetTeam, atc.SetTeam, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.SetTeam, atc.SetTeam, accessor.ViewerRole, false),

		Entry("owner :: "+atc.RenameTeam, atc.RenameTeam, accessor.OwnerRole, true),
		Entry("member :: "+atc.RenameTeam, atc.RenameTeam, accessor.MemberRole, false),
		Entry("pipeline-operator :: "+atc.RenameTeam, atc.RenameTeam, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.RenameTeam, atc.RenameTeam, accessor.ViewerRole, false),

		Entry("owner :: "+atc.DestroyTeam, atc.DestroyTeam, accessor.OwnerRole, true),
		Entry("member :: "+atc.DestroyTeam, atc.DestroyTeam, accessor.MemberRole, false),
		Entry("pipeline-operator :: "+atc.DestroyTeam, atc.DestroyTeam, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.DestroyTeam, atc.DestroyTeam, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, accessor.ViewerRole, true),

		Entry("owner :: "+atc.CreateArtifact, atc.CreateArtifact, accessor.OwnerRole, true),
		Entry("member :: "+atc.CreateArtifact, atc.CreateArtifact, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.CreateArtifact, atc.CreateArtifact, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.CreateArtifact, atc.CreateArtifact, accessor.ViewerRole, false),

		Entry("owner :: "+atc.GetArtifact, atc.GetArtifact, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetArtifact, atc.GetArtifact, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetArtifact, atc.GetArtifact, accessor.OperatorRole, false),
		Entry("viewer :: "+atc.GetArtifact, atc.GetArtifact, accessor.ViewerRole, false),

		Entry("owner :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, accessor.OwnerRole, true),
		Entry("member :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, accessor.ViewerRole, true),

		Entry("owner :: "+atc.GetWall, atc.GetWall, accessor.OwnerRole, true),
		Entry("member :: "+atc.GetWall, atc.GetWall, accessor.MemberRole, true),
		Entry("pipeline-operator :: "+atc.GetWall, atc.GetWall, accessor.OperatorRole, true),
		Entry("viewer :: "+atc.GetWall, atc.GetWall, accessor.ViewerRole, true),
	)

	Describe("Customize RBAC", func() {
		JustBeforeEach(func() {
			customData := accessor.CustomActionRoleMap{
				accessor.OperatorRole: []string{atc.HijackContainer, atc.CreatePipelineBuild},
			}

			logger := lager.NewLogger("test")
			err := accessorFactory.CustomizeActionRoleMap(logger, customData)
			Expect(err).NotTo(HaveOccurred())
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
			Entry("owner :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, accessor.OwnerRole, true),
			Entry("member :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, accessor.MemberRole, true),
			Entry("pipeline-operator :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, accessor.OperatorRole, true),
			Entry("viewer :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, accessor.ViewerRole, false),

			Entry("owner :: "+atc.HijackContainer, atc.HijackContainer, accessor.OwnerRole, true),
			Entry("member :: "+atc.HijackContainer, atc.HijackContainer, accessor.MemberRole, true),
			Entry("pipeline-operator :: "+atc.HijackContainer, atc.HijackContainer, accessor.OperatorRole, true),
			Entry("viewer :: "+atc.HijackContainer, atc.HijackContainer, accessor.ViewerRole, false),

			// Verify one un-customized action just in case.
			Entry("owner :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, accessor.OwnerRole, true),
			Entry("member :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, accessor.MemberRole, true),
			Entry("pipeline-operator :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, accessor.OperatorRole, true),
			Entry("viewer :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, accessor.ViewerRole, true),
		)
	})
})
