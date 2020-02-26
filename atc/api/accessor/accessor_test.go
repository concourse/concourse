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

	Describe("Is System", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, "some-action")
		})

		Context("when request has System claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"System": true}
			})
			It("returns true", func() {
				Expect(access.IsSystem()).To(BeTrue())
			})
		})

		Context("when request has System claim set to empty", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"System": ""}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})

		Context("when request has System claim set to nil", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"System": nil}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})

		Context("when request has System claim set to false", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"System": false}
			})
			It("returns false", func() {
				Expect(access.IsSystem()).To(BeFalse())
			})
		})

		Context("when request does not have System claim set", func() {
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

	Describe("Is Authorized OwnerRole action", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, atc.SetTeam)
		})

		Context("when request has team name claim set for some-team as OwnerRole", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": map[string][]string{"some-team": {"OwnerRole"}}}
			})
			It("returns true", func() {
				Expect(access.IsAuthorized("some-team")).To(BeTrue())
			})
		})

		Context("when request does not have a team role set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{"some-team"}}
			})
			It("returns true and defaults to role 'OwnerRole'", func() {
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

		Context("when request has team name claim set to other-team:OwnerRole", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": map[string][]string{"other-team": {"OwnerRole"}}}
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
					"team-1": {"OwnerRole", "MEMBER_ROLE"},
					"team-2": {"ViewerRole"},
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
		Context("when request has the System claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"System": true}
			})
			It("returns as a System call", func() {
				Expect(access.UserName()).To(Equal("System"))
			})
		})
		Context("when request has the System claim set as false", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"System": false}
			})
			It("returns as a System call", func() {
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
		Entry("OwnerRole :: table has no entry", "some-role", "OwnerRole", false),
		Entry("MEMBER_ROLE :: table has no entry", "some-role", "MEMBER_ROLE", false),
		Entry("pipeline-OperatorRole :: table has no entry", "some-role", "pipeline-OperatorRole", false),
		Entry("ViewerRole :: table has no entry", "some-role", "ViewerRole", false),

		Entry("OwnerRole :: "+atc.SaveConfig, atc.SaveConfig, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.SaveConfig, atc.SaveConfig, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.SaveConfig, atc.SaveConfig, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.SaveConfig, atc.SaveConfig, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.GetConfig, atc.GetConfig, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetConfig, atc.GetConfig, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetConfig, atc.GetConfig, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetConfig, atc.GetConfig, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetCC, atc.GetCC, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetCC, atc.GetCC, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetCC, atc.GetCC, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetCC, atc.GetCC, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetBuild, atc.GetBuild, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetBuild, atc.GetBuild, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetBuild, atc.GetBuild, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetBuild, atc.GetBuild, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetBuildPlan, atc.GetBuildPlan, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetBuildPlan, atc.GetBuildPlan, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetBuildPlan, atc.GetBuildPlan, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetBuildPlan, atc.GetBuildPlan, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.CreateBuild, atc.CreateBuild, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.CreateBuild, atc.CreateBuild, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.CreateBuild, atc.CreateBuild, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.CreateBuild, atc.CreateBuild, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListBuilds, atc.ListBuilds, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListBuilds, atc.ListBuilds, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListBuilds, atc.ListBuilds, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListBuilds, atc.ListBuilds, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.BuildEvents, atc.BuildEvents, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.BuildEvents, atc.BuildEvents, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.BuildEvents, atc.BuildEvents, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.BuildEvents, atc.BuildEvents, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.BuildResources, atc.BuildResources, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.BuildResources, atc.BuildResources, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.BuildResources, atc.BuildResources, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.BuildResources, atc.BuildResources, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.AbortBuild, atc.AbortBuild, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.AbortBuild, atc.AbortBuild, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.AbortBuild, atc.AbortBuild, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.AbortBuild, atc.AbortBuild, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetJob, atc.GetJob, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetJob, atc.GetJob, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetJob, atc.GetJob, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetJob, atc.GetJob, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.CreateJobBuild, atc.CreateJobBuild, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.CreateJobBuild, atc.CreateJobBuild, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.CreateJobBuild, atc.CreateJobBuild, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.CreateJobBuild, atc.CreateJobBuild, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.RerunJobBuild, atc.RerunJobBuild, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.RerunJobBuild, atc.RerunJobBuild, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.RerunJobBuild, atc.RerunJobBuild, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.RerunJobBuild, atc.RerunJobBuild, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListAllJobs, atc.ListAllJobs, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListAllJobs, atc.ListAllJobs, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListAllJobs, atc.ListAllJobs, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListAllJobs, atc.ListAllJobs, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListJobs, atc.ListJobs, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListJobs, atc.ListJobs, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListJobs, atc.ListJobs, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListJobs, atc.ListJobs, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListJobBuilds, atc.ListJobBuilds, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListJobBuilds, atc.ListJobBuilds, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListJobBuilds, atc.ListJobBuilds, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListJobBuilds, atc.ListJobBuilds, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListJobInputs, atc.ListJobInputs, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListJobInputs, atc.ListJobInputs, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListJobInputs, atc.ListJobInputs, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListJobInputs, atc.ListJobInputs, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetJobBuild, atc.GetJobBuild, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetJobBuild, atc.GetJobBuild, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetJobBuild, atc.GetJobBuild, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetJobBuild, atc.GetJobBuild, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.PauseJob, atc.PauseJob, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.PauseJob, atc.PauseJob, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.PauseJob, atc.PauseJob, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.PauseJob, atc.PauseJob, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.UnpauseJob, atc.UnpauseJob, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.UnpauseJob, atc.UnpauseJob, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.UnpauseJob, atc.UnpauseJob, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.UnpauseJob, atc.UnpauseJob, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ScheduleJob, atc.ScheduleJob, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ScheduleJob, atc.ScheduleJob, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ScheduleJob, atc.ScheduleJob, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ScheduleJob, atc.ScheduleJob, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.GetVersionsDB, atc.GetVersionsDB, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetVersionsDB, atc.GetVersionsDB, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetVersionsDB, atc.GetVersionsDB, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetVersionsDB, atc.GetVersionsDB, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.JobBadge, atc.JobBadge, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.JobBadge, atc.JobBadge, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.JobBadge, atc.JobBadge, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.JobBadge, atc.JobBadge, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.MainJobBadge, atc.MainJobBadge, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.MainJobBadge, atc.MainJobBadge, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.MainJobBadge, atc.MainJobBadge, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.MainJobBadge, atc.MainJobBadge, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ClearTaskCache, atc.ClearTaskCache, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ClearTaskCache, atc.ClearTaskCache, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ClearTaskCache, atc.ClearTaskCache, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ClearTaskCache, atc.ClearTaskCache, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListAllResources, atc.ListAllResources, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListAllResources, atc.ListAllResources, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListAllResources, atc.ListAllResources, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListAllResources, atc.ListAllResources, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListResources, atc.ListResources, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListResources, atc.ListResources, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListResources, atc.ListResources, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListResources, atc.ListResources, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListResourceTypes, atc.ListResourceTypes, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListResourceTypes, atc.ListResourceTypes, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListResourceTypes, atc.ListResourceTypes, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListResourceTypes, atc.ListResourceTypes, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetResource, atc.GetResource, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetResource, atc.GetResource, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetResource, atc.GetResource, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetResource, atc.GetResource, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.CheckResource, atc.CheckResource, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.CheckResource, atc.CheckResource, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.CheckResource, atc.CheckResource, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.CheckResource, atc.CheckResource, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.CheckResourceType, atc.CheckResourceType, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.CheckResourceType, atc.CheckResourceType, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.CheckResourceType, atc.CheckResourceType, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.CheckResourceType, atc.CheckResourceType, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListResourceVersions, atc.ListResourceVersions, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListResourceVersions, atc.ListResourceVersions, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListResourceVersions, atc.ListResourceVersions, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListResourceVersions, atc.ListResourceVersions, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetResourceVersion, atc.GetResourceVersion, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetResourceVersion, atc.GetResourceVersion, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetResourceVersion, atc.GetResourceVersion, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetResourceVersion, atc.GetResourceVersion, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetResourceCausality, atc.GetResourceCausality, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetResourceCausality, atc.GetResourceCausality, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetResourceCausality, atc.GetResourceCausality, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetResourceCausality, atc.GetResourceCausality, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListAllPipelines, atc.ListAllPipelines, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListAllPipelines, atc.ListAllPipelines, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListAllPipelines, atc.ListAllPipelines, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListAllPipelines, atc.ListAllPipelines, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListPipelines, atc.ListPipelines, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListPipelines, atc.ListPipelines, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListPipelines, atc.ListPipelines, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListPipelines, atc.ListPipelines, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetPipeline, atc.GetPipeline, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetPipeline, atc.GetPipeline, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetPipeline, atc.GetPipeline, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetPipeline, atc.GetPipeline, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.DeletePipeline, atc.DeletePipeline, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.DeletePipeline, atc.DeletePipeline, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.DeletePipeline, atc.DeletePipeline, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.DeletePipeline, atc.DeletePipeline, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.OrderPipelines, atc.OrderPipelines, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.OrderPipelines, atc.OrderPipelines, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.OrderPipelines, atc.OrderPipelines, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.OrderPipelines, atc.OrderPipelines, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.PausePipeline, atc.PausePipeline, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.PausePipeline, atc.PausePipeline, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.PausePipeline, atc.PausePipeline, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.PausePipeline, atc.PausePipeline, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.UnpausePipeline, atc.UnpausePipeline, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.UnpausePipeline, atc.UnpausePipeline, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.UnpausePipeline, atc.UnpausePipeline, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.UnpausePipeline, atc.UnpausePipeline, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ExposePipeline, atc.ExposePipeline, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ExposePipeline, atc.ExposePipeline, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ExposePipeline, atc.ExposePipeline, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.ExposePipeline, atc.ExposePipeline, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.HidePipeline, atc.HidePipeline, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.HidePipeline, atc.HidePipeline, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.HidePipeline, atc.HidePipeline, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.HidePipeline, atc.HidePipeline, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.RenamePipeline, atc.RenamePipeline, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.RenamePipeline, atc.RenamePipeline, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.RenamePipeline, atc.RenamePipeline, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.RenamePipeline, atc.RenamePipeline, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.PipelineBadge, atc.PipelineBadge, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.PipelineBadge, atc.PipelineBadge, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.PipelineBadge, atc.PipelineBadge, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.PipelineBadge, atc.PipelineBadge, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.RegisterWorker, atc.RegisterWorker, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.RegisterWorker, atc.RegisterWorker, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.RegisterWorker, atc.RegisterWorker, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.RegisterWorker, atc.RegisterWorker, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.LandWorker, atc.LandWorker, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.LandWorker, atc.LandWorker, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.LandWorker, atc.LandWorker, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.LandWorker, atc.LandWorker, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.RetireWorker, atc.RetireWorker, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.RetireWorker, atc.RetireWorker, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.RetireWorker, atc.RetireWorker, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.RetireWorker, atc.RetireWorker, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.PruneWorker, atc.PruneWorker, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.PruneWorker, atc.PruneWorker, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.PruneWorker, atc.PruneWorker, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.PruneWorker, atc.PruneWorker, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListWorkers, atc.ListWorkers, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListWorkers, atc.ListWorkers, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListWorkers, atc.ListWorkers, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListWorkers, atc.ListWorkers, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.DeleteWorker, atc.DeleteWorker, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.DeleteWorker, atc.DeleteWorker, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.DeleteWorker, atc.DeleteWorker, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.DeleteWorker, atc.DeleteWorker, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.SetLogLevel, atc.SetLogLevel, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.SetLogLevel, atc.SetLogLevel, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.SetLogLevel, atc.SetLogLevel, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.SetLogLevel, atc.SetLogLevel, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.GetLogLevel, atc.GetLogLevel, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetLogLevel, atc.GetLogLevel, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetLogLevel, atc.GetLogLevel, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetLogLevel, atc.GetLogLevel, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.DownloadCLI, atc.DownloadCLI, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.DownloadCLI, atc.DownloadCLI, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.DownloadCLI, atc.DownloadCLI, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.DownloadCLI, atc.DownloadCLI, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetInfo, atc.GetInfo, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetInfo, atc.GetInfo, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetInfo, atc.GetInfo, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetInfo, atc.GetInfo, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetInfoCreds, atc.GetInfoCreds, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetInfoCreds, atc.GetInfoCreds, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetInfoCreds, atc.GetInfoCreds, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetInfoCreds, atc.GetInfoCreds, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListContainers, atc.ListContainers, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListContainers, atc.ListContainers, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListContainers, atc.ListContainers, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListContainers, atc.ListContainers, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetContainer, atc.GetContainer, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetContainer, atc.GetContainer, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetContainer, atc.GetContainer, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetContainer, atc.GetContainer, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.HijackContainer, atc.HijackContainer, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.HijackContainer, atc.HijackContainer, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.HijackContainer, atc.HijackContainer, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.HijackContainer, atc.HijackContainer, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListVolumes, atc.ListVolumes, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListVolumes, atc.ListVolumes, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListVolumes, atc.ListVolumes, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListVolumes, atc.ListVolumes, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListTeams, atc.ListTeams, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListTeams, atc.ListTeams, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListTeams, atc.ListTeams, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListTeams, atc.ListTeams, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetTeam, atc.GetTeam, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetTeam, atc.GetTeam, "MEMBER_ROLE", true),
		Entry("ViewerRole :: "+atc.GetTeam, atc.GetTeam, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.SetTeam, atc.SetTeam, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.SetTeam, atc.SetTeam, "MEMBER_ROLE", false),
		Entry("pipeline-OperatorRole :: "+atc.SetTeam, atc.SetTeam, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.SetTeam, atc.SetTeam, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.RenameTeam, atc.RenameTeam, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.RenameTeam, atc.RenameTeam, "MEMBER_ROLE", false),
		Entry("pipeline-OperatorRole :: "+atc.RenameTeam, atc.RenameTeam, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.RenameTeam, atc.RenameTeam, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.DestroyTeam, atc.DestroyTeam, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.DestroyTeam, atc.DestroyTeam, "MEMBER_ROLE", false),
		Entry("pipeline-OperatorRole :: "+atc.DestroyTeam, atc.DestroyTeam, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.DestroyTeam, atc.DestroyTeam, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.CreateArtifact, atc.CreateArtifact, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.CreateArtifact, atc.CreateArtifact, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.CreateArtifact, atc.CreateArtifact, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.CreateArtifact, atc.CreateArtifact, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.GetArtifact, atc.GetArtifact, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetArtifact, atc.GetArtifact, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetArtifact, atc.GetArtifact, "pipeline-OperatorRole", false),
		Entry("ViewerRole :: "+atc.GetArtifact, atc.GetArtifact, "ViewerRole", false),

		Entry("OwnerRole :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "ViewerRole", true),

		Entry("OwnerRole :: "+atc.GetWall, atc.GetWall, "OwnerRole", true),
		Entry("MEMBER_ROLE :: "+atc.GetWall, atc.GetWall, "MEMBER_ROLE", true),
		Entry("pipeline-OperatorRole :: "+atc.GetWall, atc.GetWall, "pipeline-OperatorRole", true),
		Entry("ViewerRole :: "+atc.GetWall, atc.GetWall, "ViewerRole", true),
	)

	Describe("Customize RBAC", func() {
		JustBeforeEach(func() {
			customData := accessor.CustomActionRoleMap{
				"pipeline-OperatorRole": []string{atc.HijackContainer, atc.CreatePipelineBuild},
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
			Entry("OwnerRole :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "OwnerRole", true),
			Entry("MEMBER_ROLE :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "MEMBER_ROLE", true),
			Entry("pipeline-OperatorRole :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "pipeline-OperatorRole", true),
			Entry("ViewerRole :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "ViewerRole", false),

			Entry("OwnerRole :: "+atc.HijackContainer, atc.HijackContainer, "OwnerRole", true),
			Entry("MEMBER_ROLE :: "+atc.HijackContainer, atc.HijackContainer, "MEMBER_ROLE", true),
			Entry("pipeline-OperatorRole :: "+atc.HijackContainer, atc.HijackContainer, "pipeline-OperatorRole", true),
			Entry("ViewerRole :: "+atc.HijackContainer, atc.HijackContainer, "ViewerRole", false),

			// Verify one un-customized action just in case.
			Entry("OwnerRole :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "OwnerRole", true),
			Entry("MEMBER_ROLE :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "MEMBER_ROLE", true),
			Entry("pipeline-OperatorRole :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "pipeline-OperatorRole", true),
			Entry("ViewerRole :: "+atc.ListBuildArtifacts, atc.ListBuildArtifacts, "ViewerRole", true),
		)
	})
})
