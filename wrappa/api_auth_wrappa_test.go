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
		fakeValidator         *fakes.FakeValidator
		fakeUserContextReader *fakes.FakeUserContextReader
	)

	BeforeEach(func() {
		fakeValidator = new(fakes.FakeValidator)
		fakeUserContextReader = new(fakes.FakeUserContextReader)
	})

	unauthed := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			handler,
			fakeValidator,
			fakeUserContextReader,
		)
	}

	authed := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthHandler(
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

			expectedHandlers rata.Handlers
		)

		BeforeEach(func() {
			inputHandlers = rata.Handlers{}

			for _, route := range atc.Routes {
				inputHandlers[route.Name] = &stupidHandler{}
			}

			expectedHandlers = rata.Handlers{
				atc.AbortBuild:             authed(inputHandlers[atc.AbortBuild]),
				atc.CreateBuild:            authed(inputHandlers[atc.CreateBuild]),
				atc.CreateJobBuild:         authed(inputHandlers[atc.CreateJobBuild]),
				atc.CreatePipe:             authed(inputHandlers[atc.CreatePipe]),
				atc.DeletePipeline:         authed(inputHandlers[atc.DeletePipeline]),
				atc.DisableResourceVersion: authed(inputHandlers[atc.DisableResourceVersion]),
				atc.EnableResourceVersion:  authed(inputHandlers[atc.EnableResourceVersion]),
				atc.GetAuthToken:           authed(inputHandlers[atc.GetAuthToken]),
				atc.GetConfig:              authed(inputHandlers[atc.GetConfig]),
				atc.GetContainer:           authed(inputHandlers[atc.GetContainer]),
				atc.GetVersionsDB:          authed(inputHandlers[atc.GetVersionsDB]),
				atc.HijackContainer:        authed(inputHandlers[atc.HijackContainer]),
				atc.ListContainers:         authed(inputHandlers[atc.ListContainers]),
				atc.ListJobInputs:          authed(inputHandlers[atc.ListJobInputs]),
				atc.ListVolumes:            authed(inputHandlers[atc.ListVolumes]),
				atc.ListWorkers:            authed(inputHandlers[atc.ListWorkers]),
				atc.OrderPipelines:         authed(inputHandlers[atc.OrderPipelines]),
				atc.PauseJob:               authed(inputHandlers[atc.PauseJob]),
				atc.PausePipeline:          authed(inputHandlers[atc.PausePipeline]),
				atc.PauseResource:          authed(inputHandlers[atc.PauseResource]),
				atc.ReadPipe:               authed(inputHandlers[atc.ReadPipe]),
				atc.RegisterWorker:         authed(inputHandlers[atc.RegisterWorker]),
				atc.SaveConfig:             authed(inputHandlers[atc.SaveConfig]),
				atc.SetLogLevel:            authed(inputHandlers[atc.SetLogLevel]),
				atc.SetTeam:                authed(inputHandlers[atc.SetTeam]),
				atc.UnpauseJob:             authed(inputHandlers[atc.UnpauseJob]),
				atc.UnpausePipeline:        authed(inputHandlers[atc.UnpausePipeline]),
				atc.UnpauseResource:        authed(inputHandlers[atc.UnpauseResource]),
				atc.WritePipe:              authed(inputHandlers[atc.WritePipe]),

				atc.BuildEvents:                   unauthed(inputHandlers[atc.BuildEvents]),
				atc.BuildResources:                unauthed(inputHandlers[atc.BuildResources]),
				atc.DownloadCLI:                   unauthed(inputHandlers[atc.DownloadCLI]),
				atc.GetBuild:                      unauthed(inputHandlers[atc.GetBuild]),
				atc.GetJob:                        unauthed(inputHandlers[atc.GetJob]),
				atc.GetJobBuild:                   unauthed(inputHandlers[atc.GetJobBuild]),
				atc.GetLogLevel:                   unauthed(inputHandlers[atc.GetLogLevel]),
				atc.GetPipeline:                   unauthed(inputHandlers[atc.GetPipeline]),
				atc.GetResource:                   unauthed(inputHandlers[atc.GetResource]),
				atc.ListAuthMethods:               unauthed(inputHandlers[atc.ListAuthMethods]),
				atc.ListBuilds:                    unauthed(inputHandlers[atc.ListBuilds]),
				atc.ListBuildsWithVersionAsInput:  unauthed(inputHandlers[atc.ListBuildsWithVersionAsInput]),
				atc.ListBuildsWithVersionAsOutput: unauthed(inputHandlers[atc.ListBuildsWithVersionAsOutput]),
				atc.ListJobBuilds:                 unauthed(inputHandlers[atc.ListJobBuilds]),
				atc.ListJobs:                      unauthed(inputHandlers[atc.ListJobs]),
				atc.ListPipelines:                 unauthed(inputHandlers[atc.ListPipelines]),
				atc.ListResourceVersions:          unauthed(inputHandlers[atc.ListResourceVersions]),
				atc.ListResources:                 unauthed(inputHandlers[atc.ListResources]),
				atc.GetBuildPlan:                  unauthed(inputHandlers[atc.GetBuildPlan]),
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
