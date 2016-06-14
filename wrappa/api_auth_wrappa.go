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
		// unauthenticated / delegating to handler
		case atc.ListAuthMethods,
			atc.GetInfo,
			atc.BuildEvents,
			atc.GetBuild:

		// authenticated if not publicly viewable
		case atc.DownloadCLI,
			atc.BuildResources,
			atc.GetLogLevel,
			atc.ListAllPipelines,
			atc.ListBuilds,
			atc.GetBuildPlan,
			atc.GetBuildPreparation:
			if !wrappa.PubliclyViewable {
				newHandler = auth.CheckAuthHandler(handler, rejector)
			}

		// authenticated
		case atc.GetAuthToken,
			atc.AbortBuild,
			atc.CreateBuild,
			atc.CreatePipe,
			atc.GetContainer,
			atc.HijackContainer,
			atc.ListContainers,
			atc.ListWorkers,
			atc.ReadPipe,
			atc.RegisterWorker,
			atc.SetLogLevel,
			atc.SetTeam,
			atc.WritePipe,
			atc.ListVolumes:
			newHandler = auth.CheckAuthHandler(handler, rejector)

		// authorized if not publicly viewable
		case atc.GetJobBuild,
			atc.JobBadge,
			atc.ListJobBuilds,
			atc.GetResource,
			atc.ListBuildsWithVersionAsInput,
			atc.ListBuildsWithVersionAsOutput,
			atc.ListResources,
			atc.ListResourceVersions,
			atc.ListPipelines,
			atc.GetPipeline,
			atc.GetJob,
			atc.ListJobs:
			if !wrappa.PubliclyViewable {
				newHandler = auth.CheckAuthorizationHandler(handler, rejector)
			}

		// authorized
		case atc.CheckResource,
			atc.CreateJobBuild,
			atc.DeletePipeline,
			atc.DisableResourceVersion,
			atc.EnableResourceVersion,
			atc.GetConfig,
			atc.GetVersionsDB,
			atc.ListJobInputs,
			atc.OrderPipelines,
			atc.PauseJob,
			atc.PausePipeline,
			atc.PauseResource,
			atc.RenamePipeline,
			atc.UnpauseJob,
			atc.UnpausePipeline,
			atc.UnpauseResource,
			atc.RevealPipeline,
			atc.ConcealPipeline,
			atc.SaveConfig:
			newHandler = auth.CheckAuthorizationHandler(handler, rejector)

		// think about it!
		default:
			panic("you missed a spot")
		}

		newHandler = auth.WrapHandler(newHandler, wrappa.Validator, wrappa.UserContextReader)

		wrapped[name] = newHandler
	}

	return wrapped
}
