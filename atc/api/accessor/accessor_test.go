package accessor_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
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

	Describe("Is Authorized", func() {
		JustBeforeEach(func() {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access = accessorFactory.Create(req, atc.SetTeam)
		})

		Context("when request has team name claim set to some-team:admin", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{"some-team:admin"}}
			})
			It("returns true", func() {
				Expect(access.IsAuthorized("some-team")).To(BeTrue())
			})
		})

		Context("when request does not have a team role set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{"some-team"}}
			})
			It("returns true and defaults to role 'admin'", func() {
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

		Context("when request has team name claim set to other team:admin", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{"other-team:admin"}}
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

		Context("when request has teams claim set", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{"fake-team-name"}}
			})
			It("returns list of teams", func() {
				Expect(access.TeamNames()).To(Equal([]string{"fake-team-name"}))
			})
		})
		Context("when request has teams claim set to empty", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{""}}
			})
			It("returns empty list", func() {
				Expect(access.TeamNames()).To(Equal([]string{""}))
			})
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
		Context("when request has teams with multiple roles", func() {
			BeforeEach(func() {
				claims = &jwt.MapClaims{"teams": []string{"team-1:admin", "team-1:member", "team-2:viewer"}}
			})
			It("returns empty list", func() {
				Expect(access.TeamNames()).To(ConsistOf("team-1", "team-2"))
			})
		})
	})

	DescribeTable("role actions",
		func(action, role string, authorized bool) {
			claims := &jwt.MapClaims{"teams": []string{"some-team:" + role}}
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tokenString, err := token.SignedString(key)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			access := accessorFactory.Create(req, action)

			Expect(access.IsAuthorized("some-team")).To(Equal(authorized))
		},
		Entry("admin :: "+atc.SaveConfig, atc.SaveConfig, "admin", true),
		Entry("member :: "+atc.SaveConfig, atc.SaveConfig, "member", true),
		Entry("viewer :: "+atc.SaveConfig, atc.SaveConfig, "viewer", false),

		Entry("admin :: "+atc.GetConfig, atc.GetConfig, "admin", true),
		Entry("member :: "+atc.GetConfig, atc.GetConfig, "member", true),
		Entry("viewer :: "+atc.GetConfig, atc.GetConfig, "viewer", true),

		Entry("admin :: "+atc.GetBuild, atc.GetBuild, "admin", true),
		Entry("member :: "+atc.GetBuild, atc.GetBuild, "member", true),
		Entry("viewer :: "+atc.GetBuild, atc.GetBuild, "viewer", true),

		Entry("admin :: "+atc.GetBuildPlan, atc.GetBuildPlan, "admin", true),
		Entry("member :: "+atc.GetBuildPlan, atc.GetBuildPlan, "member", true),
		Entry("viewer :: "+atc.GetBuildPlan, atc.GetBuildPlan, "viewer", true),

		Entry("admin :: "+atc.CreateBuild, atc.CreateBuild, "admin", true),
		Entry("member :: "+atc.CreateBuild, atc.CreateBuild, "member", true),
		Entry("viewer :: "+atc.CreateBuild, atc.CreateBuild, "viewer", false),

		Entry("admin :: "+atc.ListBuilds, atc.ListBuilds, "admin", true),
		Entry("member :: "+atc.ListBuilds, atc.ListBuilds, "member", true),
		Entry("viewer :: "+atc.ListBuilds, atc.ListBuilds, "viewer", true),

		Entry("admin :: "+atc.BuildEvents, atc.BuildEvents, "admin", true),
		Entry("member :: "+atc.BuildEvents, atc.BuildEvents, "member", true),
		Entry("viewer :: "+atc.BuildEvents, atc.BuildEvents, "viewer", true),

		Entry("admin :: "+atc.BuildResources, atc.BuildResources, "admin", true),
		Entry("member :: "+atc.BuildResources, atc.BuildResources, "member", true),
		Entry("viewer :: "+atc.BuildResources, atc.BuildResources, "viewer", true),

		Entry("admin :: "+atc.AbortBuild, atc.AbortBuild, "admin", true),
		Entry("member :: "+atc.AbortBuild, atc.AbortBuild, "member", true),
		Entry("viewer :: "+atc.AbortBuild, atc.AbortBuild, "viewer", false),

		Entry("admin :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "admin", true),
		Entry("member :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "member", true),
		Entry("viewer :: "+atc.GetBuildPreparation, atc.GetBuildPreparation, "viewer", true),

		Entry("admin :: "+atc.GetJob, atc.GetJob, "admin", true),
		Entry("member :: "+atc.GetJob, atc.GetJob, "member", true),
		Entry("viewer :: "+atc.GetJob, atc.GetJob, "viewer", true),

		Entry("admin :: "+atc.CreateJobBuild, atc.CreateJobBuild, "admin", true),
		Entry("member :: "+atc.CreateJobBuild, atc.CreateJobBuild, "member", true),
		Entry("viewer :: "+atc.CreateJobBuild, atc.CreateJobBuild, "viewer", false),

		Entry("admin :: "+atc.ListAllJobs, atc.ListAllJobs, "admin", true),
		Entry("member :: "+atc.ListAllJobs, atc.ListAllJobs, "member", true),
		Entry("viewer :: "+atc.ListAllJobs, atc.ListAllJobs, "viewer", true),

		Entry("admin :: "+atc.ListJobs, atc.ListJobs, "admin", true),
		Entry("member :: "+atc.ListJobs, atc.ListJobs, "member", true),
		Entry("viewer :: "+atc.ListJobs, atc.ListJobs, "viewer", true),

		Entry("admin :: "+atc.ListJobBuilds, atc.ListJobBuilds, "admin", true),
		Entry("member :: "+atc.ListJobBuilds, atc.ListJobBuilds, "member", true),
		Entry("viewer :: "+atc.ListJobBuilds, atc.ListJobBuilds, "viewer", true),

		Entry("admin :: "+atc.ListJobInputs, atc.ListJobInputs, "admin", true),
		Entry("member :: "+atc.ListJobInputs, atc.ListJobInputs, "member", true),
		Entry("viewer :: "+atc.ListJobInputs, atc.ListJobInputs, "viewer", true),

		Entry("admin :: "+atc.GetJobBuild, atc.GetJobBuild, "admin", true),
		Entry("member :: "+atc.GetJobBuild, atc.GetJobBuild, "member", true),
		Entry("viewer :: "+atc.GetJobBuild, atc.GetJobBuild, "viewer", true),

		Entry("admin :: "+atc.PauseJob, atc.PauseJob, "admin", true),
		Entry("member :: "+atc.PauseJob, atc.PauseJob, "member", true),
		Entry("viewer :: "+atc.PauseJob, atc.PauseJob, "viewer", false),

		Entry("admin :: "+atc.UnpauseJob, atc.UnpauseJob, "admin", true),
		Entry("member :: "+atc.UnpauseJob, atc.UnpauseJob, "member", true),
		Entry("viewer :: "+atc.UnpauseJob, atc.UnpauseJob, "viewer", false),

		Entry("admin :: "+atc.GetVersionsDB, atc.GetVersionsDB, "admin", true),
		Entry("member :: "+atc.GetVersionsDB, atc.GetVersionsDB, "member", true),
		Entry("viewer :: "+atc.GetVersionsDB, atc.GetVersionsDB, "viewer", true),

		Entry("admin :: "+atc.JobBadge, atc.JobBadge, "admin", true),
		Entry("member :: "+atc.JobBadge, atc.JobBadge, "member", true),
		Entry("viewer :: "+atc.JobBadge, atc.JobBadge, "viewer", true),

		Entry("admin :: "+atc.MainJobBadge, atc.MainJobBadge, "admin", true),
		Entry("member :: "+atc.MainJobBadge, atc.MainJobBadge, "member", true),
		Entry("viewer :: "+atc.MainJobBadge, atc.MainJobBadge, "viewer", true),

		Entry("admin :: "+atc.ClearTaskCache, atc.ClearTaskCache, "admin", true),
		Entry("member :: "+atc.ClearTaskCache, atc.ClearTaskCache, "member", true),
		Entry("viewer :: "+atc.ClearTaskCache, atc.ClearTaskCache, "viewer", false),

		Entry("admin :: "+atc.ListAllResources, atc.ListAllResources, "admin", true),
		Entry("member :: "+atc.ListAllResources, atc.ListAllResources, "member", true),
		Entry("viewer :: "+atc.ListAllResources, atc.ListAllResources, "viewer", true),

		Entry("admin :: "+atc.ListResources, atc.ListResources, "admin", true),
		Entry("member :: "+atc.ListResources, atc.ListResources, "member", true),
		Entry("viewer :: "+atc.ListResources, atc.ListResources, "viewer", true),

		Entry("admin :: "+atc.ListResourceTypes, atc.ListResourceTypes, "admin", true),
		Entry("member :: "+atc.ListResourceTypes, atc.ListResourceTypes, "member", true),
		Entry("viewer :: "+atc.ListResourceTypes, atc.ListResourceTypes, "viewer", true),

		Entry("admin :: "+atc.GetResource, atc.GetResource, "admin", true),
		Entry("member :: "+atc.GetResource, atc.GetResource, "member", true),
		Entry("viewer :: "+atc.GetResource, atc.GetResource, "viewer", true),

		Entry("admin :: "+atc.PauseResource, atc.PauseResource, "admin", true),
		Entry("member :: "+atc.PauseResource, atc.PauseResource, "member", true),
		Entry("viewer :: "+atc.PauseResource, atc.PauseResource, "viewer", false),

		Entry("admin :: "+atc.UnpauseResource, atc.UnpauseResource, "admin", true),
		Entry("member :: "+atc.UnpauseResource, atc.UnpauseResource, "member", true),
		Entry("viewer :: "+atc.UnpauseResource, atc.UnpauseResource, "viewer", false),

		Entry("admin :: "+atc.CheckResource, atc.CheckResource, "admin", true),
		Entry("member :: "+atc.CheckResource, atc.CheckResource, "member", true),
		Entry("viewer :: "+atc.CheckResource, atc.CheckResource, "viewer", false),

		Entry("admin :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "admin", true),
		Entry("member :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "member", true),
		Entry("viewer :: "+atc.CheckResourceWebHook, atc.CheckResourceWebHook, "viewer", false),

		Entry("admin :: "+atc.CheckResourceType, atc.CheckResourceType, "admin", true),
		Entry("member :: "+atc.CheckResourceType, atc.CheckResourceType, "member", true),
		Entry("viewer :: "+atc.CheckResourceType, atc.CheckResourceType, "viewer", false),

		Entry("admin :: "+atc.ListResourceVersions, atc.ListResourceVersions, "admin", true),
		Entry("member :: "+atc.ListResourceVersions, atc.ListResourceVersions, "member", true),
		Entry("viewer :: "+atc.ListResourceVersions, atc.ListResourceVersions, "viewer", true),

		Entry("admin :: "+atc.GetResourceVersion, atc.GetResourceVersion, "admin", true),
		Entry("member :: "+atc.GetResourceVersion, atc.GetResourceVersion, "member", true),
		Entry("viewer :: "+atc.GetResourceVersion, atc.GetResourceVersion, "viewer", true),

		Entry("admin :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "admin", true),
		Entry("member :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "member", true),
		Entry("viewer :: "+atc.EnableResourceVersion, atc.EnableResourceVersion, "viewer", false),

		Entry("admin :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "admin", true),
		Entry("member :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "member", true),
		Entry("viewer :: "+atc.DisableResourceVersion, atc.DisableResourceVersion, "viewer", false),

		Entry("admin :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "admin", true),
		Entry("member :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "member", true),
		Entry("viewer :: "+atc.ListBuildsWithVersionAsInput, atc.ListBuildsWithVersionAsInput, "viewer", true),

		Entry("admin :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "admin", true),
		Entry("member :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "member", true),
		Entry("viewer :: "+atc.ListBuildsWithVersionAsOutput, atc.ListBuildsWithVersionAsOutput, "viewer", true),

		Entry("admin :: "+atc.GetResourceCausality, atc.GetResourceCausality, "admin", true),
		Entry("member :: "+atc.GetResourceCausality, atc.GetResourceCausality, "member", true),
		Entry("viewer :: "+atc.GetResourceCausality, atc.GetResourceCausality, "viewer", true),

		Entry("admin :: "+atc.ListAllPipelines, atc.ListAllPipelines, "admin", true),
		Entry("member :: "+atc.ListAllPipelines, atc.ListAllPipelines, "member", true),
		Entry("viewer :: "+atc.ListAllPipelines, atc.ListAllPipelines, "viewer", true),

		Entry("admin :: "+atc.ListPipelines, atc.ListPipelines, "admin", true),
		Entry("member :: "+atc.ListPipelines, atc.ListPipelines, "member", true),
		Entry("viewer :: "+atc.ListPipelines, atc.ListPipelines, "viewer", true),

		Entry("admin :: "+atc.GetPipeline, atc.GetPipeline, "admin", true),
		Entry("member :: "+atc.GetPipeline, atc.GetPipeline, "member", true),
		Entry("viewer :: "+atc.GetPipeline, atc.GetPipeline, "viewer", true),

		Entry("admin :: "+atc.DeletePipeline, atc.DeletePipeline, "admin", true),
		Entry("member :: "+atc.DeletePipeline, atc.DeletePipeline, "member", true),
		Entry("viewer :: "+atc.DeletePipeline, atc.DeletePipeline, "viewer", false),

		Entry("admin :: "+atc.OrderPipelines, atc.OrderPipelines, "admin", true),
		Entry("member :: "+atc.OrderPipelines, atc.OrderPipelines, "member", true),
		Entry("viewer :: "+atc.OrderPipelines, atc.OrderPipelines, "viewer", false),

		Entry("admin :: "+atc.PausePipeline, atc.PausePipeline, "admin", true),
		Entry("member :: "+atc.PausePipeline, atc.PausePipeline, "member", true),
		Entry("viewer :: "+atc.PausePipeline, atc.PausePipeline, "viewer", false),

		Entry("admin :: "+atc.UnpausePipeline, atc.UnpausePipeline, "admin", true),
		Entry("member :: "+atc.UnpausePipeline, atc.UnpausePipeline, "member", true),
		Entry("viewer :: "+atc.UnpausePipeline, atc.UnpausePipeline, "viewer", false),

		Entry("admin :: "+atc.ExposePipeline, atc.ExposePipeline, "admin", true),
		Entry("member :: "+atc.ExposePipeline, atc.ExposePipeline, "member", true),
		Entry("viewer :: "+atc.ExposePipeline, atc.ExposePipeline, "viewer", false),

		Entry("admin :: "+atc.HidePipeline, atc.HidePipeline, "admin", true),
		Entry("member :: "+atc.HidePipeline, atc.HidePipeline, "member", true),
		Entry("viewer :: "+atc.HidePipeline, atc.HidePipeline, "viewer", false),

		Entry("admin :: "+atc.RenamePipeline, atc.RenamePipeline, "admin", true),
		Entry("member :: "+atc.RenamePipeline, atc.RenamePipeline, "member", true),
		Entry("viewer :: "+atc.RenamePipeline, atc.RenamePipeline, "viewer", false),

		Entry("admin :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "admin", true),
		Entry("member :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "member", true),
		Entry("viewer :: "+atc.ListPipelineBuilds, atc.ListPipelineBuilds, "viewer", true),

		Entry("admin :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "admin", true),
		Entry("member :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "member", true),
		Entry("viewer :: "+atc.CreatePipelineBuild, atc.CreatePipelineBuild, "viewer", false),

		Entry("admin :: "+atc.PipelineBadge, atc.PipelineBadge, "admin", true),
		Entry("member :: "+atc.PipelineBadge, atc.PipelineBadge, "member", true),
		Entry("viewer :: "+atc.PipelineBadge, atc.PipelineBadge, "viewer", true),

		Entry("admin :: "+atc.RegisterWorker, atc.RegisterWorker, "admin", true),
		Entry("member :: "+atc.RegisterWorker, atc.RegisterWorker, "member", true),
		Entry("viewer :: "+atc.RegisterWorker, atc.RegisterWorker, "viewer", false),

		Entry("admin :: "+atc.LandWorker, atc.LandWorker, "admin", true),
		Entry("member :: "+atc.LandWorker, atc.LandWorker, "member", true),
		Entry("viewer :: "+atc.LandWorker, atc.LandWorker, "viewer", false),

		Entry("admin :: "+atc.RetireWorker, atc.RetireWorker, "admin", true),
		Entry("member :: "+atc.RetireWorker, atc.RetireWorker, "member", true),
		Entry("viewer :: "+atc.RetireWorker, atc.RetireWorker, "viewer", false),

		Entry("admin :: "+atc.PruneWorker, atc.PruneWorker, "admin", true),
		Entry("member :: "+atc.PruneWorker, atc.PruneWorker, "member", true),
		Entry("viewer :: "+atc.PruneWorker, atc.PruneWorker, "viewer", false),

		Entry("admin :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "admin", true),
		Entry("member :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "member", true),
		Entry("viewer :: "+atc.HeartbeatWorker, atc.HeartbeatWorker, "viewer", false),

		Entry("admin :: "+atc.ListWorkers, atc.ListWorkers, "admin", true),
		Entry("member :: "+atc.ListWorkers, atc.ListWorkers, "member", true),
		Entry("viewer :: "+atc.ListWorkers, atc.ListWorkers, "viewer", true),

		Entry("admin :: "+atc.DeleteWorker, atc.DeleteWorker, "admin", true),
		Entry("member :: "+atc.DeleteWorker, atc.DeleteWorker, "member", true),
		Entry("viewer :: "+atc.DeleteWorker, atc.DeleteWorker, "viewer", false),

		Entry("admin :: "+atc.SetLogLevel, atc.SetLogLevel, "admin", true),
		Entry("member :: "+atc.SetLogLevel, atc.SetLogLevel, "member", true),
		Entry("viewer :: "+atc.SetLogLevel, atc.SetLogLevel, "viewer", false),

		Entry("admin :: "+atc.GetLogLevel, atc.GetLogLevel, "admin", true),
		Entry("member :: "+atc.GetLogLevel, atc.GetLogLevel, "member", true),
		Entry("viewer :: "+atc.GetLogLevel, atc.GetLogLevel, "viewer", true),

		Entry("admin :: "+atc.DownloadCLI, atc.DownloadCLI, "admin", true),
		Entry("member :: "+atc.DownloadCLI, atc.DownloadCLI, "member", true),
		Entry("viewer :: "+atc.DownloadCLI, atc.DownloadCLI, "viewer", true),

		Entry("admin :: "+atc.GetInfo, atc.GetInfo, "admin", true),
		Entry("member :: "+atc.GetInfo, atc.GetInfo, "member", true),
		Entry("viewer :: "+atc.GetInfo, atc.GetInfo, "viewer", true),

		Entry("admin :: "+atc.GetInfoCreds, atc.GetInfoCreds, "admin", true),
		Entry("member :: "+atc.GetInfoCreds, atc.GetInfoCreds, "member", true),
		Entry("viewer :: "+atc.GetInfoCreds, atc.GetInfoCreds, "viewer", true),

		Entry("admin :: "+atc.ListContainers, atc.ListContainers, "admin", true),
		Entry("member :: "+atc.ListContainers, atc.ListContainers, "member", true),
		Entry("viewer :: "+atc.ListContainers, atc.ListContainers, "viewer", true),

		Entry("admin :: "+atc.GetContainer, atc.GetContainer, "admin", true),
		Entry("member :: "+atc.GetContainer, atc.GetContainer, "member", true),
		Entry("viewer :: "+atc.GetContainer, atc.GetContainer, "viewer", true),

		Entry("admin :: "+atc.HijackContainer, atc.HijackContainer, "admin", true),
		Entry("member :: "+atc.HijackContainer, atc.HijackContainer, "member", true),
		Entry("viewer :: "+atc.HijackContainer, atc.HijackContainer, "viewer", false),

		Entry("admin :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "admin", true),
		Entry("member :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "member", true),
		Entry("viewer :: "+atc.ListDestroyingContainers, atc.ListDestroyingContainers, "viewer", true),

		Entry("admin :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "admin", true),
		Entry("member :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "member", true),
		Entry("viewer :: "+atc.ReportWorkerContainers, atc.ReportWorkerContainers, "viewer", false),

		Entry("admin :: "+atc.ListVolumes, atc.ListVolumes, "admin", true),
		Entry("member :: "+atc.ListVolumes, atc.ListVolumes, "member", true),
		Entry("viewer :: "+atc.ListVolumes, atc.ListVolumes, "viewer", true),

		Entry("admin :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "admin", true),
		Entry("member :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "member", true),
		Entry("viewer :: "+atc.ListDestroyingVolumes, atc.ListDestroyingVolumes, "viewer", true),

		Entry("admin :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "admin", true),
		Entry("member :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "member", true),
		Entry("viewer :: "+atc.ReportWorkerVolumes, atc.ReportWorkerVolumes, "viewer", false),

		Entry("admin :: "+atc.ListTeams, atc.ListTeams, "admin", true),
		Entry("member :: "+atc.ListTeams, atc.ListTeams, "member", true),
		Entry("viewer :: "+atc.ListTeams, atc.ListTeams, "viewer", true),

		Entry("admin :: "+atc.SetTeam, atc.SetTeam, "admin", true),
		Entry("member :: "+atc.SetTeam, atc.SetTeam, "member", false),
		Entry("viewer :: "+atc.SetTeam, atc.SetTeam, "viewer", false),

		Entry("admin :: "+atc.RenameTeam, atc.RenameTeam, "admin", true),
		Entry("member :: "+atc.RenameTeam, atc.RenameTeam, "member", false),
		Entry("viewer :: "+atc.RenameTeam, atc.RenameTeam, "viewer", false),

		Entry("admin :: "+atc.DestroyTeam, atc.DestroyTeam, "admin", true),
		Entry("member :: "+atc.DestroyTeam, atc.DestroyTeam, "member", false),
		Entry("viewer :: "+atc.DestroyTeam, atc.DestroyTeam, "viewer", false),

		Entry("admin :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "admin", true),
		Entry("member :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "member", true),
		Entry("viewer :: "+atc.ListTeamBuilds, atc.ListTeamBuilds, "viewer", true),

		Entry("admin :: "+atc.SendInputToBuildPlan, atc.SendInputToBuildPlan, "admin", true),
		Entry("member :: "+atc.SendInputToBuildPlan, atc.SendInputToBuildPlan, "member", true),
		Entry("viewer :: "+atc.SendInputToBuildPlan, atc.SendInputToBuildPlan, "viewer", false),

		Entry("admin :: "+atc.ReadOutputFromBuildPlan, atc.ReadOutputFromBuildPlan, "admin", true),
		Entry("member :: "+atc.ReadOutputFromBuildPlan, atc.ReadOutputFromBuildPlan, "member", true),
		Entry("viewer :: "+atc.ReadOutputFromBuildPlan, atc.ReadOutputFromBuildPlan, "viewer", false),
	)
})
