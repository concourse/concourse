package wrappa

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/tedsuo/rata"
)

type APIAuthWrappa struct {
	AuthValidator                     auth.Validator
	TokenValidator                    auth.Validator
	UserContextReader                 auth.UserContextReader
	checkPipelineAccessHandlerFactory auth.CheckPipelineAccessHandlerFactory
	checkBuildAccessHandlerFactory    auth.CheckBuildAccessHandlerFactory
}

func NewAPIAuthWrappa(
	authValidator auth.Validator,
	tokenValidator auth.Validator,
	userContextReader auth.UserContextReader,
	checkPipelineAccessHandlerFactory auth.CheckPipelineAccessHandlerFactory,
	checkBuildAccessHandlerFactory auth.CheckBuildAccessHandlerFactory,
) *APIAuthWrappa {
	return &APIAuthWrappa{
		AuthValidator:                     authValidator,
		TokenValidator:                    tokenValidator,
		UserContextReader:                 userContextReader,
		checkPipelineAccessHandlerFactory: checkPipelineAccessHandlerFactory,
		checkBuildAccessHandlerFactory:    checkBuildAccessHandlerFactory,
	}
}

func (wrappa *APIAuthWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	rejector := auth.UnauthorizedRejector{}

	for name, handler := range handlers {
		newHandler := handler

		switch name {
		// unauthenticated / delegating to handler
		case atc.DownloadCLI,
			atc.ListAuthMethods,
			atc.GetInfo,
			atc.ListTeams,
			atc.ListAllPipelines,
			atc.ListPipelines,
			atc.ListBuilds:

		// pipeline is public or authorized
		case atc.GetBuild,
			atc.BuildResources,
			atc.GetBuildPlan:
			newHandler = wrappa.checkBuildAccessHandlerFactory.AnyJobHandler(handler, rejector)

		// pipeline and job are public or authorized
		case atc.GetBuildPreparation,
			atc.BuildEvents:
			newHandler = wrappa.checkBuildAccessHandlerFactory.CheckIfPrivateJobHandler(handler, rejector)

		// pipeline is public or authorized
		case atc.GetPipeline,
			atc.GetJobBuild,
			atc.JobBadge,
			atc.ListJobs,
			atc.GetJob,
			atc.ListJobBuilds,
			atc.GetResource,
			atc.ListBuildsWithVersionAsInput,
			atc.ListBuildsWithVersionAsOutput,
			atc.ListResources,
			atc.ListResourceVersions:
			newHandler = wrappa.checkPipelineAccessHandlerFactory.HandlerFor(handler, rejector)

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
			atc.ListVolumes,
			atc.GetLogLevel:
			newHandler = auth.CheckAuthenticationHandler(handler, rejector)

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

		if name == atc.GetAuthToken {
			newHandler = auth.WrapHandler(newHandler, wrappa.AuthValidator, wrappa.UserContextReader)
		} else {
			newHandler = auth.WrapHandler(newHandler, wrappa.TokenValidator, wrappa.UserContextReader)
		}
		wrapped[name] = newHandler
	}

	return wrapped
}
