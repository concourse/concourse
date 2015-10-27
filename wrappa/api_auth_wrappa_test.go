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
		fakeValidator *fakes.FakeValidator
	)

	BeforeEach(func() {
		fakeValidator = new(fakes.FakeValidator)
	})

	unauthed := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			handler,
			fakeValidator,
		)
	}

	authed := func(handler http.Handler) http.Handler {
		return auth.WrapHandler(
			auth.CheckAuthHandler(
				handler,
				auth.UnauthorizedRejector{},
			),
			fakeValidator,
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
				atc.SaveConfig:             authed(inputHandlers[atc.SaveConfig]),
				atc.GetConfig:              authed(inputHandlers[atc.GetConfig]),
				atc.GetBuild:               unauthed(inputHandlers[atc.GetBuild]),
				atc.CreateBuild:            authed(inputHandlers[atc.CreateBuild]),
				atc.ListBuilds:             unauthed(inputHandlers[atc.ListBuilds]),
				atc.BuildEvents:            unauthed(inputHandlers[atc.BuildEvents]),
				atc.AbortBuild:             authed(inputHandlers[atc.AbortBuild]),
				atc.GetJob:                 unauthed(inputHandlers[atc.GetJob]),
				atc.ListJobs:               unauthed(inputHandlers[atc.ListJobs]),
				atc.ListJobBuilds:          unauthed(inputHandlers[atc.ListJobBuilds]),
				atc.ListJobInputs:          authed(inputHandlers[atc.ListJobInputs]),
				atc.GetJobBuild:            unauthed(inputHandlers[atc.GetJobBuild]),
				atc.PauseJob:               authed(inputHandlers[atc.PauseJob]),
				atc.UnpauseJob:             authed(inputHandlers[atc.UnpauseJob]),
				atc.ListResources:          unauthed(inputHandlers[atc.ListResources]),
				atc.EnableResourceVersion:  authed(inputHandlers[atc.EnableResourceVersion]),
				atc.DisableResourceVersion: authed(inputHandlers[atc.DisableResourceVersion]),
				atc.PauseResource:          authed(inputHandlers[atc.PauseResource]),
				atc.UnpauseResource:        authed(inputHandlers[atc.UnpauseResource]),
				atc.ListPipelines:          unauthed(inputHandlers[atc.ListPipelines]),
				atc.GetPipeline:            unauthed(inputHandlers[atc.GetPipeline]),
				atc.DeletePipeline:         authed(inputHandlers[atc.DeletePipeline]),
				atc.OrderPipelines:         authed(inputHandlers[atc.OrderPipelines]),
				atc.PausePipeline:          authed(inputHandlers[atc.PausePipeline]),
				atc.UnpausePipeline:        authed(inputHandlers[atc.UnpausePipeline]),
				atc.CreatePipe:             authed(inputHandlers[atc.CreatePipe]),
				atc.WritePipe:              authed(inputHandlers[atc.WritePipe]),
				atc.ReadPipe:               authed(inputHandlers[atc.ReadPipe]),
				atc.RegisterWorker:         authed(inputHandlers[atc.RegisterWorker]),
				atc.ListWorkers:            authed(inputHandlers[atc.ListWorkers]),
				atc.SetLogLevel:            authed(inputHandlers[atc.SetLogLevel]),
				atc.GetLogLevel:            unauthed(inputHandlers[atc.GetLogLevel]),
				atc.DownloadCLI:            unauthed(inputHandlers[atc.DownloadCLI]),
				atc.ListContainers:         authed(inputHandlers[atc.ListContainers]),
				atc.GetContainer:           authed(inputHandlers[atc.GetContainer]),
				atc.HijackContainer:        authed(inputHandlers[atc.HijackContainer]),
				atc.ListVolumes:            authed(inputHandlers[atc.ListVolumes]),
				atc.ListAuthMethods:        unauthed(inputHandlers[atc.ListAuthMethods]),
				atc.GetAuthToken:           authed(inputHandlers[atc.GetAuthToken]),
			}
		})

		JustBeforeEach(func() {
			wrappedHandlers = wrappa.NewAPIAuthWrappa(
				fakeValidator,
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
