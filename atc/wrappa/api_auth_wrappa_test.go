package wrappa_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	unauthenticated := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			handler,
			rejector,
		)
	}

	authenticated := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			auth.CheckAuthenticationHandler(
				handler,
				rejector,
			),
			rejector,
		)
	}

	authenticatedAndAdmin := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			auth.CheckAdminHandler(
				handler,
				rejector,
			),
			rejector,
		)
	}

	authorized := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			auth.CheckAuthorizationHandler(
				handler,
				rejector,
			),
			rejector,
		)
	}

	openForPublicPipelineOrAuthorized := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			fakeCheckPipelineAccessHandlerFactory.HandlerFor(
				handler,
				rejector,
			),
			rejector,
		)
	}

	doesNotCheckIfPrivateJob := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			fakeCheckBuildReadAccessHandlerFactory.AnyJobHandler(
				handler,
				rejector,
			),
			rejector,
		)
	}

	checksIfPrivateJob := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			fakeCheckBuildReadAccessHandlerFactory.CheckIfPrivateJobHandler(
				handler,
				rejector,
			),
			rejector,
		)
	}

	checkWritePermissionForBuild := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			fakeCheckBuildWriteAccessHandlerFactory.HandlerFor(
				handler,
				rejector,
			),
			rejector,
		)
	}

	checkTeamAccessForWorker := func(handler http.Handler) http.Handler {
		return auth.CSRFValidationHandler(
			fakeCheckWorkerTeamAccessHandlerFactory.HandlerFor(
				handler,
				rejector,
			),
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
				//unauthenticated / delegating to handler
				atc.GetInfo:              unauthenticated(inputHandlers[atc.GetInfo]),
				atc.DownloadCLI:          unauthenticated(inputHandlers[atc.DownloadCLI]),
				atc.CheckResourceWebHook: unauthenticated(inputHandlers[atc.CheckResourceWebHook]),
				atc.ListAllPipelines:     unauthenticated(inputHandlers[atc.ListAllPipelines]),
				atc.ListBuilds:           unauthenticated(inputHandlers[atc.ListBuilds]),
				atc.ListPipelines:        unauthenticated(inputHandlers[atc.ListPipelines]),
				atc.ListAllJobs:          unauthenticated(inputHandlers[atc.ListAllJobs]),
				atc.ListAllResources:     unauthenticated(inputHandlers[atc.ListAllResources]),
				atc.ListTeams:            unauthenticated(inputHandlers[atc.ListTeams]),
				atc.MainJobBadge:         unauthenticated(inputHandlers[atc.MainJobBadge]),

				// authorized or public pipeline
				atc.GetBuild:       doesNotCheckIfPrivateJob(inputHandlers[atc.GetBuild]),
				atc.BuildResources: doesNotCheckIfPrivateJob(inputHandlers[atc.BuildResources]),
				atc.GetBuildPlan:   doesNotCheckIfPrivateJob(inputHandlers[atc.GetBuildPlan]),

				// authorized or public pipeline and public job
				atc.BuildEvents:         checksIfPrivateJob(inputHandlers[atc.BuildEvents]),
				atc.GetBuildPreparation: checksIfPrivateJob(inputHandlers[atc.GetBuildPreparation]),

				// resource belongs to authorized team
				atc.AbortBuild:              checkWritePermissionForBuild(inputHandlers[atc.AbortBuild]),
				atc.SendInputToBuildPlan:    checkWritePermissionForBuild(inputHandlers[atc.SendInputToBuildPlan]),
				atc.ReadOutputFromBuildPlan: checkWritePermissionForBuild(inputHandlers[atc.ReadOutputFromBuildPlan]),

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
				atc.SetTeam:         authenticated(inputHandlers[atc.SetTeam]),
				atc.RenameTeam:      authenticated(inputHandlers[atc.RenameTeam]),
				atc.DestroyTeam:     authenticated(inputHandlers[atc.DestroyTeam]),

				// authenticated and is admin
				atc.GetLogLevel:  authenticatedAndAdmin(inputHandlers[atc.GetLogLevel]),
				atc.SetLogLevel:  authenticatedAndAdmin(inputHandlers[atc.SetLogLevel]),
				atc.GetInfoCreds: authenticatedAndAdmin(inputHandlers[atc.GetInfoCreds]),

				// authorized (requested team matches resource team)
				atc.CheckResource:          authorized(inputHandlers[atc.CheckResource]),
				atc.CheckResourceType:      authorized(inputHandlers[atc.CheckResourceType]),
				atc.CreateJobBuild:         authorized(inputHandlers[atc.CreateJobBuild]),
				atc.DeletePipeline:         authorized(inputHandlers[atc.DeletePipeline]),
				atc.DisableResourceVersion: authorized(inputHandlers[atc.DisableResourceVersion]),
				atc.EnableResourceVersion:  authorized(inputHandlers[atc.EnableResourceVersion]),
				atc.GetConfig:              authorized(inputHandlers[atc.GetConfig]),
				atc.GetVersionsDB:          authorized(inputHandlers[atc.GetVersionsDB]),
				atc.ListJobInputs:          authorized(inputHandlers[atc.ListJobInputs]),
				atc.OrderPipelines:         authorized(inputHandlers[atc.OrderPipelines]),
				atc.PauseJob:               authorized(inputHandlers[atc.PauseJob]),
				atc.PausePipeline:          authorized(inputHandlers[atc.PausePipeline]),
				atc.PauseResource:          authorized(inputHandlers[atc.PauseResource]),
				atc.RenamePipeline:         authorized(inputHandlers[atc.RenamePipeline]),
				atc.SaveConfig:             authorized(inputHandlers[atc.SaveConfig]),
				atc.UnpauseJob:             authorized(inputHandlers[atc.UnpauseJob]),
				atc.UnpausePipeline:        authorized(inputHandlers[atc.UnpausePipeline]),
				atc.UnpauseResource:        authorized(inputHandlers[atc.UnpauseResource]),
				atc.ExposePipeline:         authorized(inputHandlers[atc.ExposePipeline]),
				atc.HidePipeline:           authorized(inputHandlers[atc.HidePipeline]),
				atc.CreatePipelineBuild:    authorized(inputHandlers[atc.CreatePipelineBuild]),
				atc.ClearTaskCache:         authorized(inputHandlers[atc.ClearTaskCache]),
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
