package wrappa_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("APIAuthWrappa", func() {
	var (
		fakeAuthValidator                     auth.Validator
		fakeTokenValidator                    auth.Validator
		fakeUserContextReader                 *authfakes.FakeUserContextReader
		fakeCheckPipelineAccessHandlerFactory auth.CheckPipelineAccessHandlerFactory
		fakeCheckBuildAccessHandlerFactory    auth.CheckBuildAccessHandlerFactory
		publiclyViewable                      bool
	)

	BeforeEach(func() {
		publiclyViewable = true
		fakeAuthValidator = new(authfakes.FakeValidator)
		fakeTokenValidator = new(authfakes.FakeValidator)
		fakeUserContextReader = new(authfakes.FakeUserContextReader)
		pipelineDBFactory := new(dbfakes.FakePipelineDBFactory)
		teamDBFactory := new(dbfakes.FakeTeamDBFactory)
		fakeCheckPipelineAccessHandlerFactory = auth.NewCheckPipelineAccessHandlerFactory(
			pipelineDBFactory,
			teamDBFactory,
		)

		buildsDB := new(authfakes.FakeBuildsDB)
		fakeCheckBuildAccessHandlerFactory = auth.NewCheckBuildAccessHandlerFactory(buildsDB)
	})

	unauthenticated := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			handler,
			fakeTokenValidator,
			fakeUserContextReader,
		)
	}

	authenticated := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthenticationHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeTokenValidator,
			fakeUserContextReader,
		)
	}

	authenticatedWithAuthValidator := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthenticationHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeAuthValidator,
			fakeUserContextReader,
		)
	}

	authorized := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthorizationHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeTokenValidator,
			fakeUserContextReader,
		)
	}

	openForPublicPipelineOrAuthorized := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			fakeCheckPipelineAccessHandlerFactory.HandlerFor(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeTokenValidator,
			fakeUserContextReader,
		)
	}

	doesNotCheckIfPrivateJob := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			fakeCheckBuildAccessHandlerFactory.AnyJobHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeTokenValidator,
			fakeUserContextReader,
		)
	}

	checksIfPrivateJob := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			fakeCheckBuildAccessHandlerFactory.CheckIfPrivateJobHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeTokenValidator,
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

				// authorized or public pipeline
				atc.GetBuild:       doesNotCheckIfPrivateJob(inputHandlers[atc.GetBuild]),
				atc.BuildResources: doesNotCheckIfPrivateJob(inputHandlers[atc.BuildResources]),
				atc.GetBuildPlan:   doesNotCheckIfPrivateJob(inputHandlers[atc.GetBuildPlan]),

				// authorized or public pipeline and public job
				atc.BuildEvents:         checksIfPrivateJob(inputHandlers[atc.BuildEvents]),
				atc.GetBuildPreparation: checksIfPrivateJob(inputHandlers[atc.GetBuildPreparation]),

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
				atc.AbortBuild:      authenticated(inputHandlers[atc.AbortBuild]),
				atc.CreateBuild:     authenticated(inputHandlers[atc.CreateBuild]),
				atc.CreatePipe:      authenticated(inputHandlers[atc.CreatePipe]),
				atc.GetAuthToken:    authenticatedWithAuthValidator(inputHandlers[atc.GetAuthToken]),
				atc.GetContainer:    authenticated(inputHandlers[atc.GetContainer]),
				atc.GetLogLevel:     authenticated(inputHandlers[atc.GetLogLevel]),
				atc.HijackContainer: authenticated(inputHandlers[atc.HijackContainer]),
				atc.ListContainers:  authenticated(inputHandlers[atc.ListContainers]),
				atc.ListVolumes:     authenticated(inputHandlers[atc.ListVolumes]),
				atc.ListWorkers:     authenticated(inputHandlers[atc.ListWorkers]),
				atc.ReadPipe:        authenticated(inputHandlers[atc.ReadPipe]),
				atc.RegisterWorker:  authenticated(inputHandlers[atc.RegisterWorker]),
				atc.SetLogLevel:     authenticated(inputHandlers[atc.SetLogLevel]),
				atc.SetTeam:         authenticated(inputHandlers[atc.SetTeam]),
				atc.WritePipe:       authenticated(inputHandlers[atc.WritePipe]),

				// authorized
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
				atc.RevealPipeline:         authorized(inputHandlers[atc.RevealPipeline]),
				atc.ConcealPipeline:        authorized(inputHandlers[atc.ConcealPipeline]),
			}
		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewAPIAuthWrappa(
				fakeAuthValidator,
				fakeTokenValidator,
				fakeUserContextReader,
				fakeCheckPipelineAccessHandlerFactory,
				fakeCheckBuildAccessHandlerFactory,
			).Wrap(inputHandlers)
		})

		It("validates sensitive routes, and noop validates public routes", func() {
			for name, _ := range inputHandlers {
				Expect(wrappedHandlers[name]).To(BeIdenticalTo(expectedHandlers[name]))
			}
		})
	})
})
