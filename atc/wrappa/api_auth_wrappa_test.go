package wrappa_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

var _ = Describe("APIAuthWrappa", func() {
	var (
		rejector                                auth.Rejector
		fakeCheckPipelineAccessHandlerFactory   auth.CheckPipelineAccessHandlerFactory
		fakeCheckBuildReadAccessHandlerFactory  auth.CheckBuildReadAccessHandlerFactory
		fakeCheckBuildWriteAccessHandlerFactory auth.CheckBuildWriteAccessHandlerFactory
		fakeCheckWorkerTeamAccessHandlerFactory auth.CheckWorkerTeamAccessHandlerFactory
		fakeBuildFactory                        *dbfakes.FakeBuildFactory
	)

	BeforeEach(func() {
		fakeTeamFactory := new(dbfakes.FakeTeamFactory)
		workerFactory := new(dbfakes.FakeWorkerFactory)
		fakeBuildFactory = new(dbfakes.FakeBuildFactory)
		fakeCheckPipelineAccessHandlerFactory = auth.NewCheckPipelineAccessHandlerFactory(
			fakeTeamFactory,
		)
		rejector = auth.UnauthorizedRejector{}

		fakeCheckBuildReadAccessHandlerFactory = auth.NewCheckBuildReadAccessHandlerFactory(fakeBuildFactory)
		fakeCheckBuildWriteAccessHandlerFactory = auth.NewCheckBuildWriteAccessHandlerFactory(fakeBuildFactory)
		fakeCheckWorkerTeamAccessHandlerFactory = auth.NewCheckWorkerTeamAccessHandlerFactory(workerFactory)
	})

	authenticateIfTokenProvided := func(handler http.Handler) http.Handler {
		return auth.CheckAuthenticationIfProvidedHandler(
			handler,
			rejector,
		)
	}

	authenticated := func(handler http.Handler) http.Handler {
		return auth.CheckAuthenticationHandler(
			handler,
			rejector,
		)
	}

	authenticatedAndAdmin := func(handler http.Handler) http.Handler {
		return auth.CheckAdminHandler(
			handler,
			rejector,
		)
	}

	authorized := func(handler http.Handler) http.Handler {
		return auth.CheckAuthorizationHandler(
			handler,
			rejector,
		)
	}

	openForPublicPipelineOrAuthorized := func(handler http.Handler) http.Handler {
		return fakeCheckPipelineAccessHandlerFactory.HandlerFor(
			handler,
			rejector,
		)
	}

	doesNotCheckIfPrivateJob := func(handler http.Handler) http.Handler {
		return fakeCheckBuildReadAccessHandlerFactory.AnyJobHandler(
			handler,
			rejector,
		)
	}

	checksIfPrivateJob := func(handler http.Handler) http.Handler {
		return fakeCheckBuildReadAccessHandlerFactory.CheckIfPrivateJobHandler(
			handler,
			rejector,
		)
	}

	checkWritePermissionForBuild := func(handler http.Handler) http.Handler {
		return fakeCheckBuildWriteAccessHandlerFactory.HandlerFor(
			handler,
			rejector,
		)
	}

	checkTeamAccessForWorker := func(handler http.Handler) http.Handler {
		return fakeCheckWorkerTeamAccessHandlerFactory.HandlerFor(
			handler,
			rejector,
		)
	}

	Describe("Wrap", func() {
		var (
			inputHandlers    rata.Handlers
			expectedHandlers rata.Handlers

			wrappedHandlers rata.Handlers
		)

		BeforeEach(func() {
			inputHandlers = rata.Handlers{}

			for _, route := range atc.Routes {
				inputHandlers[route.Name] = &stupidHandler{}
			}

			expectedHandlers = rata.Handlers{

				// authorized or public pipeline
				atc.GetBuild:       doesNotCheckIfPrivateJob(inputHandlers[atc.GetBuild]),
				atc.BuildResources: doesNotCheckIfPrivateJob(inputHandlers[atc.BuildResources]),

				// authorized or public pipeline and public job
				atc.BuildEvents:         checksIfPrivateJob(inputHandlers[atc.BuildEvents]),
				atc.ListBuildArtifacts:  checksIfPrivateJob(inputHandlers[atc.ListBuildArtifacts]),
				atc.GetBuildPreparation: checksIfPrivateJob(inputHandlers[atc.GetBuildPreparation]),
				atc.GetBuildPlan:        checksIfPrivateJob(inputHandlers[atc.GetBuildPlan]),

				// resource belongs to authorized team
				atc.AbortBuild: checkWritePermissionForBuild(inputHandlers[atc.AbortBuild]),

				// resource belongs to authorized team
				atc.PruneWorker:              checkTeamAccessForWorker(inputHandlers[atc.PruneWorker]),
				atc.LandWorker:               checkTeamAccessForWorker(inputHandlers[atc.LandWorker]),
				atc.ReportWorkerContainers:   checkTeamAccessForWorker(inputHandlers[atc.ReportWorkerContainers]),
				atc.ReportWorkerVolumes:      checkTeamAccessForWorker(inputHandlers[atc.ReportWorkerVolumes]),
				atc.RetireWorker:             checkTeamAccessForWorker(inputHandlers[atc.RetireWorker]),
				atc.ListDestroyingContainers: checkTeamAccessForWorker(inputHandlers[atc.ListDestroyingContainers]),
				atc.ListDestroyingVolumes:    checkTeamAccessForWorker(inputHandlers[atc.ListDestroyingVolumes]),

				// belongs to public pipeline or authorized
				atc.GetPipeline:                   openForPublicPipelineOrAuthorized(inputHandlers[atc.GetPipeline]),
				atc.GetJobBuild:                   openForPublicPipelineOrAuthorized(inputHandlers[atc.GetJobBuild]),
				atc.PipelineBadge:                 openForPublicPipelineOrAuthorized(inputHandlers[atc.PipelineBadge]),
				atc.JobBadge:                      openForPublicPipelineOrAuthorized(inputHandlers[atc.JobBadge]),
				atc.ListJobs:                      openForPublicPipelineOrAuthorized(inputHandlers[atc.ListJobs]),
				atc.GetJob:                        openForPublicPipelineOrAuthorized(inputHandlers[atc.GetJob]),
				atc.ListJobBuilds:                 openForPublicPipelineOrAuthorized(inputHandlers[atc.ListJobBuilds]),
				atc.ListPipelineBuilds:            openForPublicPipelineOrAuthorized(inputHandlers[atc.ListPipelineBuilds]),
				atc.GetResource:                   openForPublicPipelineOrAuthorized(inputHandlers[atc.GetResource]),
				atc.ListBuildsWithVersionAsInput:  openForPublicPipelineOrAuthorized(inputHandlers[atc.ListBuildsWithVersionAsInput]),
				atc.ListBuildsWithVersionAsOutput: openForPublicPipelineOrAuthorized(inputHandlers[atc.ListBuildsWithVersionAsOutput]),
				atc.ListResources:                 openForPublicPipelineOrAuthorized(inputHandlers[atc.ListResources]),
				atc.ListResourceTypes:             openForPublicPipelineOrAuthorized(inputHandlers[atc.ListResourceTypes]),
				atc.ListResourceVersions:          openForPublicPipelineOrAuthorized(inputHandlers[atc.ListResourceVersions]),
				atc.GetResourceCausality:          openForPublicPipelineOrAuthorized(inputHandlers[atc.GetResourceCausality]),
				atc.GetResourceVersion:            openForPublicPipelineOrAuthorized(inputHandlers[atc.GetResourceVersion]),

				// authenticated
				atc.CreateBuild:     authenticated(inputHandlers[atc.CreateBuild]),
				atc.GetContainer:    authenticated(inputHandlers[atc.GetContainer]),
				atc.HijackContainer: authenticated(inputHandlers[atc.HijackContainer]),
				atc.ListContainers:  authenticated(inputHandlers[atc.ListContainers]),
				atc.ListVolumes:     authenticated(inputHandlers[atc.ListVolumes]),
				atc.ListTeamBuilds:  authenticated(inputHandlers[atc.ListTeamBuilds]),
				atc.ListWorkers:     authenticated(inputHandlers[atc.ListWorkers]),
				atc.RegisterWorker:  authenticated(inputHandlers[atc.RegisterWorker]),
				atc.HeartbeatWorker: authenticated(inputHandlers[atc.HeartbeatWorker]),
				atc.DeleteWorker:    authenticated(inputHandlers[atc.DeleteWorker]),
				atc.GetTeam:         authenticated(inputHandlers[atc.GetTeam]),
				atc.SetTeam:         authenticated(inputHandlers[atc.SetTeam]),
				atc.RenameTeam:      authenticated(inputHandlers[atc.RenameTeam]),
				atc.DestroyTeam:     authenticated(inputHandlers[atc.DestroyTeam]),
				atc.GetUser:         authenticated(inputHandlers[atc.GetUser]),

				//authenticateIfTokenProvided / delegating to handler
				atc.GetInfo:              authenticateIfTokenProvided(inputHandlers[atc.GetInfo]),
				atc.GetCheck:             authenticateIfTokenProvided(inputHandlers[atc.GetCheck]),
				atc.DownloadCLI:          authenticateIfTokenProvided(inputHandlers[atc.DownloadCLI]),
				atc.CheckResourceWebHook: authenticateIfTokenProvided(inputHandlers[atc.CheckResourceWebHook]),
				atc.ListAllPipelines:     authenticateIfTokenProvided(inputHandlers[atc.ListAllPipelines]),
				atc.ListBuilds:           authenticateIfTokenProvided(inputHandlers[atc.ListBuilds]),
				atc.ListPipelines:        authenticateIfTokenProvided(inputHandlers[atc.ListPipelines]),
				atc.ListAllJobs:          authenticateIfTokenProvided(inputHandlers[atc.ListAllJobs]),
				atc.ListAllResources:     authenticateIfTokenProvided(inputHandlers[atc.ListAllResources]),
				atc.ListTeams:            authenticateIfTokenProvided(inputHandlers[atc.ListTeams]),
				atc.GetWall:              authenticateIfTokenProvided(inputHandlers[atc.GetWall]),

				// authenticated and is admin
				atc.GetLogLevel:          authenticatedAndAdmin(inputHandlers[atc.GetLogLevel]),
				atc.SetLogLevel:          authenticatedAndAdmin(inputHandlers[atc.SetLogLevel]),
				atc.GetInfoCreds:         authenticatedAndAdmin(inputHandlers[atc.GetInfoCreds]),
				atc.ListActiveUsersSince: authenticatedAndAdmin(inputHandlers[atc.ListActiveUsersSince]),
				atc.SetWall:              authenticatedAndAdmin(inputHandlers[atc.SetWall]),
				atc.ClearWall:            authenticatedAndAdmin(inputHandlers[atc.ClearWall]),

				// authorized (requested team matches resource team)
				atc.CheckResource:           authorized(inputHandlers[atc.CheckResource]),
				atc.CheckResourceType:       authorized(inputHandlers[atc.CheckResourceType]),
				atc.CreateJobBuild:          authorized(inputHandlers[atc.CreateJobBuild]),
				atc.RerunJobBuild:           authorized(inputHandlers[atc.RerunJobBuild]),
				atc.DeletePipeline:          authorized(inputHandlers[atc.DeletePipeline]),
				atc.DisableResourceVersion:  authorized(inputHandlers[atc.DisableResourceVersion]),
				atc.EnableResourceVersion:   authorized(inputHandlers[atc.EnableResourceVersion]),
				atc.PinResourceVersion:      authorized(inputHandlers[atc.PinResourceVersion]),
				atc.UnpinResource:           authorized(inputHandlers[atc.UnpinResource]),
				atc.SetPinCommentOnResource: authorized(inputHandlers[atc.SetPinCommentOnResource]),
				atc.GetConfig:               authorized(inputHandlers[atc.GetConfig]),
				atc.GetCC:                   authorized(inputHandlers[atc.GetCC]),
				atc.GetVersionsDB:           authorized(inputHandlers[atc.GetVersionsDB]),
				atc.ListJobInputs:           authorized(inputHandlers[atc.ListJobInputs]),
				atc.OrderPipelines:          authorized(inputHandlers[atc.OrderPipelines]),
				atc.PauseJob:                authorized(inputHandlers[atc.PauseJob]),
				atc.PausePipeline:           authorized(inputHandlers[atc.PausePipeline]),
				atc.ArchivePipeline:         authorized(inputHandlers[atc.ArchivePipeline]),
				atc.RenamePipeline:          authorized(inputHandlers[atc.RenamePipeline]),
				atc.SaveConfig:              authorized(inputHandlers[atc.SaveConfig]),
				atc.UnpauseJob:              authorized(inputHandlers[atc.UnpauseJob]),
				atc.ScheduleJob:             authorized(inputHandlers[atc.ScheduleJob]),
				atc.UnpausePipeline:         authorized(inputHandlers[atc.UnpausePipeline]),
				atc.ExposePipeline:          authorized(inputHandlers[atc.ExposePipeline]),
				atc.HidePipeline:            authorized(inputHandlers[atc.HidePipeline]),
				atc.CreatePipelineBuild:     authorized(inputHandlers[atc.CreatePipelineBuild]),
				atc.ClearTaskCache:          authorized(inputHandlers[atc.ClearTaskCache]),
				atc.CreateArtifact:          authorized(inputHandlers[atc.CreateArtifact]),
				atc.GetArtifact:             authorized(inputHandlers[atc.GetArtifact]),
			}
		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewAPIAuthWrappa(
				fakeCheckPipelineAccessHandlerFactory,
				fakeCheckBuildReadAccessHandlerFactory,
				fakeCheckBuildWriteAccessHandlerFactory,
				fakeCheckWorkerTeamAccessHandlerFactory,
			).Wrap(inputHandlers)

		})

		It("validates sensitive routes, and noop validates public routes", func() {
			for name, _ := range inputHandlers {
				Expect(wrappedHandlers[name]).To(BeIdenticalTo(expectedHandlers[name]))
			}
		})
	})
})
