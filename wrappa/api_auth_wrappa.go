package wrappa

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/tedsuo/rata"
)

type APIAuthWrappa struct {
	Validator auth.Validator
}

func NewAPIAuthWrappa(
	validator auth.Validator,
) *APIAuthWrappa {
	return &APIAuthWrappa{
		Validator: validator,
	}
}

func (wrappa *APIAuthWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	rejector := auth.UnauthorizedRejector{}

	for name, handler := range handlers {
		newHandler := handler

		switch name {
		// authenticated
		case atc.GetAuthToken, atc.AbortBuild, atc.CreateBuild, atc.CreatePipe,
			atc.DeletePipeline, atc.DisableResourceVersion, atc.EnableResourceVersion,
			atc.GetConfig, atc.GetContainer, atc.HijackContainer, atc.ListContainers,
			atc.ListJobInputs, atc.ListWorkers, atc.OrderPipelines, atc.PauseJob,
			atc.PausePipeline, atc.PauseResource, atc.ReadPipe, atc.RegisterWorker,
			atc.SaveConfig, atc.SetLogLevel, atc.UnpauseJob, atc.UnpausePipeline,
			atc.UnpauseResource, atc.WritePipe, atc.ListVolumes:
			newHandler = auth.CheckAuthHandler(handler, rejector)

		// unauthenticated
		case atc.ListAuthMethods, atc.BuildEvents, atc.DownloadCLI, atc.GetBuild,
			atc.GetJobBuild, atc.GetJob, atc.GetLogLevel, atc.ListBuilds,
			atc.ListJobBuilds, atc.ListJobs, atc.ListPipelines, atc.ListResources:

		// think about it!
		default:
			panic("you missed a spot")
		}

		newHandler = auth.WrapHandler(newHandler, wrappa.Validator)

		wrapped[name] = newHandler
	}

	return wrapped
}
