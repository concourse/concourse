package wrappa_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("APIAuthWrappa", func() {
	var (
		fakeAuthValidator                       auth.Validator
		fakeGetTokenValidator                   auth.Validator
		fakeUserContextReader                   *authfakes.FakeUserContextReader
		fakeCheckPipelineAccessHandlerFactory   auth.CheckPipelineAccessHandlerFactory
		fakeCheckBuildReadAccessHandlerFactory  auth.CheckBuildReadAccessHandlerFactory
		fakeCheckBuildWriteAccessHandlerFactory auth.CheckBuildWriteAccessHandlerFactory
		fakeCheckWorkerTeamAccessHandlerFactory auth.CheckWorkerTeamAccessHandlerFactory
	)

	BeforeEach(func() {
		fakeAuthValidator = new(authfakes.FakeValidator)
		fakeGetTokenValidator = new(authfakes.FakeValidator)
		fakeUserContextReader = new(authfakes.FakeUserContextReader)
		pipelineDBFactory := new(dbfakes.FakePipelineDBFactory)
		teamDBFactory := new(dbfakes.FakeTeamDBFactory)
		workerFactory := new(dbngfakes.FakeWorkerFactory)
		fakeCheckPipelineAccessHandlerFactory = auth.NewCheckPipelineAccessHandlerFactory(
			pipelineDBFactory,
			teamDBFactory,
		)

		buildsDB := new(authfakes.FakeBuildsDB)
		fakeCheckBuildReadAccessHandlerFactory = auth.NewCheckBuildReadAccessHandlerFactory(buildsDB)
		fakeCheckBuildWriteAccessHandlerFactory = auth.NewCheckBuildWriteAccessHandlerFactory(buildsDB)
		fakeCheckWorkerTeamAccessHandlerFactory = auth.NewCheckWorkerTeamAccessHandlerFactory(workerFactory)
	})

	unauthenticated := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			handler,
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	authenticated := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthenticationHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	authenticatedAndAdmin := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAdminHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	authenticatedWithGetTokenValidator := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthenticationHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeGetTokenValidator,
			fakeUserContextReader,
		)
	}

	authorized := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthorizationHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	openForPublicPipelineOrAuthorized := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			fakeCheckPipelineAccessHandlerFactory.HandlerFor(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	doesNotCheckIfPrivateJob := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			fakeCheckBuildReadAccessHandlerFactory.AnyJobHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	checksIfPrivateJob := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			fakeCheckBuildReadAccessHandlerFactory.CheckIfPrivateJobHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	checkWritePermissionForBuild := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			fakeCheckBuildWriteAccessHandlerFactory.HandlerFor(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	checkTeamAccessForWorker := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			fakeCheckWorkerTeamAccessHandlerFactory.HandlerFor(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
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
				// unauthenticated / delegating to handler
				atc.GetInfo:          unauthenticated(inputHandlers[atc.GetInfo]),
				atc.DownloadCLI:      unauthenticated(inputHandlers[atc.DownloadCLI]),
				atc.ListAuthMethods:  unauthenticated(inputHandlers[atc.ListAuthMethods]),
				atc.ListAllPipelines: unauthenticated(inputHandlers[atc.ListAllPipelines]),
				atc.ListBuilds:       unauthenticated(inputHandlers[atc.ListBuilds]),
				atc.ListPipelines:    unauthenticated(inputHandlers[atc.ListPipelines]),
				atc.ListTeams:        unauthenticated(inputHandlers[atc.ListTeams]),
				atc.MainJobBadge:     unauthenticated(inputHandlers[atc.MainJobBadge]),

				// authorized or public pipeline
				atc.GetBuild:       doesNotCheckIfPrivateJob(inputHandlers[atc.GetBuild]),
				atc.BuildResources: doesNotCheckIfPrivateJob(inputHandlers[atc.BuildResources]),
				atc.GetBuildPlan:   doesNotCheckIfPrivateJob(inputHandlers[atc.GetBuildPlan]),

				// authorized or public pipeline and public job
				atc.BuildEvents:         checksIfPrivateJob(inputHandlers[atc.BuildEvents]),
				atc.GetBuildPreparation: checksIfPrivateJob(inputHandlers[atc.GetBuildPreparation]),

				// resource belongs to authorized team
				atc.AbortBuild: checkWritePermissionForBuild(inputHandlers[atc.AbortBuild]),

				// resource belongs to authorized team
				atc.PruneWorker:  checkTeamAccessForWorker(inputHandlers[atc.PruneWorker]),
				atc.LandWorker:   checkTeamAccessForWorker(inputHandlers[atc.LandWorker]),
				atc.RetireWorker: checkTeamAccessForWorker(inputHandlers[atc.RetireWorker]),

				// belongs to public pipeline or authorized
				atc.GetPipeline:                   openForPublicPipelineOrAuthorized(inputHandlers[atc.GetPipeline]),
				atc.GetJobBuild:                   openForPublicPipelineOrAuthorized(inputHandlers[atc.GetJobBuild]),
				atc.JobBadge:                      openForPublicPipelineOrAuthorized(inputHandlers[atc.JobBadge]),
				atc.ListJobs:                      openForPublicPipelineOrAuthorized(inputHandlers[atc.ListJobs]),
				atc.GetJob:                        openForPublicPipelineOrAuthorized(inputHandlers[atc.GetJob]),
				atc.ListJobBuilds:                 openForPublicPipelineOrAuthorized(inputHandlers[atc.ListJobBuilds]),
				atc.GetResource:                   openForPublicPipelineOrAuthorized(inputHandlers[atc.GetResource]),
				atc.ListBuildsWithVersionAsInput:  openForPublicPipelineOrAuthorized(inputHandlers[atc.ListBuildsWithVersionAsInput]),
				atc.ListBuildsWithVersionAsOutput: openForPublicPipelineOrAuthorized(inputHandlers[atc.ListBuildsWithVersionAsOutput]),
				atc.ListResources:                 openForPublicPipelineOrAuthorized(inputHandlers[atc.ListResources]),
				atc.ListResourceVersions:          openForPublicPipelineOrAuthorized(inputHandlers[atc.ListResourceVersions]),

				// authenticated
				atc.CreateBuild:     authenticated(inputHandlers[atc.CreateBuild]),
				atc.CreatePipe:      authenticated(inputHandlers[atc.CreatePipe]),
				atc.GetAuthToken:    authenticatedWithGetTokenValidator(inputHandlers[atc.GetAuthToken]),
				atc.GetContainer:    authenticated(inputHandlers[atc.GetContainer]),
				atc.HijackContainer: authenticated(inputHandlers[atc.HijackContainer]),
				atc.ListContainers:  authenticated(inputHandlers[atc.ListContainers]),
				atc.ListVolumes:     authenticated(inputHandlers[atc.ListVolumes]),
				atc.ListWorkers:     authenticated(inputHandlers[atc.ListWorkers]),
				atc.ReadPipe:        authenticated(inputHandlers[atc.ReadPipe]),
				atc.RegisterWorker:  authenticated(inputHandlers[atc.RegisterWorker]),
				atc.HeartbeatWorker: authenticated(inputHandlers[atc.HeartbeatWorker]),
				atc.DeleteWorker:    authenticated(inputHandlers[atc.DeleteWorker]),

				atc.SetTeam:     authenticated(inputHandlers[atc.SetTeam]),
				atc.RenameTeam:  authenticated(inputHandlers[atc.RenameTeam]),
				atc.DestroyTeam: authenticated(inputHandlers[atc.DestroyTeam]),
				atc.WritePipe:   authenticated(inputHandlers[atc.WritePipe]),
				atc.GetUser:     authenticated(inputHandlers[atc.GetUser]),

				// authenticated and is admin
				atc.GetLogLevel: authenticatedAndAdmin(inputHandlers[atc.GetLogLevel]),
				atc.SetLogLevel: authenticatedAndAdmin(inputHandlers[atc.SetLogLevel]),

				// authorized (requested team matches resource team)
				atc.CheckResource:          authorized(inputHandlers[atc.CheckResource]),
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
			}
		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewAPIAuthWrappa(
				fakeAuthValidator,
				fakeGetTokenValidator,
				fakeUserContextReader,
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
