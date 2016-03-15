package wrappa

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/tedsuo/rata"
)

type APIAuthWrappa struct {
	PubliclyViewable  bool
	Validator         auth.Validator
	UserContextReader auth.UserContextReader
}

func NewAPIAuthWrappa(
	publiclyViewable bool,
	validator auth.Validator,
	userContextReader auth.UserContextReader,
) *APIAuthWrappa {
	return &APIAuthWrappa{
		PubliclyViewable:  publiclyViewable,
		Validator:         validator,
		UserContextReader: userContextReader,
	}
}

func (wrappa *APIAuthWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	rejector := auth.UnauthorizedRejector{}

	for name, handler := range handlers {
		newHandler := handler

		switch name {
		// authenticated
		case atc.GetAuthToken,
			atc.AbortBuild,
			atc.CreateBuild,
			atc.CreatePipe,
			atc.DeletePipeline,
			atc.DisableResourceVersion,
			atc.EnableResourceVersion,
			atc.GetConfig,
			atc.GetContainer,
			atc.HijackContainer,
			atc.ListContainers,
			atc.ListJobInputs,
			atc.ListWorkers,
			atc.OrderPipelines,
			atc.PauseJob,
			atc.PausePipeline,
			atc.PauseResource,
			atc.ReadPipe,
			atc.RegisterWorker,
			atc.SaveConfig,
			atc.SetLogLevel,
			atc.SetTeam,
			atc.UnpauseJob,
			atc.UnpausePipeline,
			atc.UnpauseResource,
			atc.WritePipe,
			atc.ListVolumes,
			atc.GetVersionsDB,
			atc.CreateJobBuild,
			atc.RenamePipeline:
			newHandler = auth.CheckAuthHandler(handler, rejector)

		// unauthenticated
		case atc.ListAuthMethods, atc.GetInfo:

		// unauthenticated if publicly viewable
		case atc.BuildEvents,
			atc.DownloadCLI,
			atc.GetBuild,
			atc.GetJobBuild,
			atc.BuildResources,
			atc.GetJob,
			atc.GetLogLevel,
			atc.GetResource,
			atc.ListResourceVersions,
			atc.ListBuilds,
			atc.ListBuildsWithVersionAsInput,
			atc.ListBuildsWithVersionAsOutput,
			atc.ListJobBuilds,
			atc.ListJobs,
			atc.ListPipelines,
			atc.GetPipeline,
			atc.ListResources,
			atc.GetBuildPlan,
			atc.GetBuildPreparation:
			if !wrappa.PubliclyViewable {
				newHandler = auth.CheckAuthHandler(handler, rejector)
			}

		// think about it!
		default:
			panic("you missed a spot")
		}

		newHandler = auth.WrapHandler(newHandler, wrappa.Validator, wrappa.UserContextReader)

		wrapped[name] = newHandler
	}

	return wrapped
}
