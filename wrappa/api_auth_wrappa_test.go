package wrappa_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/fakes"
	"github.com/concourse/atc/wrappa"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("APIAuthWrappa", func() {
	var (
		publiclyViewable      bool
		fakeValidator         *fakes.FakeValidator
		fakeUserContextReader *fakes.FakeUserContextReader
	)

	BeforeEach(func() {
		publiclyViewable = true
		fakeValidator = new(fakes.FakeValidator)
		fakeUserContextReader = new(fakes.FakeUserContextReader)
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
			auth.CheckAuthHandler(
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
			inputHandlers rata.Handlers

			wrappedHandlers rata.Handlers
		)

		BeforeEach(func() {
			inputHandlers = rata.Handlers{}

			for _, route := range atc.Routes {
				inputHandlers[route.Name] = &stupidHandler{}
			}
		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewAPIAuthWrappa(
				publiclyViewable,
				fakeValidator,
				fakeUserContextReader,
			).Wrap(inputHandlers)
		})

		Context("when publicly viewable", func() {
			var expectedHandlers rata.Handlers

			BeforeEach(func() {
				publiclyViewable = true

				expectedHandlers = rata.Handlers{
					// authenticated
					atc.AbortBuild:      authenticated(inputHandlers[atc.AbortBuild]),
					atc.CreateBuild:     authenticated(inputHandlers[atc.CreateBuild]),
					atc.CreatePipe:      authenticated(inputHandlers[atc.CreatePipe]),
					atc.GetAuthToken:    authenticated(inputHandlers[atc.GetAuthToken]),
					atc.GetContainer:    authenticated(inputHandlers[atc.GetContainer]),
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
					atc.SaveConfig:             authorized(inputHandlers[atc.SaveConfig]),
					atc.UnpausePipeline:        authorized(inputHandlers[atc.UnpausePipeline]),
					atc.RenamePipeline:         authorized(inputHandlers[atc.RenamePipeline]),
					atc.GetVersionsDB:          authorized(inputHandlers[atc.GetVersionsDB]),
					atc.CheckResource:          authorized(inputHandlers[atc.CheckResource]),
					atc.CreateJobBuild:         authorized(inputHandlers[atc.CreateJobBuild]),
					atc.DisableResourceVersion: authorized(inputHandlers[atc.DisableResourceVersion]),
					atc.EnableResourceVersion:  authorized(inputHandlers[atc.EnableResourceVersion]),
					atc.DeletePipeline:         authorized(inputHandlers[atc.DeletePipeline]),
					atc.PauseResource:          authorized(inputHandlers[atc.PauseResource]),
					atc.OrderPipelines:         authorized(inputHandlers[atc.OrderPipelines]),
					atc.PauseJob:               authorized(inputHandlers[atc.PauseJob]),
					atc.UnpauseJob:             authorized(inputHandlers[atc.UnpauseJob]),
					atc.UnpauseResource:        authorized(inputHandlers[atc.UnpauseResource]),
					atc.PausePipeline:          authorized(inputHandlers[atc.PausePipeline]),
					atc.RevealPipeline:         authorized(inputHandlers[atc.RevealPipeline]),
					atc.ConcealPipeline:        authorized(inputHandlers[atc.ConcealPipeline]),
					atc.ListJobInputs:          authorized(inputHandlers[atc.ListJobInputs]),
					atc.GetConfig:              authorized(inputHandlers[atc.GetConfig]),

					// unauthenticated
					atc.BuildEvents:                   unauthenticated(inputHandlers[atc.BuildEvents]),
					atc.BuildResources:                unauthenticated(inputHandlers[atc.BuildResources]),
					atc.DownloadCLI:                   unauthenticated(inputHandlers[atc.DownloadCLI]),
					atc.GetBuild:                      unauthenticated(inputHandlers[atc.GetBuild]),
					atc.GetBuildPlan:                  unauthenticated(inputHandlers[atc.GetBuildPlan]),
					atc.GetBuildPreparation:           unauthenticated(inputHandlers[atc.GetBuildPreparation]),
					atc.GetInfo:                       unauthenticated(inputHandlers[atc.GetInfo]),
					atc.GetJob:                        unauthenticated(inputHandlers[atc.GetJob]),
					atc.GetJobBuild:                   unauthenticated(inputHandlers[atc.GetJobBuild]),
					atc.GetLogLevel:                   unauthenticated(inputHandlers[atc.GetLogLevel]),
					atc.GetPipeline:                   unauthenticated(inputHandlers[atc.GetPipeline]),
					atc.GetResource:                   unauthenticated(inputHandlers[atc.GetResource]),
					atc.JobBadge:                      unauthenticated(inputHandlers[atc.JobBadge]),
					atc.ListAuthMethods:               unauthenticated(inputHandlers[atc.ListAuthMethods]),
					atc.ListBuilds:                    unauthenticated(inputHandlers[atc.ListBuilds]),
					atc.ListBuildsWithVersionAsInput:  unauthenticated(inputHandlers[atc.ListBuildsWithVersionAsInput]),
					atc.ListBuildsWithVersionAsOutput: unauthenticated(inputHandlers[atc.ListBuildsWithVersionAsOutput]),
					atc.ListJobBuilds:                 unauthenticated(inputHandlers[atc.ListJobBuilds]),
					atc.ListJobs:                      unauthenticated(inputHandlers[atc.ListJobs]),
					atc.ListAllPipelines:              unauthenticated(inputHandlers[atc.ListAllPipelines]),
					atc.ListPipelines:                 unauthenticated(inputHandlers[atc.ListPipelines]),
					atc.ListResourceVersions:          unauthenticated(inputHandlers[atc.ListResourceVersions]),
					atc.ListResources:                 unauthenticated(inputHandlers[atc.ListResources]),
				}
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

		Context("when not publicly viewable", func() {
			var expectedHandlers rata.Handlers

			BeforeEach(func() {
				publiclyViewable = false

				expectedHandlers = rata.Handlers{
					// unauthenticated
					atc.GetInfo:         unauthenticated(inputHandlers[atc.GetInfo]),
					atc.ListAuthMethods: unauthenticated(inputHandlers[atc.ListAuthMethods]),
					atc.BuildEvents:     unauthenticated(inputHandlers[atc.BuildEvents]),
					atc.GetBuild:        unauthenticated(inputHandlers[atc.GetBuild]),

					// authenticated
					atc.AbortBuild:          authenticated(inputHandlers[atc.AbortBuild]),
					atc.BuildResources:      authenticated(inputHandlers[atc.BuildResources]),
					atc.CreateBuild:         authenticated(inputHandlers[atc.CreateBuild]),
					atc.CreatePipe:          authenticated(inputHandlers[atc.CreatePipe]),
					atc.DownloadCLI:         authenticated(inputHandlers[atc.DownloadCLI]),
					atc.GetAuthToken:        authenticated(inputHandlers[atc.GetAuthToken]),
					atc.GetBuildPlan:        authenticated(inputHandlers[atc.GetBuildPlan]),
					atc.GetBuildPreparation: authenticated(inputHandlers[atc.GetBuildPreparation]),
					atc.GetContainer:        authenticated(inputHandlers[atc.GetContainer]),
					atc.GetLogLevel:         authenticated(inputHandlers[atc.GetLogLevel]),
					atc.HijackContainer:     authenticated(inputHandlers[atc.HijackContainer]),
					atc.ListAllPipelines:    authenticated(inputHandlers[atc.ListAllPipelines]),
					atc.ListBuilds:          authenticated(inputHandlers[atc.ListBuilds]),
					atc.ListContainers:      authenticated(inputHandlers[atc.ListContainers]),
					atc.ListVolumes:         authenticated(inputHandlers[atc.ListVolumes]),
					atc.ListWorkers:         authenticated(inputHandlers[atc.ListWorkers]),
					atc.ReadPipe:            authenticated(inputHandlers[atc.ReadPipe]),
					atc.RegisterWorker:      authenticated(inputHandlers[atc.RegisterWorker]),
					atc.SetLogLevel:         authenticated(inputHandlers[atc.SetLogLevel]),
					atc.SetTeam:             authenticated(inputHandlers[atc.SetTeam]),
					atc.WritePipe:           authenticated(inputHandlers[atc.WritePipe]),

					// authorized
					atc.CheckResource:                 authorized(inputHandlers[atc.CheckResource]),
					atc.CreateJobBuild:                authorized(inputHandlers[atc.CreateJobBuild]),
					atc.DeletePipeline:                authorized(inputHandlers[atc.DeletePipeline]),
					atc.DisableResourceVersion:        authorized(inputHandlers[atc.DisableResourceVersion]),
					atc.EnableResourceVersion:         authorized(inputHandlers[atc.EnableResourceVersion]),
					atc.GetConfig:                     authorized(inputHandlers[atc.GetConfig]),
					atc.GetJob:                        authorized(inputHandlers[atc.GetJob]),
					atc.GetJobBuild:                   authorized(inputHandlers[atc.GetJobBuild]),
					atc.GetPipeline:                   authorized(inputHandlers[atc.GetPipeline]),
					atc.GetResource:                   authorized(inputHandlers[atc.GetResource]),
					atc.GetVersionsDB:                 authorized(inputHandlers[atc.GetVersionsDB]),
					atc.JobBadge:                      authorized(inputHandlers[atc.JobBadge]),
					atc.ListBuildsWithVersionAsInput:  authorized(inputHandlers[atc.ListBuildsWithVersionAsInput]),
					atc.ListBuildsWithVersionAsOutput: authorized(inputHandlers[atc.ListBuildsWithVersionAsOutput]),
					atc.ListJobBuilds:                 authorized(inputHandlers[atc.ListJobBuilds]),
					atc.ListJobInputs:                 authorized(inputHandlers[atc.ListJobInputs]),
					atc.ListJobs:                      authorized(inputHandlers[atc.ListJobs]),
					atc.ListPipelines:                 authorized(inputHandlers[atc.ListPipelines]),
					atc.ListResourceVersions:          authorized(inputHandlers[atc.ListResourceVersions]),
					atc.ListResources:                 authorized(inputHandlers[atc.ListResources]),
					atc.OrderPipelines:                authorized(inputHandlers[atc.OrderPipelines]),
					atc.PauseJob:                      authorized(inputHandlers[atc.PauseJob]),
					atc.PausePipeline:                 authorized(inputHandlers[atc.PausePipeline]),
					atc.PauseResource:                 authorized(inputHandlers[atc.PauseResource]),
					atc.RenamePipeline:                authorized(inputHandlers[atc.RenamePipeline]),
					atc.SaveConfig:                    authorized(inputHandlers[atc.SaveConfig]),
					atc.UnpauseJob:                    authorized(inputHandlers[atc.UnpauseJob]),
					atc.UnpausePipeline:               authorized(inputHandlers[atc.UnpausePipeline]),
					atc.UnpauseResource:               authorized(inputHandlers[atc.UnpauseResource]),
					atc.RevealPipeline:                authorized(inputHandlers[atc.RevealPipeline]),
					atc.ConcealPipeline:               authorized(inputHandlers[atc.ConcealPipeline]),
				}
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
})
