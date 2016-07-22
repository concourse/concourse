package wrappa_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("APIAuthWrappa", func() {
	var (
		fakeValidator         *authfakes.FakeValidator
		fakeUserContextReader *authfakes.FakeUserContextReader
		publiclyViewable      bool
	)

	BeforeEach(func() {
		publiclyViewable = true
		fakeValidator = new(authfakes.FakeValidator)
		fakeUserContextReader = new(authfakes.FakeUserContextReader)
	})

	unauthenticated := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			handler,
			fakeValidator,
			fakeUserContextReader,
		)
	}

	authenticated := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthenticationHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeValidator,
			fakeUserContextReader,
		)
	}

	authorized := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthorizationHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeValidator,
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
				atc.GetInfo:                       unauthenticated(inputHandlers[atc.GetInfo]),
				atc.DownloadCLI:                   unauthenticated(inputHandlers[atc.DownloadCLI]),
				atc.ListAuthMethods:               unauthenticated(inputHandlers[atc.ListAuthMethods]),
				atc.BuildEvents:                   unauthenticated(inputHandlers[atc.BuildEvents]),
				atc.GetBuild:                      unauthenticated(inputHandlers[atc.GetBuild]),
				atc.BuildResources:                unauthenticated(inputHandlers[atc.BuildResources]),
				atc.GetBuildPlan:                  unauthenticated(inputHandlers[atc.GetBuildPlan]),
				atc.GetBuildPreparation:           unauthenticated(inputHandlers[atc.GetBuildPreparation]),
				atc.ListAllPipelines:              unauthenticated(inputHandlers[atc.ListAllPipelines]),
				atc.ListBuilds:                    unauthenticated(inputHandlers[atc.ListBuilds]),
				atc.GetJobBuild:                   unauthenticated(inputHandlers[atc.GetJobBuild]),
				atc.JobBadge:                      unauthenticated(inputHandlers[atc.JobBadge]),
				atc.ListJobs:                      unauthenticated(inputHandlers[atc.ListJobs]),
				atc.GetJob:                        unauthenticated(inputHandlers[atc.GetJob]),
				atc.ListJobBuilds:                 unauthenticated(inputHandlers[atc.ListJobBuilds]),
				atc.GetResource:                   unauthenticated(inputHandlers[atc.GetResource]),
				atc.ListBuildsWithVersionAsInput:  unauthenticated(inputHandlers[atc.ListBuildsWithVersionAsInput]),
				atc.ListBuildsWithVersionAsOutput: unauthenticated(inputHandlers[atc.ListBuildsWithVersionAsOutput]),
				atc.ListResources:                 unauthenticated(inputHandlers[atc.ListResources]),
				atc.ListResourceVersions:          unauthenticated(inputHandlers[atc.ListResourceVersions]),
				atc.ListPipelines:                 unauthenticated(inputHandlers[atc.ListPipelines]),
				atc.GetPipeline:                   unauthenticated(inputHandlers[atc.GetPipeline]),

				// authenticated
				atc.AbortBuild:      authenticated(inputHandlers[atc.AbortBuild]),
				atc.CreateBuild:     authenticated(inputHandlers[atc.CreateBuild]),
				atc.CreatePipe:      authenticated(inputHandlers[atc.CreatePipe]),
				atc.GetAuthToken:    authenticated(inputHandlers[atc.GetAuthToken]),
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
				fakeValidator,
				fakeUserContextReader,
			).Wrap(inputHandlers)
		})

		It("validates sensitive routes, and noop validates public routes", func() {
			for name, _ := range inputHandlers {
				Expect(descriptiveRoute{
					route:   name,
					handler: wrappedHandlers[name],
				}).To(Equal(descriptiveRoute{
					route:   name,
					handler: expectedHandlers[name],
				}))
			}
		})
	})
})
